// Formato de las celdas del Visor: etiquetas de columna, texto con decimales, y clase de color (verde / rojo).
// Va en Go (no en JS) para que el HTML salga ya armado; visor.js usa los valores crudos solo para filtrar y ordenar.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"topwx/internal/visor"
)

var textCols = map[string]bool{"ticker": true, "nombre": true, "sector": true, "categoria": true, "vs_spy": true}

var labels = map[string]string{
	"ticker": "Ticker", "nombre": "Nombre", "sector": "Sector", "categoria": "Categoría", "market_cap_B": "Cap. (B)", "aum_B": "AUM (B)",
	"expense_ratio": "TER %", "sharpe": "Sharpe", "beta": "Beta", "max_drawdown": "Máx. caída 5a", "dist_max52": "Dist. máx 52s",
	"vs_ma200": "vs MA200", "rel_spy_1y": "vs SPY 1a", "rel_spy_3y": "vs SPY 3a", "dividend_yield": "Dividendo", "ytd": "YTD", "ytd_spy": "YTD SPY",
	"ytd_vs_spy": "YTD vs SPY", "vs_spy": "vs SPY", "verdes": "Verdes",
}

var retCol = regexp.MustCompile(`^ret_(\d+)([my])$`)

func label(col string) string {
	if m := retCol.FindStringSubmatch(col); m != nil {
		if m[2] == "y" {
			return m[1] + "a"
		}
		return m[1] + "m"
	}
	if l, ok := labels[col]; ok {
		return l
	}
	return col
}

func isText(col string) bool { return textCols[col] }

func num(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func format(sec visor.Section, col string, v any) string {
	if v == nil {
		return ""
	}
	n, isNum := num(v)
	if !isNum {
		return fmt.Sprint(v)
	}
	switch sec.Fmt[col] {
	case "pct":
		return strconv.FormatFloat(n, 'f', 1, 64) + "%"
	case "dec":
		return strconv.FormatFloat(n, 'f', 2, 64)
	case "int":
		return groupEs(n)
	}
	return fmt.Sprint(v)
}

// groupEs formatea un entero como el locale «es»: separador de miles «.» solo desde 5 dígitos (5122, 12.345).
func groupEs(n float64) string {
	r := math.Round(n)
	s := strconv.FormatFloat(math.Abs(r), 'f', 0, 64)
	if len(s) >= 5 {
		var b strings.Builder
		for i, c := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteByte('.')
			}
			b.WriteRune(c)
		}
		s = b.String()
	}
	if r < 0 {
		return "-" + s
	}
	return s
}

// cellClass devuelve «pos», «neg» o "" según las reglas de la sección (signo, umbral o SUPERA / NO SUPERA).
func cellClass(sec visor.Section, col string, v any) string {
	if v == nil {
		return ""
	}
	n, isNum := num(v)
	if slices.Contains(sec.Sign, col) {
		switch {
		case isNum && n > 0:
			return "pos"
		case isNum && n < 0:
			return "neg"
		}
		return ""
	}
	if thr, ok := sec.Thr[col]; ok {
		op, _ := thr[0].(string)
		limit, _ := num(thr[1])
		if isNum && compare(op, n, limit) {
			return "pos"
		}
		return ""
	}
	if col == "vs_spy" {
		switch v {
		case "SUPERA":
			return "pos"
		case "NO SUPERA":
			return "neg"
		}
	}
	return ""
}

func compare(op string, a, b float64) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	}
	return false
}
