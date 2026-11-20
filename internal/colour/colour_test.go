package colour_test

import (
	"errors"
	"math"
	"testing"

	"github.com/hy-shine/dipo/internal/colour"
)

func TestParseAndFormat(t *testing.T) {
	for _, test := range []struct {
		input string
		hex   string
	}{
		{"#13161B", "#13161B"},
		{" #abc ", "#AABBCC"},
		{"#abcd", "#AABBCCDD"},
		{"rgb(19 22 27)", "#13161B"},
		{"rgb(19, 22, 27)", "#13161B"},
		{"rgba(19,22,27,0.5)", "#13161B80"},
		{"rgb(100% 0% 0% / 50%)", "#FF000080"},
		{"hsl(217.5 17.3913% 9.0196%)", "#13161B"},
		{"hsla(120,100%,50%,0.25)", "#00FF0040"},
		{"hsl(-120deg 100% 50%)", "#0000FF"},
		{"hsl(0.5turn 100% 50%)", "#00FFFF"},
		{"hsl(200grad 100% 50%)", "#00FFFF"},
		{"rgb(-20 999 0)", "#00FF00"},
	} {
		t.Run(test.input, func(t *testing.T) {
			value, err := colour.Parse(test.input)
			if err != nil || value.Hex() != test.hex {
				t.Fatalf("Parse(%q) = %s, %v; want %s", test.input, value.Hex(), err, test.hex)
			}
			for _, format := range []colour.Format{colour.HEX, colour.RGB, colour.HSL} {
				reparsed, err := colour.Parse(value.Text(format))
				if err != nil || reparsed != value {
					t.Fatalf("roundtrip %s = %v, %v; want %v", value.Text(format), reparsed, err, value)
				}
			}
		})
	}
}

func TestRejectInvalidColor(t *testing.T) {
	for _, input := range []string{"", "13161B", "#12", "#GG0000", "#+12345", "#-12345", "rgb(1 2)", "rgb(1 2 3 4)", "rgb(1, 2 3)", "rgb(1,2,3 / .5)", "rgb(1%,2,3)", "hsl(20 30 40)", "rgb(NaN 0 0)", "hsl(inf 100% 50%)", "color(display-p3 1 0 0)", "rgb(1 2 3 /)", "rgb(1 2 3))", "rgb(0x1p2 0 0)", "rgb(1_0 0 0)", "rgb(1. 0 0)", "hsl(1_0deg 100% 50%)", "rgb(1 2 3 / 1_0%)"} {
		if _, err := colour.Parse(input); !errors.Is(err, colour.ErrSyntax) {
			t.Errorf("Parse(%q) error = %v; want ErrSyntax", input, err)
		}
	}
}

func TestDecimalCSSNumbers(t *testing.T) {
	for _, test := range []struct {
		input string
		hex   string
	}{
		{"RGB(+1.9e1 2.2e1 27 / 5e1%)", "#13161B80"},
		{"rgb(.5e2% 0 0)", "#800000"},
		{"hsl(1e308turn 100% 50%)", "#FF0000"},
	} {
		value, err := colour.Parse(test.input)
		if err != nil || value.Hex() != test.hex {
			t.Errorf("Parse(%q) = %s, %v; want %s", test.input, value.Hex(), err, test.hex)
		}
	}
}

func TestModelRoundTrips(t *testing.T) {
	for red := 0; red < 256; red += 17 {
		for green := 0; green < 256; green += 17 {
			for blue := 0; blue < 256; blue += 17 {
				value := colour.Opaque(uint8(red), uint8(green), uint8(blue))
				hue, saturation, brightness := value.HSV()
				if got := colour.FromHSV(hue, saturation, brightness, 255); got != value {
					t.Fatalf("HSV roundtrip %v = %v", value, got)
				}
				hue, saturation, lightness := value.HSL()
				if got := colour.FromHSL(hue, saturation, lightness, 255); got != value {
					t.Fatalf("HSL roundtrip %v = %v", value, got)
				}
			}
		}
	}
}

func TestContrast(t *testing.T) {
	black, white := colour.Opaque(0, 0, 0), colour.Opaque(255, 255, 255)
	for _, test := range []struct {
		foreground colour.Color
		background colour.Color
		want       float64
	}{
		{black, white, 21},
		{white, black, 21},
		{white, white, 1},
		{colour.Opaque(119, 119, 119), white, 4.478089453577214},
		{colour.Opaque(118, 118, 118), white, 4.542224959605253},
	} {
		got, err := colour.Contrast(test.foreground, test.background)
		if err != nil || math.Abs(got-test.want) > 0.000001 {
			t.Errorf("Contrast(%v, %v) = %v, %v; want %v", test.foreground, test.background, got, err, test.want)
		}
	}
	if _, err := colour.Contrast(black, colour.Color{}); !errors.Is(err, colour.ErrTransparentBackground) {
		t.Fatalf("transparent background error = %v", err)
	}
	transparentBlack := black
	transparentBlack.A = 0
	if got, err := colour.Contrast(transparentBlack, white); err != nil || got != 1 {
		t.Fatalf("transparent foreground = %v, %v; want 1", got, err)
	}
}

func TestTransparentContrastKeepsCompositingPrecision(t *testing.T) {
	foreground := colour.Color{R: 1, G: 1, B: 1, A: 137}
	background := colour.Opaque(255, 255, 255)
	ratio, err := colour.Contrast(foreground, background)
	if err != nil || math.Abs(ratio-4.507625357230237) > 1e-12 || ratio < 4.5 {
		t.Fatalf("transparent contrast = %.15f, %v; want 4.507625357230237 and AA pass", ratio, err)
	}
}

func TestOutputRoundTripsAcrossChannels(t *testing.T) {
	for red := range 256 {
		for green := range 256 {
			blue := uint8((red*73 + green*151) % 256)
			value := colour.Color{R: uint8(red), G: uint8(green), B: blue, A: uint8((red + green) % 256)}
			for _, format := range []colour.Format{colour.RGB, colour.HSL} {
				reparsed, err := colour.Parse(value.Text(format))
				if err != nil || reparsed != value {
					t.Fatalf("roundtrip %s = %v, %v; want %v", value.Text(format), reparsed, err, value)
				}
			}
		}
	}
}

func FuzzParse(fuzz *testing.F) {
	for _, seed := range []string{"#13161B", "rgb(1 2 3 / 50%)", "hsl(217.5 17.4% 9%)", "#abcd", "rgb(NaN 0 0)"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, input string) {
		value, err := colour.Parse(input)
		if err != nil {
			return
		}
		if got, err := colour.Parse(value.Hex()); err != nil || got != value {
			t.Fatalf("hex roundtrip = %v, %v; want %v", got, err, value)
		}
	})
}
