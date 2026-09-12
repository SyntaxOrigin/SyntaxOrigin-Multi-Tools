//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// windowsLocale Windows kullanıcı arayüz dilini GetUserDefaultLocaleName ile alır.
func windowsLocale() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getLocale := k32.NewProc("GetUserDefaultLocaleName")
	var buf [85]uint16
	r, _, _ := getLocale.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}
