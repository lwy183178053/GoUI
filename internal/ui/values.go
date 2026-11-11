package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"
)

func parseColor(value string) (color.NRGBA, error) {
	if (len(value) != 7 && len(value) != 9) || value[0] != '#' {
		return color.NRGBA{}, strconv.ErrSyntax
	}
	n, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, err
	}
	if len(value) == 7 {
		n = n<<8 | 0xff
	}
	return color.NRGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, nil
}

func integers(value string, count int) ([]int, error) {
	parts := strings.Split(value, ",")
	if len(parts) != count {
		return nil, strconv.ErrSyntax
	}
	result := make([]int, count)
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		result[i] = n
	}
	return result, nil
}

func ratio(value string) (Vec2, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return Vec2{}, strconv.ErrSyntax
	}
	var values [2]float64
	for i, part := range parts {
		n, err := number(strings.TrimSpace(part))
		if err != nil || n < 0 || n > 1 {
			return Vec2{}, strconv.ErrSyntax
		}
		values[i] = n
	}
	return Vec2{values[0], values[1]}, nil
}

func number(value string) (float64, error) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, strconv.ErrSyntax
	}
	return n, nil
}

func boolean(value string) (bool, error) {
	if value != "true" && value != "false" {
		return false, strconv.ErrSyntax
	}
	return value == "true", nil
}
