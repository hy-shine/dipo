package colour_test

import (
	"math"
	"testing"

	"github.com/hy-shine/dipo/internal/colour"
)

// The references are ColorSync's conversions, from NSColor.usingColorSpace,
// which agree with these to 0.0003 across a grid of 5,832 sRGB colors.
func TestDisplayP3MatchesColorSync(t *testing.T) {
	for _, test := range []struct {
		value colour.SRGB
		want  colour.DisplayP3
	}{
		{colour.Opaque(0, 255, 0).SRGB(), colour.DisplayP3{R: 0.45842450857162476, G: 0.9852612614631653, B: 0.29824867844581604}},
		{colour.Opaque(60, 120, 225).SRGB(), colour.DisplayP3{R: 0.2944561243057251, G: 0.46501022577285767, B: 0.8542827367782593}},
		{colour.Opaque(255, 105, 45).SRGB(), colour.DisplayP3{R: 0.9297195076942444, G: 0.44894731044769287, B: 0.25078779458999634}},
		{colour.Opaque(255, 255, 255).SRGB(), colour.DisplayP3{R: 1, G: 1, B: 1}},
		{colour.Opaque(0, 0, 0).SRGB(), colour.DisplayP3{}},
		// Display P3 green as ColorSync reads it into extended sRGB.
		{colour.SRGB{R: -0.5116, G: 1.0183, B: -0.3107}, colour.DisplayP3{R: 0, G: 1, B: 0}},
	} {
		got := test.value.DisplayP3()
		if math.Abs(got.R-test.want.R) > 0.0003 || math.Abs(got.G-test.want.G) > 0.0003 || math.Abs(got.B-test.want.B) > 0.0003 {
			t.Errorf("%+v DisplayP3 = %+v; want %+v", test.value, got, test.want)
		}
	}
}

func TestDisplayP3Text(t *testing.T) {
	gray := colour.Opaque(128, 128, 128)
	translucent := colour.Color{R: 128, G: 128, B: 128, A: 128}
	for _, test := range []struct {
		got, want string
	}{
		{gray.Text(colour.P3), "color(display-p3 0.502 0.502 0.502)"},
		{translucent.Text(colour.P3), "color(display-p3 0.502 0.502 0.502 / 50.2%)"},
		{gray.Text(colour.P3HEX), "#808080"},
		{translucent.Text(colour.P3HEX), "#80808080"},
		{colour.Opaque(0, 255, 0).Text(colour.P3HEX), "#75FB4C"},
		{colour.Opaque(60, 120, 225).Text(colour.P3HEX), "#4B77DA"},
		{colour.Opaque(255, 255, 255).Text(colour.P3), "color(display-p3 1 1 1)"},
		{colour.Opaque(0, 0, 0).Text(colour.P3), "color(display-p3 0 0 0)"},
		{colour.SRGB{R: -0.5116, G: 1.0183, B: -0.3107}.Text(colour.P3HEX, 255), "#00FF00"},
		{colour.SRGB{R: -0.5116, G: 1.0183, B: -0.3107}.Text(colour.HEX, 255), "#00FF00"},
		// Brighter than white, as HDR content is: clamped to the edge in both spaces.
		{colour.SRGB{R: 1.2, G: 1.2, B: 1.2}.Text(colour.P3, 255), "color(display-p3 1 1 1)"},
	} {
		if test.got != test.want {
			t.Errorf("Text = %q; want %q", test.got, test.want)
		}
	}
}

func TestClippedFollowsTheFormatsSpace(t *testing.T) {
	p3Green := colour.SRGB{R: -0.5116, G: 1.0183, B: -0.3107}
	hdrWhite := colour.SRGB{R: 1.2, G: 1.2, B: 1.2}
	for _, test := range []struct {
		value   colour.SRGB
		format  colour.Format
		clipped bool
	}{
		{p3Green, colour.HEX, true},
		{p3Green, colour.HSL, true},
		{p3Green, colour.P3HEX, false},
		{p3Green, colour.P3, false},
		{hdrWhite, colour.P3, true},
		{colour.Opaque(255, 105, 45).SRGB(), colour.RGB, false},
		{colour.Opaque(255, 105, 45).SRGB(), colour.P3, false},
	} {
		if got := test.value.Clipped(test.format); got != test.clipped {
			t.Errorf("%+v Clipped(%s) = %t; want %t", test.value, test.format, got, test.clipped)
		}
	}
	for _, format := range []colour.Format{colour.HEX, colour.RGB, colour.HSL, colour.P3HEX, colour.P3} {
		if !format.Valid() {
			t.Errorf("%s is not a valid format", format)
		}
	}
}

func TestSRGBInsideRoundsToEightBits(t *testing.T) {
	step := 1.0 / 255
	for _, test := range []struct {
		value  colour.SRGB
		inside bool
		hex    string
	}{
		{colour.SRGB{R: 0, G: 0.5, B: 1}, true, "#0080FF"},
		{colour.SRGB{R: -0.49 * step, G: 1 + 0.49*step, B: 0}, true, "#00FF00"},
		{colour.SRGB{R: -0.51 * step, G: 0, B: 0}, false, "#000000"},
		{colour.SRGB{R: 0, G: 1 + 0.51*step, B: 0}, false, "#00FF00"},
		{colour.SRGB{R: -0.5116, G: 1.0183, B: -0.3107}, false, "#00FF00"},
	} {
		if got := test.value.Inside(); got != test.inside {
			t.Errorf("%+v Inside = %t; want %t", test.value, got, test.inside)
		}
		if got := test.value.Color(255).Hex(); got != test.hex {
			t.Errorf("%+v Color = %s; want %s", test.value, got, test.hex)
		}
	}
}
