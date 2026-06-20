//go:build linux

// internal/core/infra/platform/linux/gnome_shell_linux.go
// D-Bus client for the Neru GNOME Shell overlay extension (org.neru.ShellOverlay):
// sends overlay draw frames and queries monitor / active-window geometry from
// Mutter via the extension. It does NOT render anything itself or handle input.

package linux

import (
	"encoding/json"
	"image"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	gnomeOverlayBusName    = "org.neru.ShellOverlay"
	gnomeOverlayObjectPath = "/org/neru/ShellOverlay"
	gnomeOverlayIface      = "org.neru.ShellOverlay"
)

// GNOMEMonitor describes one monitor in global logical coordinates as reported
// by the extension (Mutter is the authoritative source on GNOME).
type GNOMEMonitor struct {
	X       int     `json:"x"`
	Y       int     `json:"y"`
	W       int     `json:"w"`
	H       int     `json:"h"`
	Primary bool    `json:"primary"`
	Scale   float64 `json:"scale"`
}

type gnomeShellClient struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

var gnomeShell = &gnomeShellClient{}

// object returns a live D-Bus object for the extension, connecting lazily.
// The session-bus connection is cached and reused; a dead connection is
// transparently replaced on the next call.
func (c *gnomeShellClient) object() (dbus.BusObject, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.conn.Connected() {
		conn, err := dbus.SessionBus()
		if err != nil {
			return nil, err
		}

		c.conn = conn
	}

	return c.conn.Object(gnomeOverlayBusName, dbus.ObjectPath(gnomeOverlayObjectPath)), nil
}

func (c *gnomeShellClient) call(method string, args ...any) ([]any, error) {
	obj, err := c.object()
	if err != nil {
		return nil, err
	}

	call := obj.Call(gnomeOverlayIface+"."+method, 0, args...)
	if call.Err != nil {
		return nil, call.Err
	}

	return call.Body, nil
}

// GNOMEShellAvailable reports whether the Neru overlay extension is running and
// answering on the session bus.
func GNOMEShellAvailable() bool {
	body, err := gnomeShell.call("Ping")
	if err != nil || len(body) == 0 {
		return false
	}

	_, ok := body[0].(string)

	return ok
}

// GNOMEShellRender replaces the overlay scene with the given frame JSON and
// repaints. The frame is {"prims":[...]} where each primitive is rect/rrect/text.
func GNOMEShellRender(frameJSON string) error {
	_, err := gnomeShell.call("Render", frameJSON)

	return err
}

// GNOMEShellClear clears the overlay scene.
func GNOMEShellClear() error {
	_, err := gnomeShell.call("Clear")

	return err
}

// GNOMEShellShow makes the overlay actor visible.
func GNOMEShellShow() error {
	_, err := gnomeShell.call("Show")

	return err
}

// GNOMEShellHide hides the overlay actor.
func GNOMEShellHide() error {
	_, err := gnomeShell.call("Hide")

	return err
}

// GNOMEShellMonitors returns the monitor layout from Mutter via the extension.
func GNOMEShellMonitors() ([]GNOMEMonitor, error) {
	body, err := gnomeShell.call("GetMonitors")
	if err != nil {
		return nil, err
	}

	raw, _ := body[0].(string)

	var monitors []GNOMEMonitor
	if err := json.Unmarshal([]byte(raw), &monitors); err != nil {
		return nil, err
	}

	return monitors, nil
}

type gnomeActiveWindowRect struct {
	OK      bool   `json:"ok"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
	Title   string `json:"title"`
	WMClass string `json:"wmclass"`
}

// GNOMEShellActiveWindowRect returns the focused window's frame rectangle in
// global logical coordinates. ok is false when there is no focused window or
// the extension is unavailable.
func GNOMEShellActiveWindowRect() (image.Rectangle, bool) {
	body, err := gnomeShell.call("GetActiveWindowRect")
	if err != nil || len(body) == 0 {
		return image.Rectangle{}, false
	}

	raw, _ := body[0].(string)

	var rect gnomeActiveWindowRect
	if err := json.Unmarshal([]byte(raw), &rect); err != nil || !rect.OK {
		return image.Rectangle{}, false
	}

	return image.Rect(rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H), true
}
