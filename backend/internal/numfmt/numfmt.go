// Package numfmt formatea números como lo hacen las tablas y los textos del proyecto (miles con coma, signo explícito).
//
// Fecha: 2026-11-19
package numfmt

import (
	"math"
	"strconv"
	"strings"
)

func Comma(x float64, decimals int) string {
	if math.IsNaN(x) {
		return "nan"
	}
	if math.IsInf(x, 0) {
		if x > 0 {
			return "inf"
		}
		return "-inf"
	}
	s := strconv.FormatFloat(x, 'f', decimals, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func Signed(x float64, decimals int) string {
	s := strconv.FormatFloat(x, 'f', decimals, 64)
	if !strings.HasPrefix(s, "-") {
		s = "+" + s
	}
	return s
}

func Pct(x float64) string { return Signed(x, 1) + "%" }

func Frac(x float64, signed bool) string {
	if math.IsNaN(x) {
		return "n/d"
	}
	if signed {
		return Signed(x*100, 1) + "%"
	}
	return strconv.FormatFloat(x*100, 'f', 1, 64) + "%"
}

func G(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }
