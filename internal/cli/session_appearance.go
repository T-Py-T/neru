package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
)

func init() {
	RootCmd.AddCommand(sessionAppearanceCmd)
}

// sessionAppearanceCmd prints live session theme detection via the platform
// SystemPort adapter (same path as doctor / IPC capabilities).
//
// On Linux over SSH, import the graphical user's D-Bus/session environment
// first, e.g. eval "$(systemctl --user show-environment | sed 's/^/export /')".
var sessionAppearanceCmd = &cobra.Command{
	Use:   "session-appearance",
	Short: "Print session dark/light preference via the system adapter",
	Long: `Report current dark-mode detection using ports.SystemPort — the same
adapter the daemon uses — without requiring an IPC connection to a running Neru
process.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		sys, err := platform.NewSystemPort()
		if err != nil {
			return err
		}

		caps := sys.Capabilities()
		cmd.Printf("platform: %s\n", caps.Platform)
		cmd.Printf("dark_mode_status: %s\n", caps.DarkModeDetection.Status)
		cmd.Printf("dark_mode_detail: %s\n", caps.DarkModeDetection.Detail)
		cmd.Printf("is_dark: %v\n", sys.IsDarkMode())

		return nil
	},
}
