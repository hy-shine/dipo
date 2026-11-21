package colour

import (
	"fmt"
	"math"
	"strconv"
)

// SRGB is a color in extended sRGB: sRGB's primaries and transfer curve, with
// components that may pass 0 and 1. A screen pick arrives in it, so a color
// that sRGB cannot show keeps how far outside it lies.
type SRGB struct{ R, G, B float64 }

// DisplayP3 is a color in Display P3, the space of Mac displays: sRGB's white
// point and transfer curve with wider primaries. Inside its gamut the
// components run from 0 to 1.
type DisplayP3 struct{ R, G, B float64 }

// SRGB returns value's components on the 0 to 1 scale, without its alpha.
func (value Color) SRGB() SRGB {
	return SRGB{float64(value.R) / 255, float64(value.G) / 255, float64(value.B) / 255}
}

// DisplayP3 converts c to Display P3. Every sRGB color lies inside Display
// P3; a color outside sRGB may lie outside it too.
func (c SRGB) DisplayP3() DisplayP3 {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	m := &srgbToP3
	return DisplayP3{
		encoded(m[0][0]*r + m[0][1]*g + m[0][2]*b),
		encoded(m[1][0]*r + m[1][1]*g + m[1][2]*b),
		encoded(m[2][0]*r + m[2][1]*g + m[2][2]*b),
	}
}

// Inside reports whether Display P3 can show c, by SRGB.Inside's rule.
func (c DisplayP3) Inside() bool {
	return inGamut(c.R) && inGamut(c.G) && inGamut(c.B)
}

// Text writes c with alpha in format. A color outside the format's space
// comes out as the nearest color on its edge, which Clipped reports.
func (c SRGB) Text(format Format, alpha uint8) string {
	switch format {
	case P3HEX:
		p3 := c.DisplayP3()
		return Color{channel(p3.R), channel(p3.G), channel(p3.B), alpha}.Hex()
	case P3:
		p3 := c.DisplayP3()
		return fmt.Sprintf("color(display-p3 %s %s %s%s)", p3Component(p3.R), p3Component(p3.G), p3Component(p3.B), alphaSuffix(alpha))
	default:
		return c.Color(alpha).Text(format)
	}
}

// Clipped reports whether format cannot write c as it is because c lies
// outside the format's space.
func (c SRGB) Clipped(format Format) bool {
	if format.DisplayP3() {
		return !c.DisplayP3().Inside()
	}
	return !c.Inside()
}

// p3Component writes a Display P3 component clamped to the gamut with up to
// four decimals, finer than an 8-bit step, as CSS takes it.
func p3Component(value float64) string {
	return strconv.FormatFloat(math.Round(max(0, min(1, value))*10000)/10000, 'f', -1, 64)
}

// linear and encoded apply sRGB's transfer curve, which Display P3 shares,
// mirrored for negative components as extended sRGB does.
func linear(component float64) float64 {
	magnitude := math.Abs(component)
	if magnitude <= 0.04045 {
		return component / 12.92
	}
	return math.Copysign(math.Pow((magnitude+0.055)/1.055, 2.4), component)
}

func encoded(component float64) float64 {
	magnitude := math.Abs(component)
	if magnitude <= 0.0031308 {
		return component * 12.92
	}
	return math.Copysign(1.055*math.Pow(magnitude, 1/2.4)-0.055, component)
}

type matrix [3][3]float64

func (a matrix) times(b matrix) matrix {
	var product matrix
	for row := range 3 {
		for column := range 3 {
			for k := range 3 {
				product[row][column] += a[row][k] * b[k][column]
			}
		}
	}
	return product
}

// srgbToP3 takes linear sRGB to linear Display P3 through CIE XYZ, with the
// matrices from CSS Color 4's sample conversion code.
var srgbToP3 = matrix{
	{2.493496911941425, -0.9313836179191239, -0.40271078445071684},
	{-0.8294889695615747, 1.7626640603183463, 0.023624685841943577},
	{0.03584583024378447, -0.07617238926804182, 0.9568845240076872},
}.times(matrix{
	{0.41239079926595934, 0.357584339383878, 0.1804807884018343},
	{0.21263900587151027, 0.715168678767756, 0.07219231536073371},
	{0.01933081871559182, 0.11919477979462598, 0.9505321522496607},
})

// Color is the nearest sRGB Color: each component clamped to the gamut and
// rounded to 8 bits.
func (c SRGB) Color(alpha uint8) Color {
	return Color{channel(c.R), channel(c.G), channel(c.B), alpha}
}

// Inside reports whether sRGB can show c: every component rounds to an 8-bit
// value without clamping. A component within half a step past 0 or 1, as
// capture noise at black or white is, still counts as inside. The picker's
// loupe draws the same line.
func (c SRGB) Inside() bool {
	return inGamut(c.R) && inGamut(c.G) && inGamut(c.B)
}

func inGamut(component float64) bool {
	return component*255 >= -0.5 && component*255 <= 255.5
}
