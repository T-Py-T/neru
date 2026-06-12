//go:build windows

// internal/core/infra/platform/windows/overlay.go
// Layered Win32 overlay window with a 32-bit ARGB backing bitmap for GDI drawing.
// Does not implement grid logic or ports; ui/overlay consumes this surface.

package windows

import (
	"fmt"
	"image"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	overlayClassName = "NeruOverlayWindow"

	wsPopup            = 0x80000000
	wsExLayered        = 0x00080000
	wsExTransparent    = 0x00000020
	wsExTopmost        = 0x00000008
	wsExToolWindow     = 0x00000080
	wsExNoActivate     = 0x08000000
	swHide             = 0
	swShowNoActivate   = 4
	ulwAlpha           = 0x00000002
	acSrcOver          = 0x00
	acSrcAlpha         = 0x01
	biRGB              = 0
	transparentBk      = 1
	dibRGBColors       = 0
	dtCenter           = 0x00000001
	dtVCenter          = 0x00000004
	dtSingleLine       = 0x00000020
	hwndTopMost        = ^uintptr(0)
	swpNoActivate      = 0x0010
	swpShowWindow      = 0x0040
	defaultOverlayFont = "Segoe UI"
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procCreateFontW         = gdi32.NewProc("CreateFontW")
	procSetBkMode           = gdi32.NewProc("SetBkMode")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
	procDrawTextW           = user32.NewProc("DrawTextW")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")

	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	overlayClassOnce sync.Once
	overlayClassErr  error
)

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type blendFunction struct {
	blendOp             byte
	blendFlags          byte
	sourceConstantAlpha byte
	alphaFormat         byte
}

// OverlayWindow is a fullscreen click-through layered HWND backed by a 32-bit bitmap.
type OverlayWindow struct {
	hwnd     windows.HWND
	bounds   image.Rectangle
	width    int
	height   int
	pixels   []byte
	hdcMem   windows.Handle
	hBitmap  windows.Handle
	visible  bool
}

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)

	return ret
}

func registerOverlayWindowClass() error {
	overlayClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(overlayClassName)
		if err != nil {
			overlayClassErr = err

			return
		}

		wndProc := syscall.NewCallback(overlayWndProc)
		instance, _, _ := procGetModuleHandleW.Call(0)

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			lpfnWndProc:   wndProc,
			hInstance:     windows.Handle(instance),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			overlayClassErr = fmt.Errorf("RegisterClassExW: %w", err)
		}
	})

	return overlayClassErr
}

// NewOverlayWindow creates a layered overlay sized to the active monitor.
func NewOverlayWindow() (*OverlayWindow, error) {
	if err := registerOverlayWindowClass(); err != nil {
		return nil, err
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return nil, err
	}

	overlay := &OverlayWindow{bounds: bounds}
	if err := overlay.createHWND(); err != nil {
		return nil, err
	}

	if err := overlay.createBitmap(); err != nil {
		overlay.destroyHWND()

		return nil, err
	}

	overlay.Clear()

	return overlay, nil
}

func (o *OverlayWindow) createHWND() error {
	className, err := windows.UTF16PtrFromString(overlayClassName)
	if err != nil {
		return err
	}

	width := o.bounds.Dx()
	height := o.bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid overlay bounds %v", o.bounds)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)),
		0,
		wsPopup,
		uintptr(o.bounds.Min.X),
		uintptr(o.bounds.Min.Y),
		uintptr(width),
		uintptr(height),
		0,
		0,
		moduleHandle(),
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}

	o.hwnd = windows.HWND(hwnd)
	o.width = width
	o.height = height

	return nil
}

func (o *OverlayWindow) createBitmap() error {
	if o.hdcMem != 0 {
		procDeleteDC.Call(uintptr(o.hdcMem))
		o.hdcMem = 0
	}

	if o.hBitmap != 0 {
		procDeleteObject.Call(uintptr(o.hBitmap))
		o.hBitmap = 0
	}

	hdcScreen := getDesktopDC()
	if hdcScreen == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer releaseDesktopDC(hdcScreen)

	hdcMem, _, err := procCreateCompatibleDC.Call(uintptr(hdcScreen))
	if hdcMem == 0 {
		return fmt.Errorf("CreateCompatibleDC: %w", err)
	}

	var header bitmapInfoHeader
	header.biSize = uint32(unsafe.Sizeof(header))
	header.biWidth = int32(o.width)
	header.biHeight = -int32(o.height)
	header.biPlanes = 1
	header.biBitCount = 32
	header.biCompression = biRGB

	var bits unsafe.Pointer
	hBitmap, _, err := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&header)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hBitmap == 0 {
		procDeleteDC.Call(hdcMem)

		return fmt.Errorf("CreateDIBSection: %w", err)
	}

	procSelectObject.Call(hdcMem, hBitmap)
	o.hdcMem = windows.Handle(hdcMem)
	o.hBitmap = windows.Handle(hBitmap)
	o.pixels = unsafe.Slice((*byte)(bits), o.width*o.height*4)

	return nil
}

