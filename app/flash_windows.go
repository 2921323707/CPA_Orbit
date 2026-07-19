//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	findWindowProc  = user32.NewProc("FindWindowW")
	flashWindowProc = user32.NewProc("FlashWindowEx")
)

type flashInfo struct {
	cbSize    uint32
	hwnd      uintptr
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

func flashWindow(title string) {
	caption, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	hwnd, _, _ := findWindowProc.Call(0, uintptr(unsafe.Pointer(caption)))
	if hwnd == 0 {
		return
	}
	const flashwAll = 0x00000003
	info := flashInfo{cbSize: uint32(unsafe.Sizeof(flashInfo{})), hwnd: hwnd, dwFlags: flashwAll, uCount: 4, dwTimeout: 350}
	_, _, _ = flashWindowProc.Call(uintptr(unsafe.Pointer(&info)))
	go func() {
		time.Sleep(2 * time.Second)
		info.dwFlags = 0
		_, _, _ = flashWindowProc.Call(uintptr(unsafe.Pointer(&info)))
	}()
}
