//go:build linux

package main

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

type linuxDaemonHost struct{}

func newDaemonHost() daemonHost {
	return linuxDaemonHost{}
}

func (linuxDaemonHost) Run(application *app.App) error {
	defer application.Cleanup()

	// On KDE/KWin, input is injected via libei through the RemoteDesktop
	// portal, which shows a one-time consent prompt. Warm it up at startup (in
	// the background) so the prompt appears now rather than blocking the first
	// action past the IPC timeout. No-op on wlroots / X11.
	go func() {
		if err := linux.WarmWaylandInput(); err != nil {
			// Surface at WARN: on KDE this means the libei/RemoteDesktop
			// session did not establish (consent not approved in time, or
			// portal unavailable), so the first action falls back to a slow
			// lazy connect that can exceed the IPC client timeout.
			application.Logger().Warn(
				"Wayland input warm-up failed; first action may be slow until the RemoteDesktop consent prompt is approved",
				zap.Error(err),
			)
			return
		}
		application.Logger().Info("Wayland input warm-up complete")
	}()

	return application.Run()
}
