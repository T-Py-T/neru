package logger

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultLogFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}

	got, err := defaultLogFilePath()
	if err != nil {
		t.Fatalf("defaultLogFilePath() error = %v", err)
	}

	var want string

	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join(home, "Library", "Logs", "neru", "app.log")
	case "windows":
		want = filepath.Join(home, "AppData", "Local", "neru", "log", "app.log")
	default:
		want = filepath.Join(home, ".local", "state", "neru", "log", "app.log")
	}

	if got != want {
		t.Fatalf("defaultLogFilePath() = %q, want %q", got, want)
	}
}
