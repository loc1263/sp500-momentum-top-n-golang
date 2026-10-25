// Package numfmt formatea números como lo hacen las tablas y los textos del proyecto (miles con coma, signo explícito).
package numfmt

import (
	"math"
	"strconv"
	"strings"
)

// Comma es x con `decimals` decimales y comas de miles ("1,234.50"),.
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

// Signed es x con `decimals` decimales y siempre con signo ("+1.5", "-2.0"), como f"{x:+.1f}".
func Signed(x float64, decimals int) string {
	s := strconv.FormatFloat(x, 'f', decimals, 64)
	if !strings.HasPrefix(s, "-") {
		s = "+" + s
	}
	return s
}

// Pct es x (ya en porcentaje) con un decimal, signo y "%": "+12.3%".
func Pct(x float64) string { return Signed(x, 1) + "%" }

// Frac es la fracción x en porcentaje con signo ("0.123" -> "+12.3%"); "n/d" si es NaN.
func Frac(x float64, signed bool) string {
	if math.IsNaN(x) {
		return "n/d"
	}
	if signed {
		return Signed(x*100, 1) + "%"
	}
	return strconv.FormatFloat(x*100, 'f', 1, 64) + "%"
}

// G es x como %g (sin ceros de más: 4 -> "4", 0.5 -> "0.5").
func G(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }
