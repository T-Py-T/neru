//go:build linux

// internal/core/infra/platform/linux/capability_probe_linux.go
// Live host probes for the Linux capabilities that actually vary per
// desktop/compositor: AT-SPI accessibility reachability and evdev input access.
// It does NOT probe overlay/cursor/screen (those are verified at runtime by the
// daemon's own component health checks) and never injects input or mutates state.

package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	// capabilityProbeTimeout bounds the D-Bus calls so `neru doctor` can never
	// hang on a wedged accessibility registry.
	capabilityProbeTimeout = 2 * time.Second

	a11yProbeBusDest = "org.a11y.Bus"
	a11yProbeBusPath = dbus.ObjectPath("/org/a11y/bus")

	atspiProbeRegistryDest  = "org.a11y.atspi.Registry"
	atspiProbeRegistryRoot  = dbus.ObjectPath("/org/a11y/atspi/accessible/root")
	atspiProbeAccessibleIfc = "org.a11y.atspi.Accessible"

	evdevInputDir = "/dev/input"
)

func supportedFeature(detail string) ports.FeatureCapability {
	return ports.FeatureCapability{Status: ports.FeatureStatusSupported, Detail: detail}
}

func stubFeature(detail string) ports.FeatureCapability {
	return ports.FeatureCapability{Status: ports.FeatureStatusStub, Detail: detail}
}

// probeAccessibility reports whether the AT-SPI accessibility bus is reachable
// and how many applications are registered on it. AT-SPI is the only Linux
// accessibility path Neru uses (hints walk the AT-SPI tree), and whether it
// works is entirely host-dependent: at-spi2-registryd must be running and
// toolkits must expose their trees. A failed probe downgrades to stub with a
// fix-it hint, which makes `neru doctor` exit non-zero.
func probeAccessibility() ports.FeatureCapability {
	ctx, cancel := context.WithTimeout(context.Background(), capabilityProbeTimeout)
	defer cancel()

	session, err := dbus.SessionBus()
	if err != nil {
		return stubFeature("no D-Bus session bus: " + err.Error())
	}

	var addr string

	getAddrErr := session.Object(a11yProbeBusDest, a11yProbeBusPath).
		CallWithContext(ctx, "org.a11y.Bus.GetAddress", 0).Store(&addr)
	if getAddrErr != nil || addr == "" {
		return stubFeature(
			"AT-SPI bus not answering (org.a11y.Bus); ensure at-spi2-registryd is installed and running",
		)
	}

	a11yConn, connErr := dbus.Connect(addr)
	if connErr != nil {
		return stubFeature("AT-SPI bus address found but connection failed: " + connErr.Error())
	}
	defer func() { _ = a11yConn.Close() }()

	var childCount dbus.Variant

	getErr := a11yConn.Object(atspiProbeRegistryDest, atspiProbeRegistryRoot).
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
			atspiProbeAccessibleIfc, "ChildCount").Store(&childCount)
	if getErr != nil {
		return stubFeature("AT-SPI registry not responding; ensure at-spi2-registryd is running")
	}

	count, _ := childCount.Value().(int32)

	return supportedFeature(fmt.Sprintf("AT-SPI reachable; %d application(s) registered", count))
}

// probeEvdevInput reports whether Neru can read evdev devices, which is what
// the Wayland global-hotkey listener and keyboard event tap depend on. The
// common failure is the user not being in the "input" group, so a permission
// failure downgrades to stub with that exact fix-it hint.
func probeEvdevInput() ports.FeatureCapability {
	entries, err := os.ReadDir(evdevInputDir)
	if err != nil {
		return stubFeature("cannot read " + evdevInputDir + ": " + err.Error())
	}

	var (
		eventDevices int
		readable     int
		permDenied   bool
	)

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "event") {
			continue
		}

		eventDevices++

		path := filepath.Join(evdevInputDir, entry.Name())

		file, openErr := os.OpenFile(path, os.O_RDONLY, 0)
		if openErr != nil {
			if os.IsPermission(openErr) {
				permDenied = true
			}

			continue
		}

		_ = file.Close()
		readable++
	}

	switch {
	case eventDevices == 0:
		return stubFeature("no " + evdevInputDir + "/event* devices found")
	case readable == 0 && permDenied:
		return stubFeature(fmt.Sprintf(
			"no readable %s devices (%d present, permission denied); add your user to the 'input' group and re-login",
			evdevInputDir, eventDevices,
		))
	case readable == 0:
		return stubFeature(fmt.Sprintf("no readable %s devices (%d present)", evdevInputDir, eventDevices))
	default:
		return supportedFeature(fmt.Sprintf(
			"evdev input accessible (%d/%d %s devices readable)",
			readable, eventDevices, evdevInputDir,
		))
	}
}
