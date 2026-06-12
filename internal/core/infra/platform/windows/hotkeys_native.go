//go:build windows

// internal/core/infra/platform/windows/hotkeys_native.go
// Global hotkey registration via RegisterHotKey and a message-only window.
// Does not parse Neru config bindings.

package windows

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmHotkey        = 0x0312
	hotkeyClassName = "NeruHotkeyWindow"
	hwndMessage     = ^uintptr(2)
)

type hotkeyRegistration struct {
	id       int
	modifiers uint32
	virtualKey uint32
}

// HotkeyRegistry manages RegisterHotKey bindings on a hidden message window.
type HotkeyRegistry struct {
	mu          sync.Mutex
	hwnd        uintptr
	callbacks   map[int]func()
	threadDone  chan struct{}
	threadStop  chan struct{}
	registered  map[int]hotkeyRegistration
	nextID      int
	classOnce   sync.Once
	classErr    error
}

var (
	procRegisterHotKeyW   = user32.NewProc("RegisterHotKeyW")
	procUnregisterHotKeyW = user32.NewProc("UnregisterHotKeyW")

	globalHotkeyRegistry *HotkeyRegistry
	globalHotkeyOnce     sync.Once
)

// GlobalHotkeyRegistry returns the process-wide hotkey registry.
func GlobalHotkeyRegistry() (*HotkeyRegistry, error) {
	var initErr error

	globalHotkeyOnce.Do(func() {
		registry := &HotkeyRegistry{
			callbacks:  make(map[int]func()),
			registered: make(map[int]hotkeyRegistration),
			threadStop: make(chan struct{}),
			threadDone: make(chan struct{}),
			nextID:     1,
		}

		if err := registry.start(); err != nil {
			initErr = err

			return
		}

		globalHotkeyRegistry = registry
	})

	if initErr != nil {
		return nil, initErr
	}

	return globalHotkeyRegistry, nil
}

func (r *HotkeyRegistry) start() error {
	if err := r.registerHotkeyClass(); err != nil {
		return err
	}

	go r.messageLoop()

	return nil
}

func (r *HotkeyRegistry) registerHotkeyClass() error {
	r.classOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(hotkeyClassName)
		if err != nil {
			r.classErr = err

			return
		}

		wndProc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
			if msg == wmHotkey {
				if registry := globalHotkeyRegistry; registry != nil {
					registry.dispatch(int(wParam))
				}

				return 0
			}

			ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)

			return ret
		})

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			lpfnWndProc:   wndProc,
			hInstance:     windows.Handle(moduleHandle()),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			r.classErr = fmt.Errorf("RegisterClassExW hotkey: %w", err)
		}
	})

	return r.classErr
}

func (r *HotkeyRegistry) messageLoop() {
	defer close(r.threadDone)

	className, err := windows.UTF16PtrFromString(hotkeyClassName)
	if err != nil {
		return
	}

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0,
		0,
		0,
		0,
		hwndMessage,
		0,
		moduleHandle(),
		0,
	)
	if hwnd == 0 {
		return
	}

	r.mu.Lock()
	r.hwnd = hwnd
	r.mu.Unlock()

	defer procDestroyWindow.Call(hwnd)

	var message msg
	for {
		select {
		case <-r.threadStop:
			threadID, _, _ := procGetCurrentThreadId.Call()
			procPostThreadMessageW.Call(
				threadID,
				wmQuit,
				0,
				0,
			)
		default:
		}

		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&message)),
			0,
			0,
			0,
		)
		if ret == 0 || int32(ret) == -1 {
			return
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (r *HotkeyRegistry) dispatch(id int) {
	r.mu.Lock()
	callback := r.callbacks[id]
	r.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// Register binds a hotkey string to a callback and returns a registry id.
func (r *HotkeyRegistry) Register(keyString string, callback func()) (int, error) {
	if callback == nil {
		return 0, fmt.Errorf("hotkey callback is nil")
	}

	mods, vk, err := ParseHotkeyString(keyString)
	if err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hwnd == 0 {
		return 0, fmt.Errorf("hotkey message window not ready")
	}

	id := r.nextID
	r.nextID++

	ret, _, regErr := procRegisterHotKeyW.Call(
		r.hwnd,
		uintptr(id),
		uintptr(mods),
		uintptr(vk),
	)
	if ret == 0 {
		return 0, fmt.Errorf("RegisterHotKeyW: %w", regErr)
	}

	r.callbacks[id] = callback
	r.registered[id] = hotkeyRegistration{id: id, modifiers: mods, virtualKey: vk}

	return id, nil
}

// Unregister removes a previously registered hotkey id.
func (r *HotkeyRegistry) Unregister(id int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hwnd != 0 {
		procUnregisterHotKeyW.Call(r.hwnd, uintptr(id))
	}

	delete(r.callbacks, id)
	delete(r.registered, id)
}

// UnregisterAll removes all hotkeys.
func (r *HotkeyRegistry) UnregisterAll() {
	r.mu.Lock()
	ids := make([]int, 0, len(r.registered))
	for id := range r.registered {
		ids = append(ids, id)
	}
	r.mu.Unlock()

	for _, id := range ids {
		r.Unregister(id)
	}
}
