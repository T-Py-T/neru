//go:build windows

// internal/core/infra/accessibility/uia_windows.go
// Pure-Go IUIAutomation (COM) element discovery for the Windows hints mode.
// Does not perform actions or build a deep cached tree; it returns a flat
// list of on-screen, clickable controls for the given top-level window.

package accessibility

import (
	"image"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CGO is disabled on Windows (see justfile), so UI Automation is driven
// through raw COM vtable calls rather than a C wrapper. All COM work for a
// single enumeration happens on one locked OS thread: CoInitialize, object
// creation, property reads, and release. Every property is copied into a
// plain Go value before the COM object is released, so no COM pointer ever
// escapes this file or crosses a goroutine boundary.

var (
	modole32    = windows.NewLazySystemDLL("ole32.dll")
	modoleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

	procCoInitializeEx   = modole32.NewProc("CoInitializeEx")
	procCoUninitialize   = modole32.NewProc("CoUninitialize")
	procCoCreateInstance = modole32.NewProc("CoCreateInstance")
	procSysFreeString    = modoleaut32.NewProc("SysFreeString")
)

const (
	// Multithreaded apartment: UIA enumeration runs on a background worker
	// goroutine with no Windows message pump, so MTA is required to avoid the
	// STA cross-process marshaling deadlock (Microsoft recommends MTA for UIA
	// calls made off the UI thread).
	coinitMultithreaded = 0x0
	clsctxInprocServer  = 0x1

	// TreeScope_Descendants: every element below the root, at any depth.
	treeScopeDescendants = 0x4
)

// COM GUIDs for the default UI Automation client object and interface.
var (
	clsidCUIAutomation = windows.GUID{
		Data1: 0xff48dba4,
		Data2: 0x60ef,
		Data3: 0x4201,
		Data4: [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e},
	}
	iidIUIAutomation = windows.GUID{
		Data1: 0x30cbe57d,
		Data2: 0xd9d0,
		Data3: 0x452a,
		Data4: [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee},
	}
)

// Vtable slot indices (IUnknown occupies 0,1,2). These match the public
// UIAutomationClient IDL and have been stable since Windows 7.
const (
	vtRelease = 2

	// IUIAutomation
	vtElementFromHandle   = 6
	vtCreateTrueCondition = 21

	// IUIAutomationElement
	vtFindAll                     = 6
	vtGetCurrentControlType       = 21
	vtGetCurrentName              = 23
	vtGetCurrentIsOffscreen       = 38
	vtGetCurrentBoundingRectangle = 43

	// IUIAutomationElementArray
	vtArrayGetLength = 3
	vtArrayGetElement = 4
)

// winRect mirrors the Win32 RECT returned by get_CurrentBoundingRectangle.
type winRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

// winElement is the extracted, COM-free description of a clickable control.
type winElement struct {
	bounds    image.Rectangle
	role      string
	name      string
	clickable bool
}

// comCall invokes the method at vtable slot index on the COM object this.
// It returns the HRESULT (or boolean/handle) in the low bits of the result.
func comCall(this uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(this))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))

	full := make([]uintptr, 0, len(args)+1)
	full = append(full, this)
	full = append(full, args...)

	ret, _, _ := syscall.SyscallN(fn, full...)

	return ret
}

// failed reports whether an HRESULT indicates failure (high bit set).
func failed(hr uintptr) bool {
	return int32(hr) < 0
}

// enumerateClickableElements returns the on-screen, clickable controls of the
// given top-level window handle. It returns nil on any failure; callers treat
// an empty result as "no hints", never as a crash.
func enumerateClickableElements(hwnd uintptr) []winElement {
	if hwnd == 0 {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)

	// S_OK (0) and S_FALSE (1) mean this call owns initialization on the
	// thread and must balance it with CoUninitialize. RPC_E_CHANGED_MODE
	// means COM is already up in another mode; leave it alone.
	if uint32(hr) == 0 || uint32(hr) == 1 {
		defer procCoUninitialize.Call()
	}

	automation := createAutomation()
	if automation == 0 {
		return nil
	}
	defer comCall(automation, vtRelease)

	var root uintptr

	hr = comCall(
		automation,
		vtElementFromHandle,
		hwnd,
		uintptr(unsafe.Pointer(&root)),
	)
	if failed(hr) || root == 0 {
		return nil
	}
	defer comCall(root, vtRelease)

	var condition uintptr

	hr = comCall(automation, vtCreateTrueCondition, uintptr(unsafe.Pointer(&condition)))
	if failed(hr) || condition == 0 {
		return nil
	}
	defer comCall(condition, vtRelease)

	var array uintptr

	hr = comCall(
		root,
		vtFindAll,
		uintptr(treeScopeDescendants),
		condition,
		uintptr(unsafe.Pointer(&array)),
	)
	if failed(hr) || array == 0 {
		return nil
	}
	defer comCall(array, vtRelease)

	return collectArray(array)
}