func (o *OverlayWindow) destroyHWND() {
	if o.hwnd != 0 {
		procDestroyWindow.Call(uintptr(o.hwnd))
		o.hwnd = 0
	}
}

// HWND returns the native window handle.
func (o *OverlayWindow) HWND() windows.HWND {
	return o.hwnd
}

// Healthy reports whether the overlay window and bitmap are initialized.
func (o *OverlayWindow) Healthy() bool {
	return o != nil && o.hwnd != 0 && o.hdcMem != 0 && len(o.pixels) > 0
}

// Bounds returns the overlay rectangle in screen coordinates.
func (o *OverlayWindow) Bounds() image.Rectangle {
	return o.bounds
}

// Show displays the overlay without taking focus.
func (o *OverlayWindow) Show() {
	if o == nil || o.hwnd == 0 {
		return
	}

	procShowWindow.Call(uintptr(o.hwnd), swShowNoActivate)
	const swpNomove = 0x0002
	const swpNosize = 0x0001
	procSetWindowPos.Call(
		uintptr(o.hwnd),
		hwndTopMost,
		0,
		0,
		0,
		0,
		swpNoActivate|swpShowWindow|swpNomove|swpNosize,
	)
	o.visible = true

	_ = o.Flush()
}

// Hide hides the overlay window.
func (o *OverlayWindow) Hide() {
	if o == nil || o.hwnd == 0 {
		return
	}

	procShowWindow.Call(uintptr(o.hwnd), swHide)
	o.visible = false
}

// Clear resets the backing bitmap to fully transparent.
func (o *OverlayWindow) Clear() {
	if o == nil || len(o.pixels) == 0 {
		return
	}

	for i := range o.pixels {
		o.pixels[i] = 0
	}
}

// ResizeToActiveScreen moves and resizes the overlay to the active monitor.
func (o *OverlayWindow) ResizeToActiveScreen() error {
	if o == nil {
		return fmt.Errorf("overlay is nil")
	}

	bounds, err := activeScreenBounds()
	if err != nil {
		return err
	}

	if bounds == o.bounds && o.width == bounds.Dx() && o.height == bounds.Dy() {
		return nil
	}

	o.bounds = bounds
	o.width = bounds.Dx()
	o.height = bounds.Dy()

	if o.hwnd != 0 {
		procSetWindowPos.Call(
			uintptr(o.hwnd),
			hwndTopMost,
			uintptr(bounds.Min.X),
			uintptr(bounds.Min.Y),
			uintptr(o.width),
			uintptr(o.height),
			swpNoActivate|swpShowWindow,
		)
	}

	return o.createBitmap()
}

// Destroy releases native overlay resources.
func (o *OverlayWindow) Destroy() {
	if o == nil {
		return
	}

	if o.hdcMem != 0 {
		procDeleteDC.Call(uintptr(o.hdcMem))
		o.hdcMem = 0
	}

	if o.hBitmap != 0 {
		procDeleteObject.Call(uintptr(o.hBitmap))
		o.hBitmap = 0
	}

	o.pixels = nil
	o.destroyHWND()
}

func (o *OverlayWindow) localBounds() image.Rectangle {
	return image.Rect(0, 0, o.width, o.height)
}

// FillRect fills a rectangle with an ARGB color.
// Bounds are window-local coordinates (0,0 at the overlay top-left).
func (o *OverlayWindow) FillRect(bounds image.Rectangle, color uint32) {
	if o == nil || len(o.pixels) == 0 || bounds.Empty() {
		return
	}

	rect := bounds.Intersect(o.localBounds())
	if rect.Empty() {
		return
	}

	alpha := uint8(color >> 24)
	red := uint8((color >> 16) & 0xFF)
	green := uint8((color >> 8) & 0xFF)
	blue := uint8(color & 0xFF)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		row := y * o.width * 4
		for x := rect.Min.X; x < rect.Max.X; x++ {
			off := row + x*4
			o.pixels[off] = blue
			o.pixels[off+1] = green
			o.pixels[off+2] = red
			o.pixels[off+3] = alpha
		}
	}
}

