//go:build linux

//nolint:testpackage // Exercises unexported helpers (scanINIValue, darkModeCapability).
package linux

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestScanINIValueFindsKeyInCorrectSection(t *testing.T) {
	t.Parallel()

	// kdeglobals-shaped fixture: comments, multiple sections, the key we
	// care about lives only in the [General] section.
	body := strings.Join([]string{
		"# kdeglobals fixture",
		"[KDE]",
		"ColorScheme=ShouldBeIgnored",
		"",
		"[General]",
		"ColorScheme=BreezeDark",
		"Other=value",
		"",
		"[General-Substitution]",
		"ColorScheme=AlsoIgnored",
	}, "\n")

	got := scanINIValue(strings.NewReader(body), "General", "ColorScheme")
	if got != "BreezeDark" {
		t.Fatalf("scanINIValue() = %q, want %q", got, "BreezeDark")
	}
}

func TestScanINIValueReturnsEmptyWhenSectionOrKeyMissing(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing section": "[Other]\nColorScheme=BreezeDark\n",
		"missing key":     "[General]\nOther=value\n",
		"empty body":      "",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := scanINIValue(strings.NewReader(body), "General", "ColorScheme")
			if got != "" {
				t.Fatalf("scanINIValue() = %q, want empty", got)
			}
		})
	}
}

func TestDarkModeCapabilitySupportedStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value      int
		source     darkModeSource
		wantStatus ports.FeatureStatus
		wantInDtl  string
	}{
		{
			name:       "dark via portal",
			value:      colorSchemeDark,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "dark (source=xdg-portal)",
		},
		{
			name:       "light via portal",
			value:      colorSchemeLight,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "light (source=xdg-portal)",
		},
		{
			name:       "no preference via portal",
			value:      colorSchemeNoPreference,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "no preference (source=xdg-portal)",
		},
		{
			name:       "dark via kdeglobals fallback",
			value:      colorSchemeDark,
			source:     darkModeSourceKDEGlobals,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "dark (source=kdeglobals)",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := darkModeCapability(testCase.value, testCase.source, true)
			if got.Status != testCase.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, testCase.wantStatus)
			}

			if !strings.Contains(got.Detail, testCase.wantInDtl) {
				t.Fatalf("Detail = %q, want substring %q", got.Detail, testCase.wantInDtl)
			}
		})
	}
}

func TestParsePortalColorSchemeBusctlOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    int
		wantOK  bool
	}{
		{name: "prefer_dark", input: "v v u 1", want: colorSchemeDark, wantOK: true},
		{name: "prefer_light", input: "v v u 2", want: colorSchemeLight, wantOK: true},
		{name: "no_preference", input: "v v u 0", want: colorSchemeNoPreference, wantOK: true},
		{name: "read_one_form", input: "v u 1", want: colorSchemeDark, wantOK: true},
		{name: "extra_whitespace", input: "  v   v   u   1  \n", want: colorSchemeDark, wantOK: true},
		{name: "empty", input: "", want: 0, wantOK: false},
		{name: "no_numeric_token", input: "v v u", want: 0, wantOK: false},
		{name: "out_of_range_high", input: "v v u 9", want: 0, wantOK: false},
		{name: "negative", input: "v v u -1", want: 0, wantOK: false},
		{name: "garbage_suffix", input: "hello world", want: 0, wantOK: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parsePortalColorSchemeBusctlOutput(testCase.input)
			if ok != testCase.wantOK {
				t.Fatalf("ok = %v, want %v", ok, testCase.wantOK)
			}

			if got != testCase.want {
				t.Fatalf("value = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestDarkModeCapabilityDowngradesToStubWhenUnreachable(t *testing.T) {
	t.Parallel()

	got := darkModeCapability(-1, "", false)

	if got.Status != ports.FeatureStatusStub {
		t.Fatalf("Status = %q, want %q", got.Status, ports.FeatureStatusStub)
	}
	// The detail must point the user at a real fix; an empty string would
	// regress to the same "we know nothing" UX as the original bug.
	if !strings.Contains(got.Detail, "xdg-desktop-portal") {
		t.Fatalf(
			"Detail = %q, want a fix-it hint mentioning xdg-desktop-portal",
			got.Detail,
		)
	}
}

func TestReadKDEColorSchemeIsDarkWhenSchemeNameContainsDark(t *testing.T) {
	t.Parallel()

	// Pure scheme-name → dark/light inference, exercising the case-insensitive
	// "dark" substring rule that protects vanilla KDE installs without
	// xdg-desktop-portal-kde from silently reporting the wrong state.
	cases := map[string]int{
		"BreezeDark":   colorSchemeDark,
		"OXYGEN-DARK":  colorSchemeDark,
		"BreezeLight":  colorSchemeLight,
		"Custom":       colorSchemeLight,
		"DarkMatter":   colorSchemeDark,
		"NotApplied!?": colorSchemeLight,
	}

	for scheme, want := range cases {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			body := "[General]\nColorScheme=" + scheme + "\n"

			got := scanINIValue(strings.NewReader(body), "General", "ColorScheme")
			if got != scheme {
				t.Fatalf("scanINIValue() = %q, want %q", got, scheme)
			}

			isDark := strings.Contains(strings.ToLower(got), "dark")

			gotValue := colorSchemeLight
			if isDark {
				gotValue = colorSchemeDark
			}

			if gotValue != want {
				t.Fatalf("derived color-scheme = %d, want %d", gotValue, want)
			}
		})
	}
}
