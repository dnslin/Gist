package ownership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxFrameSize = 1024

var (
	ErrInvalidIdentity           = errors.New("invalid desktop ownership identity")
	ErrOwnedSameSession          = errors.New("data_owned_same_session")
	ErrOwnedOtherSession         = errors.New("data_owned_other_session")
	ErrOwnerUnreachable          = errors.New("data_owner_unreachable")
	ErrActivationProtocolInvalid = errors.New("activation_protocol_invalid")
)

type Identity struct {
	MutexName    string
	MetadataName string
	PipeName     string
	UserSID      string
	SessionID    uint32
}

func DeriveIdentity(root, userSID string, sessionID uint32) (Identity, error) {
	root = strings.ToLower(filepath.Clean(strings.TrimSpace(root)))
	userSID = strings.TrimSpace(userSID)
	if root == "" || root == "." || userSID == "" || strings.IndexByte(root, 0) >= 0 || strings.IndexByte(userSID, 0) >= 0 {
		return Identity{}, ErrInvalidIdentity
	}
	pathSum := sha256.Sum256([]byte("gist.desktop.data.v1\x00" + root))
	userSum := sha256.Sum256([]byte(userSID))
	pathHash := hex.EncodeToString(pathSum[:8])
	userHash := hex.EncodeToString(userSum[:8])
	base := userHash + "." + pathHash
	return Identity{
		MutexName:    `Global\Gist.Data.v1.` + base,
		MetadataName: `Software\Gist\Desktop\Ownership\v1\` + base,
		PipeName:     fmt.Sprintf(`\\.\pipe\Gist.Activate.v1.%s.%d.%s`, userHash, sessionID, pathHash),
		UserSID:      userSID,
		SessionID:    sessionID,
	}, nil
}

type OwnerMetadata struct {
	Version   uint32
	PID       uint32
	SessionID uint32
	Nonce     [16]byte
}

type Outcome uint8

const (
	OutcomeAcquired Outcome = iota + 1
	OutcomeAcquiredAbandoned
	OutcomeOwnedSameSession
	OutcomeOwnedOtherSession
	OutcomeOwnedUnreachable
)

type Lease interface {
	Close() error
}

type Acquisition struct {
	Outcome Outcome
	Lease   Lease
	Owner   OwnerMetadata
}

type Acquirer interface {
	Acquire(context.Context, Identity) (Acquisition, error)
}

type ActivationSink interface {
	Activate(context.Context) error
}

type ActivationSinkFunc func(context.Context) error

func (f ActivationSinkFunc) Activate(ctx context.Context) error {
	return f(ctx)
}

type Request struct {
	Version int    `json:"version"`
	Action  string `json:"action"`
}

type Response struct {
	Version int    `json:"version"`
	Result  string `json:"result"`
}

const (
	ResultAccepted             = "accepted"
	ResultOccupiedOtherSession = "occupied_other_session"
	ResultOccupiedUnreachable  = "occupied_unreachable"
)

func EncodeRequest() ([]byte, error) {
	return encodeFrame(Request{Version: 1, Action: "activate"})
}

func EncodeResponse(result string) ([]byte, error) {
	return encodeFrame(Response{Version: 1, Result: result})
}

func encodeFrame(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(body) > MaxFrameSize {
		return nil, ErrActivationProtocolInvalid
	}
	out := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(out, uint32(len(body)))
	copy(out[4:], body)
	return out, nil
}

func DecodeResponse(frame []byte) (Response, error) {
	var response Response
	if err := decodeFrame(frame, &response); err != nil {
		return Response{}, err
	}
	if response.Version != 1 || response.Result != ResultAccepted && response.Result != ResultOccupiedOtherSession && response.Result != ResultOccupiedUnreachable {
		return Response{}, ErrActivationProtocolInvalid
	}
	return response, nil
}

func HandleFrame(ctx context.Context, frame []byte, sink ActivationSink) (Response, error) {
	var request Request
	if sink == nil || decodeFrame(frame, &request) != nil || request.Version != 1 || request.Action != "activate" {
		return Response{}, ErrActivationProtocolInvalid
	}
	if err := sink.Activate(ctx); err != nil {
		return Response{Version: 1, Result: ResultOccupiedUnreachable}, nil
	}
	return Response{Version: 1, Result: ResultAccepted}, nil
}

func decodeFrame(frame []byte, target any) error {
	if len(frame) < 4 {
		return ErrActivationProtocolInvalid
	}
	size := binary.LittleEndian.Uint32(frame[:4])
	if size == 0 || size > MaxFrameSize || int(size) != len(frame)-4 || !utf8.Valid(frame[4:]) {
		return ErrActivationProtocolInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(frame[4:]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrActivationProtocolInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrActivationProtocolInvalid
	}
	return nil
}
