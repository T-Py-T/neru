package logger

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultLogFilePath returns the platform default neru log file path when
// [logging].log_file is empty. Layouts match ports.SystemPort.LogDir() for
// darwin, linux, and windows.
func defaultLogFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var logDir string

	switch runtime.GOOS {
	case "darwin":
		logDir = filepath.Join(homeDir, "Library", "Logs", "neru")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}

		logDir = filepath.Join(localAppData, "neru", "log")
	default:
		// Linux, *BSD, etc. — XDG-style state dir (matches linux SystemAdapter.LogDir).
		logDir = filepath.Join(homeDir, ".local", "state", "neru", "log")
	}

	return filepath.Join(logDir, "app.log"), nil
}
