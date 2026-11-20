package colour

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var ErrSyntax = errors.New("unsupported color syntax")

func Parse(input string) (Color, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if strings.HasPrefix(input, "#") {
		return parseHex(input[1:])
	}
	name, body, found := strings.Cut(input, "(")
	if !found || !strings.HasSuffix(body, ")") {
		return Color{}, ErrSyntax
	}
	body = strings.TrimSuffix(body, ")")
	if name != "rgb" && name != "rgba" && name != "hsl" && name != "hsla" {
		return Color{}, ErrSyntax
	}
	var parts []string
	if strings.Contains(body, ",") {
		if strings.Contains(body, "/") {
			return Color{}, ErrSyntax
		}
		parts = strings.Split(body, ",")
	} else {
		channels, alpha, hasAlpha := strings.Cut(body, "/")
		parts = strings.Fields(channels)
		if len(parts) != 3 {
			return Color{}, ErrSyntax
		}
		if hasAlpha {
			parts = append(parts, alpha)
		}
	}
	if len(parts) != 3 && len(parts) != 4 {
		return Color{}, ErrSyntax
	}
	alpha := uint8(255)
	if len(parts) == 4 {
		value, err := component(parts[3], 1, false)
		if err != nil {
			return Color{}, err
		}
		alpha = channel(value)
	}
	if strings.HasPrefix(name, "rgb") {
		var channels [3]uint8
		legacyPercentage := strings.HasSuffix(strings.TrimSpace(parts[0]), "%")
		for index := range 3 {
			if strings.Contains(body, ",") && strings.HasSuffix(strings.TrimSpace(parts[index]), "%") != legacyPercentage {
				return Color{}, ErrSyntax
			}
			value, err := component(parts[index], 255, false)
			if err != nil {
				return Color{}, err
			}
			channels[index] = channel(value)
		}
		return Color{channels[0], channels[1], channels[2], alpha}, nil
	}
	hue, err := angle(parts[0])
	if err != nil {
		return Color{}, err
	}
	saturation, err := component(parts[1], 100, true)
	if err != nil {
		return Color{}, err
	}
	lightness, err := component(parts[2], 100, true)
	if err != nil {
		return Color{}, err
	}
	return FromHSL(hue, saturation*100, lightness*100, alpha), nil
}

func parseHex(text string) (Color, error) {
	if len(text) == 3 || len(text) == 4 {
		var expanded strings.Builder
		for _, digit := range text {
			expanded.WriteRune(digit)
			expanded.WriteRune(digit)
		}
		text = expanded.String()
	}
	if len(text) != 6 && len(text) != 8 {
		return Color{}, ErrSyntax
	}
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("%w: %v", ErrSyntax, err)
	}
	if len(text) == 6 {
		value = value<<8 | 255
	}
	return Color{uint8(value >> 24), uint8(value >> 16), uint8(value >> 8), uint8(value)}, nil
}

func component(text string, maximum float64, percentageRequired bool) (float64, error) {
	text = strings.TrimSpace(text)
	if percentage, ok := strings.CutSuffix(text, "%"); ok {
		text, maximum = percentage, 100
	} else if percentageRequired {
		return 0, ErrSyntax
	}
	value, err := number(text)
	if err != nil {
		return 0, ErrSyntax
	}
	return max(0, min(1, value/maximum)), nil
}

func angle(text string) (float64, error) {
	text = strings.TrimSpace(text)
	multiplier := 1.0
	for _, unit := range []struct {
		suffix string
		factor float64
	}{{"grad", 0.9}, {"turn", 360}, {"rad", 180 / math.Pi}, {"deg", 1}} {
		if number, ok := strings.CutSuffix(text, unit.suffix); ok {
			text, multiplier = number, unit.factor
			break
		}
	}
	value, err := number(text)
	if err != nil {
		return 0, ErrSyntax
	}
	return math.Mod(math.Mod(value, 360/multiplier)*multiplier+360, 360), nil
}

func number(text string) (float64, error) {
	position := 0
	if position < len(text) && (text[position] == '+' || text[position] == '-') {
		position++
	}
	integerStart := position
	for position < len(text) && text[position] >= '0' && text[position] <= '9' {
		position++
	}
	integerDigits := position - integerStart
	fractionDigits := 0
	if position < len(text) && text[position] == '.' {
		position++
		fractionStart := position
		for position < len(text) && text[position] >= '0' && text[position] <= '9' {
			position++
		}
		fractionDigits = position - fractionStart
		if fractionDigits == 0 {
			return 0, ErrSyntax
		}
	}
	if integerDigits+fractionDigits == 0 {
		return 0, ErrSyntax
	}
	if position < len(text) && (text[position] == 'e' || text[position] == 'E') {
		position++
		if position < len(text) && (text[position] == '+' || text[position] == '-') {
			position++
		}
		exponentStart := position
		for position < len(text) && text[position] >= '0' && text[position] <= '9' {
			position++
		}
		if position == exponentStart {
			return 0, ErrSyntax
		}
	}
	if position != len(text) {
		return 0, ErrSyntax
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, ErrSyntax
	}
	return value, nil
}
