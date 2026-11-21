package colour

import (
	"fmt"
	"math"
	"strconv"
)

type Color struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

type Format string

const (
	HEX Format = "HEX"
	RGB Format = "RGB"
	HSL Format = "HSL"
	// P3HEX is HEX in Display P3, as design tools read it in a Display P3
	// document.
	P3HEX Format = "P3HEX"
	// P3 is CSS color(display-p3 r g b).
	P3 Format = "P3"
)

func (format Format) Valid() bool {
	return format == HEX || format == RGB || format == HSL || format.DisplayP3()
}

// DisplayP3 reports whether format writes Display P3 rather than sRGB.
func (format Format) DisplayP3() bool {
	return format == P3HEX || format == P3
}

func Opaque(red, green, blue uint8) Color {
	return Color{R: red, G: green, B: blue, A: 255}
}

func (value Color) Hex() string {
	if value.A != 255 {
		return fmt.Sprintf("#%02X%02X%02X%02X", value.R, value.G, value.B, value.A)
	}
	return fmt.Sprintf("#%02X%02X%02X", value.R, value.G, value.B)
}

func (value Color) Text(format Format) string {
	if format.DisplayP3() {
		return value.SRGB().Text(format, value.A)
	}
	alpha := alphaSuffix(value.A)
	switch format {
	case RGB:
		return fmt.Sprintf("rgb(%d %d %d%s)", value.R, value.G, value.B, alpha)
	case HSL:
		hue, saturation, lightness := value.HSL()
		return fmt.Sprintf("hsl(%s %s%% %s%%%s)", decimal(hue), decimal(saturation), decimal(lightness), alpha)
	default:
		return value.Hex()
	}
}

// alphaSuffix is the " / 50%" a CSS color function writes for a translucent
// color, and nothing for an opaque one.
func alphaSuffix(alpha uint8) string {
	if alpha == 255 {
		return ""
	}
	return " / " + decimal(float64(alpha)/255*100) + "%"
}

func decimal(value float64) string {
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', -1, 64)
}

func channel(value float64) uint8 {
	return uint8(math.Round(max(0, min(1, value)) * 255))
}

func FromHSV(hue, saturation, brightness float64, alpha uint8) Color {
	hue = math.Mod(math.Mod(hue, 360)+360, 360) / 60
	saturation = max(0, min(100, saturation)) / 100
	brightness = max(0, min(100, brightness)) / 100
	chroma := brightness * saturation
	second := chroma * (1 - math.Abs(math.Mod(hue, 2)-1))
	minimum := brightness - chroma
	var red, green, blue float64
	switch int(hue) {
	case 0:
		red, green = chroma, second
	case 1:
		red, green = second, chroma
	case 2:
		green, blue = chroma, second
	case 3:
		green, blue = second, chroma
	case 4:
		red, blue = second, chroma
	case 5:
		red, blue = chroma, second
	}
	return Color{channel(red + minimum), channel(green + minimum), channel(blue + minimum), alpha}
}

func FromHSL(hue, saturation, lightness float64, alpha uint8) Color {
	saturation = max(0, min(100, saturation)) / 100
	lightness = max(0, min(100, lightness)) / 100
	brightness := lightness + saturation*min(lightness, 1-lightness)
	hsvSaturation := 0.0
	if brightness != 0 {
		hsvSaturation = 2 * (1 - lightness/brightness)
	}
	return FromHSV(hue, hsvSaturation*100, brightness*100, alpha)
}

func (value Color) HSV() (hue, saturation, brightness float64) {
	red, green, blue := float64(value.R)/255, float64(value.G)/255, float64(value.B)/255
	highest, lowest := max(red, green, blue), min(red, green, blue)
	delta := highest - lowest
	if delta != 0 {
		switch highest {
		case red:
			hue = math.Mod((green-blue)/delta, 6)
		case green:
			hue = (blue-red)/delta + 2
		case blue:
			hue = (red-green)/delta + 4
		}
		hue = math.Mod(hue*60+360, 360)
	}
	if highest != 0 {
		saturation = delta / highest * 100
	}
	return hue, saturation, highest * 100
}

func (value Color) HSL() (hue, saturation, lightness float64) {
	hue, _, brightness := value.HSV()
	highest := brightness / 100
	lowest := float64(min(value.R, value.G, value.B)) / 255
	lightness = (highest + lowest) / 2
	if highest != lowest {
		saturation = (highest - lowest) / (1 - math.Abs(2*lightness-1)) * 100
	}
	return hue, saturation, lightness * 100
}

func (value Color) MarshalText() ([]byte, error) {
	return []byte(value.Hex()), nil
}

func (value *Color) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
