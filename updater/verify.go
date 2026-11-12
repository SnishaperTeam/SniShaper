//go:build windows

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyFileSHA256(path, want string) error {
	want = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(want), "sha256:"))
	if want == "" {
		return fmt.Errorf("missing expected sha256 for %s", path)
	}
	got, err := fileSHA256Hex(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, got, want)
	}
	return nil
}

func decodePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length %d", len(b))
	}
	return ed25519.PublicKey(b), nil
}

func verifyManifestSignature(body []byte, sigText, pubB64 string) error {
	pub, err := decodePublicKey(pubB64)
	if err != nil {
		return err
	}
	sig := []byte(sigText)
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigText)); err == nil && len(decoded) == ed25519.SignatureSize {
		sig = decoded
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid manifest signature length %d", len(sig))
	}
	if !ed25519.Verify(pub, body, sig) {
		return fmt.Errorf("manifest signature verification failed")
	}
	return nil
}

type wintrustGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type wintrustFileInfo struct {
	CbStruct     uint32
	FilePath     uintptr
	HFile        uintptr
	KnownSubject uintptr
}

type wintrustData struct {
	CbStruct           uint32
	PolicyCallbackData uintptr
	SIPClientData      uintptr
	UIChoice           uint32
	RevocationChecks   uint32
	UnionChoice        uint32
	FileInfo           uintptr
	StateAction        uint32
	StateData          uintptr
	URLReference       uintptr
	ProvFlags          uint32
	UIContext          uint32
	SignatureSettings  uintptr
}

var wintrustActionGenericVerifyV2 = wintrustGUID{
	Data1: 0x00aac56b,
	Data2: 0xcd44,
	Data3: 0x11d0,
	Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee},
}

func verifyAuthenticode(path string) error {
	fp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var fi wintrustFileInfo
	fi.CbStruct = uint32(unsafe.Sizeof(fi))
	fi.FilePath = uintptr(unsafe.Pointer(fp))
	var wd wintrustData
	wd.CbStruct = uint32(unsafe.Sizeof(wd))
	wd.UIChoice = 2
	wd.RevocationChecks = 0
	wd.UnionChoice = 1
	wd.FileInfo = uintptr(unsafe.Pointer(&fi))
	wd.StateAction = 1
	action := wintrustActionGenericVerifyV2
	r, _, callErr := procWinVerifyTrust.Call(0, uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&wd)))
	wd.StateAction = 0
	_, _, _ = procWinVerifyTrust.Call(0, uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&wd)))
	runtime.KeepAlive(fp)
	if r != 0 {
		return fmt.Errorf("WinVerifyTrust failed for %s: 0x%08X (%v)", path, uint32(r), callErr)
	}
	return nil
}
