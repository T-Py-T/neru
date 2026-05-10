//go:build linux

package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

func init() {
	RootCmd.AddCommand(sessionAppearanceCmd)
}

// sessionAppearanceCmd prints live session theme detection (portal + KDE
// fallback). For SSH, import the graphical user's environment first, e.g.:
//
//	eval "$(systemctl --user show-environment | sed 's/^/export /')"
var sessionAppearanceCmd = &cobra.Command{
	Use:   "session-appearance",
	Short: "Print session dark/light preference (Linux only)",
	Long: `Report the current freedesktop appearance color-scheme (and KDE
kdeglobals fallback) without starting the Neru daemon.

Use the same D-Bus session as your desktop (e.g. import systemd user
environment over SSH before running this command).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Printf("linux_backend_env: %s\n", platform.DetectLinuxBackend().String())

		cap, isDark := linux.SessionAppearance()
		cmd.Printf("dark_mode_status: %s\n", cap.Status)
		cmd.Printf("dark_mode_detail: %s\n", cap.Detail)
		cmd.Printf("is_dark: %v\n", isDark)

		return nil
	},
}
