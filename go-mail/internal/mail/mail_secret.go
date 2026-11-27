package mail

import (
	"encoding/base64"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

type dpapiBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32DLL    = syscall.NewLazyDLL("crypt32.dll")
	procProtect   = crypt32DLL.NewProc("CryptProtectData")
	procUnprotect = crypt32DLL.NewProc("CryptUnprotectData")
	kernel32DLL   = syscall.NewLazyDLL("kernel32.dll")
	procLocalFree = kernel32DLL.NewProc("LocalFree")
)

func dpapiProtect(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	data := []byte(plain)
	in := dpapiBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dpapiBlob
	r, _, err := procProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", fmt.Errorf("CryptProtectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	raw := unsafe.Slice(out.pbData, out.cbData)
	return "dpapi:" + base64.StdEncoding.EncodeToString(raw), nil
}

func dpapiUnprotect(stored string) (string, error) {
	if !strings.HasPrefix(stored, "dpapi:") {
		// Legacy plain-text password (or empty).
		return stored, nil
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "dpapi:"))
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", nil
	}
	in := dpapiBlob{cbData: uint32(len(b)), pbData: &b[0]}
	var out dpapiBlob
	r, _, err := procUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return string(unsafe.Slice(out.pbData, out.cbData)), nil
}

func EncryptSecret(plain string) string {
	if plain == "" {
		return ""
	}
	if enc, err := dpapiProtect(plain); err == nil {
		return enc
	}
	return plain
}

func DecryptSecret(stored string) string {
	if stored == "" {
		return ""
	}
	if plain, err := dpapiUnprotect(stored); err == nil {
		return plain
	}
	return ""
}
