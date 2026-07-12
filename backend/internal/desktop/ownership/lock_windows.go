//go:build windows

package ownership

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	metadataSize      = 32
	metadataValueName = "owner"
	waitTimeout       = 0x00000102
)

type WindowsAcquirer struct{}

func CurrentIdentity(root string) (Identity, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return Identity{}, fmt.Errorf("current user SID: %w", err)
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil {
		return Identity{}, fmt.Errorf("current session: %w", err)
	}
	return DeriveIdentity(root, user.User.Sid.String(), session)
}

func (WindowsAcquirer) Acquire(_ context.Context, identity Identity) (Acquisition, error) {
	if identity.UserSID == "" || identity.MutexName == "" || identity.MetadataName == "" {
		return Acquisition{}, ErrInvalidIdentity
	}
	sa, err := securityAttributes(identity.UserSID)
	if err != nil {
		return Acquisition{}, err
	}
	name, err := windows.UTF16PtrFromString(identity.MutexName)
	if err != nil {
		return Acquisition{}, ErrInvalidIdentity
	}
	mutex, err := windows.CreateMutex(sa, true, name)
	alreadyExists := errors.Is(err, windows.ERROR_ALREADY_EXISTS)
	if err != nil && !alreadyExists {
		return Acquisition{}, fmt.Errorf("create global data mutex: %w", err)
	}

	outcome := OutcomeAcquired
	if alreadyExists {
		waitResult, waitErr := windows.WaitForSingleObject(mutex, 0)
		if waitErr != nil {
			_ = windows.CloseHandle(mutex)
			return Acquisition{}, fmt.Errorf("probe global data mutex: %w", waitErr)
		}
		switch waitResult {
		case windows.WAIT_OBJECT_0:
			outcome = OutcomeAcquired
		case windows.WAIT_ABANDONED:
			outcome = OutcomeAcquiredAbandoned
		case waitTimeout:
			_ = windows.CloseHandle(mutex)
			metadata, metadataErr := readOwnerMetadata(identity.MetadataName)
			if metadataErr != nil || !validOwnerMetadata(metadata) {
				return Acquisition{Outcome: OutcomeOwnedUnreachable}, nil
			}
			if metadata.SessionID == identity.SessionID {
				return Acquisition{Outcome: OutcomeOwnedSameSession, Owner: metadata}, nil
			}
			return Acquisition{Outcome: OutcomeOwnedOtherSession, Owner: metadata}, nil
		default:
			_ = windows.CloseHandle(mutex)
			return Acquisition{}, fmt.Errorf("probe global data mutex: unexpected wait result %d", waitResult)
		}
	}

	metadataKey, metadata, err := publishOwnerMetadata(identity)
	if err != nil {
		_ = windows.ReleaseMutex(mutex)
		_ = windows.CloseHandle(mutex)
		return Acquisition{}, fmt.Errorf("publish owner metadata: %w", err)
	}
	return Acquisition{
		Outcome: outcome,
		Owner:   metadata,
		Lease: &windowsLease{
			mutex:        mutex,
			metadata:     metadataKey,
			metadataPath: identity.MetadataName,
		},
	}, nil
}

func validOwnerMetadata(metadata OwnerMetadata) bool {
	return metadata.Version == 1 && metadata.PID != 0 && metadata.Nonce != ([16]byte{})
}

func securityDescriptor(sid string) (*windows.SECURITY_DESCRIPTOR, error) {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, fmt.Errorf("invalid ownership security descriptor: %w", err)
	}
	return sd, nil
}

func securityAttributes(sid string) (*windows.SecurityAttributes, error) {
	sd, err := securityDescriptor(sid)
	if err != nil {
		return nil, err
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}, nil
}

func publishOwnerMetadata(identity Identity) (registry.Key, OwnerMetadata, error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, identity.MetadataName, registry.ALL_ACCESS)
	if err != nil {
		return 0, OwnerMetadata{}, err
	}
	metadata := OwnerMetadata{Version: 1, PID: uint32(os.Getpid()), SessionID: identity.SessionID}
	if _, err := rand.Read(metadata.Nonce[:]); err != nil {
		_ = key.Close()
		return 0, OwnerMetadata{}, err
	}
	sd, err := securityDescriptor(identity.UserSID)
	if err != nil {
		_ = key.Close()
		return 0, OwnerMetadata{}, err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		_ = key.Close()
		return 0, OwnerMetadata{}, err
	}
	if err := windows.SetSecurityInfo(
		windows.Handle(key),
		windows.SE_REGISTRY_KEY,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		_ = key.Close()
		return 0, OwnerMetadata{}, err
	}
	encoded := make([]byte, metadataSize)
	encodeMetadata(encoded, metadata)
	if err := key.SetBinaryValue(metadataValueName, encoded); err != nil {
		_ = key.Close()
		return 0, OwnerMetadata{}, err
	}
	return key, metadata, nil
}

func readOwnerMetadata(name string) (OwnerMetadata, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, name, registry.QUERY_VALUE)
	if err != nil {
		return OwnerMetadata{}, err
	}
	defer key.Close()
	encoded, _, err := key.GetBinaryValue(metadataValueName)
	if err != nil {
		return OwnerMetadata{}, err
	}
	if len(encoded) != metadataSize {
		return OwnerMetadata{}, ErrOwnerUnreachable
	}
	return decodeMetadata(encoded), nil
}

func encodeMetadata(dst []byte, metadata OwnerMetadata) {
	binary.LittleEndian.PutUint32(dst[0:4], metadata.Version)
	binary.LittleEndian.PutUint32(dst[4:8], metadata.PID)
	binary.LittleEndian.PutUint32(dst[8:12], metadata.SessionID)
	copy(dst[12:28], metadata.Nonce[:])
}

func decodeMetadata(src []byte) OwnerMetadata {
	var nonce [16]byte
	copy(nonce[:], src[12:28])
	return OwnerMetadata{
		Version:   binary.LittleEndian.Uint32(src[0:4]),
		PID:       binary.LittleEndian.Uint32(src[4:8]),
		SessionID: binary.LittleEndian.Uint32(src[8:12]),
		Nonce:     nonce,
	}
}

type windowsLease struct {
	mu           sync.Mutex
	mutex        windows.Handle
	metadata     registry.Key
	metadataPath string
	closed       bool
}

func (l *windowsLease) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var result error
	if l.metadata != 0 {
		if err := l.metadata.DeleteValue(metadataValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			result = errors.Join(result, err)
		}
		result = errors.Join(result, l.metadata.Close())
		if err := registry.DeleteKey(registry.CURRENT_USER, l.metadataPath); err != nil && !errors.Is(err, registry.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}
	result = errors.Join(result, windows.ReleaseMutex(l.mutex))
	result = errors.Join(result, windows.CloseHandle(l.mutex))
	return result
}
