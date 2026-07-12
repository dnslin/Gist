package ownership_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/desktop/ownership"
)

func TestIdentityHidesRawPathAndScopesObjects(t *testing.T) {
	id, err := ownership.DeriveIdentity(`C:\Users\Alice\AppData\Local\Gist`, "S-1-5-21-123", 7)
	require.NoError(t, err)
	require.Contains(t, id.MutexName, `Global\Gist.Data.v1.`)
	require.Contains(t, id.MetadataName, `Software\Gist\Desktop\Ownership\v1\`)
	require.Contains(t, id.PipeName, `\\.\pipe\Gist.Activate.v1.`)
	for _, value := range []string{id.MutexName, id.MetadataName, id.PipeName} {
		require.NotContains(t, strings.ToLower(value), "alice")
		require.NotContains(t, value, "S-1-5-21")
	}
}

func TestActivationFrameStrictBoundedAndDeadlineAware(t *testing.T) {
	valid, err := ownership.EncodeRequest()
	require.NoError(t, err)
	called := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := ownership.HandleFrame(ctx, valid, ownership.ActivationSinkFunc(func(sinkCtx context.Context) error {
		called++
		_, hasDeadline := sinkCtx.Deadline()
		require.True(t, hasDeadline)
		return nil
	}))
	require.NoError(t, err)
	require.Equal(t, 1, called)
	require.Equal(t, ownership.ResultAccepted, response.Result)

	invalidBodies := [][]byte{
		[]byte(`{"version":2,"action":"activate"}`),
		[]byte(`{"version":1,"action":"open"}`),
		[]byte(`{"version":1,"action":"activate","url":"https://example.com"}`),
		[]byte(`{"version":1,"action":"activate"} trailing`),
		append([]byte(`{"version":1,"action":"act`), 0xff),
	}
	for _, body := range invalidBodies {
		_, err := ownership.HandleFrame(context.Background(), frame(body), ownership.ActivationSinkFunc(func(context.Context) error { return nil }))
		require.ErrorIs(t, err, ownership.ErrActivationProtocolInvalid)
	}
	_, err = ownership.HandleFrame(context.Background(), frame(bytes.Repeat([]byte("x"), ownership.MaxFrameSize+1)), ownership.ActivationSinkFunc(func(context.Context) error { return nil }))
	require.ErrorIs(t, err, ownership.ErrActivationProtocolInvalid)

	trailingFrame := append(append([]byte{}, valid...), valid...)
	_, err = ownership.HandleFrame(context.Background(), trailingFrame, ownership.ActivationSinkFunc(func(context.Context) error { return nil }))
	require.ErrorIs(t, err, ownership.ErrActivationProtocolInvalid)
}

func frame(body []byte) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(body)))
	out.Write(body)
	return out.Bytes()
}
