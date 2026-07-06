//go:build linux

//nolint:testpackage // test covers unexported KDE-only gate
package accessibility

import "testing"

func TestKwinBridgeShouldRun(t *testing.T) {
	tests := []struct {
		name    string
		desktop string
		want    bool
	}{
		{name: "kde", desktop: "KDE", want: true},
		{name: "kde lowercased", desktop: "kde", want: true},
		{name: "cosmic", desktop: "COSMIC", want: false},
		{name: "gnome", desktop: "ubuntu:GNOME", want: false},
		{name: "wlroots", desktop: "sway", want: false},
		{name: "empty", desktop: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CURRENT_DESKTOP", tc.desktop)
			if got := kwinBridgeShouldRun(); got != tc.want {
				t.Fatalf("kwinBridgeShouldRun() = %v, want %v", got, tc.want)
			}
		})
	}
}
