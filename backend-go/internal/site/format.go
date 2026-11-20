// Formato numérico es-AR de las páginas (equivalente a frontend/src/lib/format.js): coma decimal, punto de miles desde 4 dígitos,
// porcentajes con signo y montos abreviados (M / B).
//
// Fecha: 2026-11-19

package site

import (
	"math"
	"strconv"
	"strings"
)

const nd = "n/d"

// esNum formatea con exactamente digits decimales: 1234.567 -> «1.234,57».
func esNum(v float64, digits int) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', digits, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if v < 0 && strings.Trim(s, "0.") != "" {
		out = "-" + out
	}
	return out
}

func derefOr(p *float64, ok func(float64) string) string {
	if p == nil {
		return nd
	}
	return ok(*p)
}

// pctFrac muestra una fracción como porcentaje con signo explícito (0,123 -> «+12,3%»).
func pctFrac(p *float64, digits int) string {
	return derefOr(p, func(v float64) string {
		sign := "+"
		if v < 0 {
			sign = "-"
		}
		return sign + esNum(math.Abs(v*100), digits) + "%"
	})
}

// share muestra una fracción como porcentaje sin signo (0,25 -> «25,0%»).
func share(v float64, digits int) string { return esNum(v*100, digits) + "%" }

func dec(p *float64, digits int) string {
	return derefOr(p, func(v float64) string { return esNum(v, digits) })
}

func usd(p *float64) string { return dec(p, 2) }

func billions(p *float64) string {
	return derefOr(p, func(v float64) string { return esNum(v/1e9, 0) })
}

// aum abrevia un patrimonio: 12,3 B (miles de millones) o 450 M.
func aum(p *float64) string {
	return derefOr(p, func(v float64) string {
		if v >= 1e9 {
			return esNum(v/1e9, 1) + " B"
		}
		return esNum(v/1e6, 0) + " M"
	})
}

func signClass(p *float64) string {
	switch {
	case p == nil:
		return ""
	case *p > 0:
		return "pos"
	case *p < 0:
		return "neg"
	}
	return ""
}

var accents = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n{")

// collateEs compara textos como lo hace localeCompare('es') en lo básico: sin distinguir mayúsculas ni acentos, con la ñ después de la n.
func collateEs(a, b string) int {
	return strings.Compare(accents.Replace(strings.ToLower(a)), accents.Replace(strings.ToLower(b)))
}
