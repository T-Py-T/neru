package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var (
	errDaemonNotRunning  = errors.New("daemon not running")
	errDaemonUnreachable = errors.New("daemon unreachable")
)

// DoctorCmd is the CLI doctor command.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive diagnostics",
	Long: `Run a comprehensive health check of the Neru system.

This command performs client-side checks (IPC socket, config) first,
then queries the running daemon for component-level health status
(accessibility permissions, overlay state, input monitoring).

Runs client-side checks even when the daemon is not running, so you
can use it to verify accessibility permissions before launching.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Println("Neru Doctor — pre-flight checks")
		cmd.Println()
		// --- client-side checks (no daemon needed) --------------------------
		// Check IPC socket exists
		socketPath := ipc.SocketPath()

		_, statErr := os.Stat(socketPath)
		if statErr != nil {
			cmd.Printf("  ❌ %-24s %s\n", "ipc_socket", "not found: "+socketPath)
			cmd.Println()
			cmd.Println("The neru daemon does not appear to be running.")
			cmd.Println("Start it with: neru launch")

			return &silentError{err: errDaemonNotRunning}
		}

		cmd.Printf("  ✅ %-24s %s\n", "ipc_socket", socketPath)
		cmd.Println()
		// --- daemon-side checks (via IPC) -----------------------------------
		cmd.Println("Querying daemon...")
		cmd.Println()

		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err != nil {
			cmd.Printf("  ❌ %-24s %s\n", "daemon", "unreachable")
			cmd.Println()
			cmd.Println("The daemon socket exists but is not responding.")
			cmd.Println("Try restarting: neru launch")

			return &silentError{err: errDaemonUnreachable}
		}

		err = formatter.PrintHealth(cmd, ipcResponse.Success, ipcResponse.Data)

		// doctor is informational: the printed report (with per-item fix-it
		// hints) is the result. Unimplemented or degraded capabilities are
		// shown but must not fail the command, mirroring macOS where there is
		// nothing to fail on. Only genuine errors (bad payload) propagate.
		if errors.Is(err, cliutil.ErrUnhealthy) {
			return nil
		}

		return err
	},
}

func init() {
	RootCmd.AddCommand(DoctorCmd)
}