// StrokeRect draws a rectangular border with the given ARGB color and width.
func (o *OverlayWindow) StrokeRect(bounds image.Rectangle, color uint32, lineWidth float64) {
	if o == nil || bounds.Empty() || lineWidth <= 0 {
		return
	}

	width := int(lineWidth)
	if width < 1 {
		width = 1
	}

	for i := 0; i < width; i++ {
		inset := bounds.Inset(i)
		o.FillRect(image.Rect(inset.Min.X, inset.Min.Y, inset.Max.X, inset.Min.Y+1), color)
		o.FillRect(image.Rect(inset.Min.X, inset.Max.Y-1, inset.Max.X, inset.Max.Y), color)
		o.FillRect(image.Rect(inset.Min.X, inset.Min.Y, inset.Min.X+1, inset.Max.Y), color)
		o.FillRect(image.Rect(inset.Max.X-1, inset.Min.Y, inset.Max.X, inset.Max.Y), color)
	}
}

// DrawTextCentered renders centered text inside bounds using GDI.
func (o *OverlayWindow) DrawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	if o == nil || o.hdcMem == 0 || text == "" || bounds.Empty() {
		return
	}

	if fontFamily == "" {
		fontFamily = defaultOverlayFont
	}

	size := int(-fontSize)
	if size == 0 {
		size = -14
	}

	fontName, err := windows.UTF16PtrFromString(fontFamily)
	if err != nil {
		return
	}

	hFont, _, _ := procCreateFontW.Call(
		uintptr(size),
		0,
		0,
		0,
		400,
		0,
		0,
		0,
		1,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(fontName)),
	)
	if hFont == 0 {
		return
	}

	defer procDeleteObject.Call(hFont)

	procSelectObject.Call(uintptr(o.hdcMem), hFont)
	procSetBkMode.Call(uintptr(o.hdcMem), transparentBk)
	procSetTextColor.Call(uintptr(o.hdcMem), uintptr(argbToColorRef(color)))

	utf16Text, err := windows.UTF16FromString(text)
	if err != nil {
		return
	}

	rect := windows.Rect{
		Left:   int32(bounds.Min.X),
		Top:    int32(bounds.Min.Y),
		Right:  int32(bounds.Max.X),
		Bottom: int32(bounds.Max.Y),
	}

	procDrawTextW.Call(
		uintptr(o.hdcMem),
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&rect)),
		dtCenter|dtVCenter|dtSingleLine,
	)

	o.fixAlphaFromRGB()
}

// Flush presents the backing bitmap through UpdateLayeredWindow.
func (o *OverlayWindow) Flush() error {
	if o == nil || o.hwnd == 0 || o.hdcMem == 0 {
		return fmt.Errorf("overlay window is not initialized")
	}

	o.fixAlphaFromRGB()

	var srcPoint struct {
		x int32
		y int32
	}
	var dstPoint struct {
		x int32
		y int32
	}
	var windowSize struct {
		cx int32
		cy int32
	}
	var blend blendFunction
	blend.blendOp = acSrcOver
	blend.alphaFormat = acSrcAlpha

	dstPoint.x = int32(o.bounds.Min.X)
	dstPoint.y = int32(o.bounds.Min.Y)
	windowSize.cx = int32(o.width)
	windowSize.cy = int32(o.height)

	hdcScreen := getDesktopDC()
	if hdcScreen == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer releaseDesktopDC(hdcScreen)

	ret, _, err := procUpdateLayeredWindow.Call(
		uintptr(o.hwnd),
		uintptr(hdcScreen),
		uintptr(unsafe.Pointer(&dstPoint)),
		uintptr(unsafe.Pointer(&windowSize)),
		uintptr(o.hdcMem),
		uintptr(unsafe.Pointer(&srcPoint)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlpha,
	)
	if ret == 0 {
		return fmt.Errorf("UpdateLayeredWindow: %w", err)
	}

	return nil
}

func (o *OverlayWindow) fixAlphaFromRGB() {
	if o == nil || len(o.pixels) == 0 {
		return
	}

	for i := 0; i < len(o.pixels); i += 4 {
		if o.pixels[i]|o.pixels[i+1]|o.pixels[i+2] != 0 && o.pixels[i+3] == 0 {
			o.pixels[i+3] = 255
		}
	}
}

func moduleHandle() uintptr {
	handle, _, _ := procGetModuleHandleW.Call(0)

	return handle
}

func getDesktopDC() uintptr {
	hdc, _, _ := procGetDC.Call(0)

	return hdc
}

func releaseDesktopDC(hdc uintptr) {
	procReleaseDC.Call(0, hdc)
}

func argbToColorRef(color uint32) uint32 {
	red := color & 0xFF0000
	green := color & 0x00FF00
	blue := color & 0x0000FF

	return blue | (green << 8) | (red << 16)
}
