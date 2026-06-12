//go:build windows

// internal/core/infra/platform/windows/overlay_ui.go
// Dedicated Win32 UI thread with a message pump for HWND creation and painting.
// Does not implement overlay drawing; overlay.go marshals HWND work here.

package windows

import (
	"runtime"
	"sync"
	"unsafe"
)

var (
	overlayUIOnce sync.Once
	overlayUIOps  chan func()
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct {
		x int32
		y int32
	}
}

func startOverlayUIThread() {
	overlayUIOnce.Do(func() {
		overlayUIOps = make(chan func(), 256)
		ready := make(chan struct{})

		go func() {
			runtime.LockOSThread()
			close(ready)

			for fn := range overlayUIOps {
				fn()
				pumpOverlayMessages()
			}
		}()

		<-ready
	})
}

func runOnOverlayUI(fn func()) {
	startOverlayUIThread()

	done := make(chan struct{})
	overlayUIOps <- func() {
		fn()
		close(done)
	}
	<-done
}

func pumpOverlayMessages() {
	var msg winMsg

	for {
		ret, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
			pmRemove,
		)
		if ret == 0 {
			return
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