// createAutomation creates the default IUIAutomation instance.
func createAutomation() uintptr {
	var automation uintptr

	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIUIAutomation)),
		uintptr(unsafe.Pointer(&automation)),
	)
	if failed(hr) {
		return 0
	}

	return automation
}

// collectArray walks an IUIAutomationElementArray and extracts the clickable
// controls. Each element is released as soon as its data is copied out.
func collectArray(array uintptr) []winElement {
	var length int32

	hr := comCall(array, vtArrayGetLength, uintptr(unsafe.Pointer(&length)))
	if failed(hr) || length <= 0 {
		return nil
	}

	result := make([]winElement, 0, length)

	for i := int32(0); i < length; i++ {
		var element uintptr

		hr = comCall(array, vtArrayGetElement, uintptr(i), uintptr(unsafe.Pointer(&element)))
		if failed(hr) || element == 0 {
			continue
		}

		extracted, ok := extractWinElement(element)

		comCall(element, vtRelease)

		if ok {
			result = append(result, extracted)
		}
	}

	return result
}

// extractWinElement copies the relevant properties from a single UIA element.
// It returns ok=false for non-clickable, offscreen, or zero-size controls.
func extractWinElement(element uintptr) (winElement, bool) {
	var controlType int32
	if failed(comCall(element, vtGetCurrentControlType, uintptr(unsafe.Pointer(&controlType)))) {
		return winElement{}, false
	}

	role, clickable := mapControlType(controlType)
	if !clickable {
		return winElement{}, false
	}

	var offscreen int32
	if !failed(comCall(element, vtGetCurrentIsOffscreen, uintptr(unsafe.Pointer(&offscreen)))) &&
		offscreen != 0 {
		return winElement{}, false
	}

	var rect winRect
	if failed(comCall(element, vtGetCurrentBoundingRectangle, uintptr(unsafe.Pointer(&rect)))) {
		return winElement{}, false
	}

	bounds := image.Rect(int(rect.left), int(rect.top), int(rect.right), int(rect.bottom))
	if bounds.Empty() {
		return winElement{}, false
	}

	return winElement{
		bounds:    bounds,
		role:      role,
		name:      currentName(element),
		clickable: true,
	}, true
}

// currentName reads the element's name (BSTR) and frees it.
func currentName(element uintptr) string {
	var bstr uintptr
	if failed(comCall(element, vtGetCurrentName, uintptr(unsafe.Pointer(&bstr)))) || bstr == 0 {
		return ""
	}

	name := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(bstr)))

	procSysFreeString.Call(bstr)

	return name
}

// UI Automation CONTROLTYPEID values for the controls neru treats as
// clickable hint targets, mapped onto the shared AX-style role names.
func mapControlType(controlType int32) (role string, clickable bool) {
	switch controlType {
	case 50000: // Button
		return "AXButton", true
	case 50002: // CheckBox
		return "AXCheckBox", true
	case 50003: // ComboBox
		return "AXComboBox", true
	case 50004: // Edit
		return "AXTextField", true
	case 50005: // Hyperlink
		return "AXLink", true
	case 50007: // ListItem
		return "AXCell", true
	case 50011: // MenuItem
		return "AXMenuItem", true
	case 50013: // RadioButton
		return "AXRadioButton", true
	case 50015: // Slider
		return "AXSlider", true
	case 50016: // Spinner
		return "AXIncrementor", true
	case 50019: // TabItem
		return "AXTabButton", true
	case 50024: // TreeItem
		return "AXRow", true
	case 50029: // DataItem
		return "AXCell", true
	case 50031: // SplitButton
		return "AXButton", true
	default:
		return "AXUnknown", false
	}
}
