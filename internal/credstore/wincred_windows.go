//go:build windows

package credstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errNotFoundWin32        = 1168 // ERROR_NOT_FOUND
)

var (
	modAdvapi32     = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW  = modAdvapi32.NewProc("CredWriteW")
	procCredReadW   = modAdvapi32.NewProc("CredReadW")
	procCredDeleteW = modAdvapi32.NewProc("CredDeleteW")
	procCredFree    = modAdvapi32.NewProc("CredFree")

	modCrypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
)

type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// WinCredStore stores credentials natively in Windows Credential Manager.
type WinCredStore struct{}

func newWinCredStore(_ string, _ func() ([]byte, error)) (Store, error) {
	return &WinCredStore{}, nil
}

func (s *WinCredStore) BackendName() string {
	return "Windows Credential Manager"
}

func (s *WinCredStore) Set(ref string, secret []byte) error {
	targetPtr, err := windows.UTF16PtrFromString(ref)
	if err != nil {
		return err
	}
	userPtr, err := windows.UTF16PtrFromString("sshm")
	if err != nil {
		return err
	}
	commentPtr, err := windows.UTF16PtrFromString("Managed by sshm CLI SSH client")
	if err != nil {
		return err
	}

	if len(secret) > math.MaxUint32 {
		return errors.New("credential secret exceeds Win32 maximum blob size")
	}
	var blobPtr *byte
	if len(secret) > 0 {
		blobPtr = &secret[0]
	}

	cred := winCredential{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		Comment:            commentPtr,
		CredentialBlobSize: uint32(len(secret)), // #nosec G115 -- bounds checked against math.MaxUint32 above
		CredentialBlob:     blobPtr,
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}

	r1, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r1 == 0 {
		return fmt.Errorf("CredWriteW failed for %s: %w", ref, callErr)
	}
	return nil
}

func (s *WinCredStore) Get(ref string) ([]byte, error) {
	targetPtr, err := windows.UTF16PtrFromString(ref)
	if err != nil {
		return nil, err
	}
	var credPtr *winCredential
	r1, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&credPtr)),
	)
	if r1 == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errNotFoundWin32 {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("CredReadW failed for %s: %w", ref, callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credPtr)))

	if credPtr.CredentialBlobSize == 0 || credPtr.CredentialBlob == nil {
		return []byte{}, nil
	}
	raw := unsafe.Slice(credPtr.CredentialBlob, credPtr.CredentialBlobSize)
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func (s *WinCredStore) Delete(ref string) error {
	targetPtr, err := windows.UTF16PtrFromString(ref)
	if err != nil {
		return err
	}
	r1, _, callErr := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
	)
	if r1 == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errNotFoundWin32 {
			return nil
		}
		return fmt.Errorf("CredDeleteW failed for %s: %w", ref, callErr)
	}
	return nil
}

// DPAPIStore encrypts secrets with Windows DPAPI (CURRENT_USER) and persists ciphertext in dpapi_creds.json.
type DPAPIStore struct {
	mu   sync.Mutex
	path string
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newDPAPIStore(configDir string, _ func() ([]byte, error)) (Store, error) {
	return &DPAPIStore{
		path: filepath.Join(configDir, "dpapi_creds.json"),
	}, nil
}

func (s *DPAPIStore) BackendName() string {
	return "Windows DPAPI"
}

func dpapiEncrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) > math.MaxUint32 {
		return nil, errors.New("plaintext exceeds DPAPI maximum blob size")
	}
	var inBlob dataBlob
	if len(plaintext) > 0 {
		inBlob = dataBlob{cbData: uint32(len(plaintext)), pbData: &plaintext[0]} // #nosec G115 -- bounds checked against math.MaxUint32 above
	}
	var outBlob dataBlob
	r1, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0,
		0, // Current user scope
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.pbData)))
	rawOut := unsafe.Slice(outBlob.pbData, outBlob.cbData)
	out := make([]byte, outBlob.cbData)
	copy(out, rawOut)
	ZeroBytes(rawOut)
	return out, nil
}

func dpapiDecrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return []byte{}, nil
	}
	if len(ciphertext) > math.MaxUint32 {
		return nil, errors.New("ciphertext exceeds DPAPI maximum blob size")
	}
	inBlob := dataBlob{cbData: uint32(len(ciphertext)), pbData: &ciphertext[0]} // #nosec G115 -- bounds checked against math.MaxUint32 above
	var outBlob dataBlob
	r1, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0,
		0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.pbData)))
	rawOut := unsafe.Slice(outBlob.pbData, outBlob.cbData)
	out := make([]byte, outBlob.cbData)
	copy(out, rawOut)
	ZeroBytes(rawOut)
	return out, nil
}

func (s *DPAPIStore) loadMap() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *DPAPIStore) saveMap(m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.path, data, 0o600)
}

func (s *DPAPIStore) Set(ref string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc, err := dpapiEncrypt(secret)
	if err != nil {
		return err
	}
	m, err := s.loadMap()
	if err != nil {
		return err
	}
	m[ref] = base64.StdEncoding.EncodeToString(enc)
	return s.saveMap(m)
}

func (s *DPAPIStore) Get(ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.loadMap()
	if err != nil {
		return nil, err
	}
	encoded, ok := m[ref]
	if !ok {
		return nil, ErrNotFound
	}
	enc, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return dpapiDecrypt(enc)
}

func (s *DPAPIStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.loadMap()
	if err != nil {
		return err
	}
	if _, ok := m[ref]; !ok {
		return nil
	}
	delete(m, ref)
	return s.saveMap(m)
}
