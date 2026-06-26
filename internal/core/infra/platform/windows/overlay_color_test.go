//go:build windows

package windows //nolint:testpackage // exercises unexported color-blend helpers directly

import "testing"

func TestArgbToGDIColorRef_opaqueUsesRGB(t *testing.T) {
	const argb = uint32(0xFF17327A)

	got := argbToGDIColorRef(argb, themeSurfaceLight)
	want := rgbToColorRef(0x17, 0x32, 0x7A)

	if got != want {
		t.Fatalf("got %#x want %#x", got, want)
	}
}

func TestArgbToGDIColorRef_blendsSemiTransparentBorder(t *testing.T) {
	// #99465FBC border over light surface #EEF2FF
	const argb = uint32(0x99465FBC)

	got := argbToGDIColorRef(argb, themeSurfaceLight)

	// Expected: alpha blend over EEF2FF
	alpha := uint16(0x99)
	inv := 255 - alpha
	wantR := uint8((uint16(0x46)*alpha + uint16(0xEE)*inv) / 255)
	wantG := uint8((uint16(0x5F)*alpha + uint16(0xF2)*inv) / 255)
	wantB := uint8((uint16(0xBC)*alpha + uint16(0xFF)*inv) / 255)
	want := rgbToColorRef(wantR, wantG, wantB)

	if got != want {
		t.Fatalf("got %#x want %#x", got, want)
	}
}

func TestArgbToGDIColorRef_avoidsColorKey(t *testing.T) {
	got := argbToGDIColorRef(0xFF010101, themeSurfaceLight)

	if got == overlayColorKey {
		t.Fatalf("color key collision was not avoided")
	}
}

// winRGB mirrors the Win32 RGB macro: COLORREF is 0x00BBGGRR, so red is the
// low byte, green the middle, blue the high byte. These tests pin the COLORREF
// byte order independently of rgbToColorRef so a red/blue swap regresses loudly.
func winRGB(r, g, b uint8) uint32 {
	return uint32(r) | (uint32(g) << 8) | (uint32(b) << 16)
}

func TestRgbToColorRef_matchesWin32RGBMacro(t *testing.T) {
	cases := []struct {
		name         string
		r, g, b      uint8
		wantColorRef uint32
	}{
		{"pure red is 0x000000FF", 0xFF, 0x00, 0x00, 0x000000FF},
		{"pure green is 0x0000FF00", 0x00, 0xFF, 0x00, 0x0000FF00},
		{"pure blue is 0x00FF0000", 0x00, 0x00, 0xFF, 0x00FF0000},
		{"white is 0x00FFFFFF", 0xFF, 0xFF, 0xFF, 0x00FFFFFF},
		{"black is 0x00000000", 0x00, 0x00, 0x00, 0x00000000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rgbToColorRef(tc.r, tc.g, tc.b); got != tc.wantColorRef {
				t.Fatalf("rgbToColorRef(%#x,%#x,%#x) = %#x, want COLORREF %#x", tc.r, tc.g, tc.b, got, tc.wantColorRef)
			}
		})
	}
}

// TestArgbToGDIColorRef_pureRedBlueNotSwapped is the direct regression guard for
// issue #938: blue theme accents were rendering orange-red because red and blue
// were swapped in the COLORREF packing. A pure-blue input must yield a COLORREF
// whose high byte is blue, and pure-red must yield red in the low byte.
func TestArgbToGDIColorRef_pureRedBlueNotSwapped(t *testing.T) {
	red := argbToGDIColorRef(0xFFFF0000, themeSurfaceLight)
	if red != winRGB(0xFF, 0x00, 0x00) {
		t.Fatalf("pure red ARGB 0xFFFF0000 -> COLORREF %#x, want %#x (red in low byte)", red, winRGB(0xFF, 0x00, 0x00))
	}

	blue := argbToGDIColorRef(0xFF0000FF, themeSurfaceLight)
	if blue != winRGB(0x00, 0x00, 0xFF) {
		t.Fatalf("pure blue ARGB 0xFF0000FF -> COLORREF %#x, want %#x (blue in high byte)", blue, winRGB(0x00, 0x00, 0xFF))
	}
}
