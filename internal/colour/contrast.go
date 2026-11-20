package colour

import (
	"errors"
	"math"
)

var ErrTransparentBackground = errors.New("contrast requires an opaque background")

func (value Color) Over(background Color) Color {
	foregroundAlpha, backgroundAlpha := float64(value.A)/255, float64(background.A)/255
	alpha := foregroundAlpha + backgroundAlpha*(1-foregroundAlpha)
	if alpha == 0 {
		return Color{}
	}
	blend := func(foreground, base uint8) uint8 {
		return channel((float64(foreground)/255*foregroundAlpha + float64(base)/255*backgroundAlpha*(1-foregroundAlpha)) / alpha)
	}
	return Color{blend(value.R, background.R), blend(value.G, background.G), blend(value.B, background.B), channel(alpha)}
}

func Contrast(foreground, background Color) (float64, error) {
	if background.A != 255 {
		return 0, ErrTransparentBackground
	}
	alpha := float64(foreground.A) / 255
	blend := func(component, base uint8) float64 {
		return (float64(component)*alpha + float64(base)*(1-alpha)) / 255
	}
	first := luminance(blend(foreground.R, background.R), blend(foreground.G, background.G), blend(foreground.B, background.B))
	second := luminance(float64(background.R)/255, float64(background.G)/255, float64(background.B)/255)
	return (max(first, second) + 0.05) / (min(first, second) + 0.05), nil
}

func luminance(red, green, blue float64) float64 {
	linear := func(value float64) float64 {
		if value <= 0.04045 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(red) + 0.7152*linear(green) + 0.0722*linear(blue)
}
