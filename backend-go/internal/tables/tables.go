// Package tables arma las tablas de resultados como (encabezados, filas) de texto; las usa la consola.
package tables

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"top3/internal/dateutil"
	"top3/internal/model"
	"top3/internal/numfmt"
)

// Table son los encabezados y las filas de una tabla de texto.
type Table struct {
	Headers []string
	Rows    [][]string
}

// HistoryTable es una fila por rebalanceo: el Top N, el capital al momento de entrar y su % vs. el capital inicial.
func HistoryTable(history []model.Rebalance, equity []model.EquityPoint, initialCapital float64) Table {
	capByDate := make(map[string]float64, len(equity))
	for _, e := range equity {
		capByDate[dateutil.Day(e.Date)] = e.Capital
	}
	n := 0
	for _, h := range history {
		n = max(n, len(h.Picks))
	}
	headers := []string{"Fecha"}
	for i := 1; i <= n; i++ {
		headers = append(headers, "#"+strconv.Itoa(i))
	}
	headers = append(headers, "Cap. aprox. (US$ B)", "Capital (US$)", "Gan/Pérd")

	capsCol := func(h model.Rebalance) string {
		parts := make([]string, len(h.Picks))
		for i, p := range h.Picks {
			if c, ok := h.Caps[p.Ticker]; ok {
				parts[i] = numfmt.Comma(c/1e9, 0)
			} else {
				parts[i] = "n/d"
			}
		}
		return strings.Join(parts, " / ")
	}
	capitalCols := func(capital float64) []string {
		return []string{numfmt.Comma(capital, 2), numfmt.Pct((capital/initialCapital - 1) * 100)}
	}

	var rows [][]string
	seen := map[string]bool{}
	for _, h := range history {
		seen[h.Date] = true
		row := []string{h.Date}
		for _, p := range h.Picks {
			row = append(row, p.Ticker+" "+numfmt.Pct(p.Return*100))
		}
		for len(row) < 1+n {
			row = append(row, "")
		}
		row = append(row, capsCol(h))
		row = append(row, capitalCols(capByDate[h.Date])...)
		rows = append(rows, row)
	}
	// Fila final: valor al cierre del último tramo
	if len(equity) > 0 {
		last := equity[len(equity)-1]
		if day := dateutil.Day(last.Date); !seen[day] {
			row := []string{day}
			row = append(row, make([]string, n+1)...)
			rows = append(rows, append(row, capitalCols(last.Capital)...))
		}
	}
	return Table{headers, rows}
}

// CalendarTable es una fila por calendario de arranque: primer rebalanceo, rebalanceos y resultado.
func CalendarTable(calendars []model.Calendar) Table {
	t := Table{Headers: []string{"Primer rebalanceo", "Rebalanceos", "Capital final (US$)", "Retorno total", "CAGR"}}
	for _, c := range calendars {
		t.Rows = append(t.Rows, []string{c.History[0].Date, strconv.Itoa(len(c.History)), numfmt.Comma(c.FinalCapital, 2),
			numfmt.Pct(c.TotalReturnPct), numfmt.Pct(c.CAGRPct)})
	}
	return t
}

// byMedianCAGR ordena los plazos de mejor a peor CAGR mediano (estable).
func byMedianCAGR(terms []model.Term) []model.Term {
	out := append([]model.Term(nil), terms...)
	med := func(t model.Term) float64 { m, _, _ := t.CagrStats(); return m }
	sort.SliceStable(out, func(a, b int) bool { return med(out[a]) > med(out[b]) })
	return out
}

func withBase(terms []model.Term) []model.Term {
	var out []model.Term
	for _, t := range terms {
		if t.Base() != nil {
			out = append(out, t)
		}
	}
	return out
}

func withCalendars(terms []model.Term) []model.Term {
	var out []model.Term
	for _, t := range terms {
		if len(t.Calendars) > 0 {
			out = append(out, t)
		}
	}
	return out
}

// SweepTable compara los plazos, de mejor a peor CAGR mediano; los plazos sin resultado van al final.
func SweepTable(terms []model.Term) Table {
	t := Table{Headers: []string{"Plazo", "Rebalanceos", "Capital final (US$)", "Retorno total", "CAGR",
		"Calendarios", "CAGR mediana", "CAGR mín", "CAGR máx"}}
	for _, term := range byMedianCAGR(withBase(terms)) {
		median, low, high := term.CagrStats()
		base := term.Base()
		t.Rows = append(t.Rows, []string{fmt.Sprintf("%d meses", term.Months), strconv.Itoa(len(base.History)),
			numfmt.Comma(base.FinalCapital, 2), numfmt.Pct(base.TotalReturnPct), numfmt.Pct(base.CAGRPct),
			strconv.Itoa(len(term.Calendars)), numfmt.Pct(median), numfmt.Pct(low), numfmt.Pct(high)})
	}
	for _, term := range terms {
		if term.Base() == nil {
			t.Rows = append(t.Rows, []string{fmt.Sprintf("%d meses", term.Months), "0", "sin datos", "-", "-", "-", "-", "-", "-"})
		}
	}
	return t
}

// SweepBest es la línea del mejor plazo ("" si ninguno tiene resultado).
func SweepBest(terms []model.Term, initialCapital float64) string {
	ranked := byMedianCAGR(withBase(terms))
	if len(ranked) == 0 {
		return ""
	}
	t := ranked[0]
	median, low, high := t.CagrStats()
	return fmt.Sprintf("Mejor plazo: %d meses (CAGR mediano %s, rango %s a %s; calendario base US$ %s → US$ %s, %s)",
		t.Months, numfmt.Pct(median), numfmt.Pct(low), numfmt.Pct(high), numfmt.Comma(initialCapital, 0),
		numfmt.Comma(t.Base().FinalCapital, 2), numfmt.Pct(t.Base().TotalReturnPct))
}

// OffsetsTable es la sensibilidad al arranque: mediana y rango del CAGR y del capital final de cada plazo.
func OffsetsTable(terms []model.Term) Table {
	t := Table{Headers: []string{"Plazo", "Calendarios", "CAGR mediana", "CAGR mín", "CAGR máx",
		"Capital mediana", "Capital mín", "Capital máx"}}
	for _, term := range byMedianCAGR(withCalendars(terms)) {
		cm, cl, ch := term.CagrStats()
		km, kl, kh := term.CapitalStats()
		t.Rows = append(t.Rows, []string{fmt.Sprintf("%d meses", term.Months), strconv.Itoa(len(term.Calendars)),
			numfmt.Pct(cm), numfmt.Pct(cl), numfmt.Pct(ch),
			numfmt.Comma(km, 0), numfmt.Comma(kl, 0), numfmt.Comma(kh, 0)})
	}
	return t
}

// OffsetsBest es la línea del mejor plazo por CAGR mediano ("" si ninguno tiene calendarios).
func OffsetsBest(terms []model.Term) string {
	ranked := byMedianCAGR(withCalendars(terms))
	if len(ranked) == 0 {
		return ""
	}
	t := ranked[0]
	median, low, high := t.CagrStats()
	return fmt.Sprintf("Mejor plazo por CAGR mediano: %d meses (mediana %s, rango %s a %s, %d calendarios)",
		t.Months, numfmt.Pct(median), numfmt.Pct(low), numfmt.Pct(high), len(t.Calendars))
}

// ModelsTable es el modelo de siempre (N-0) frente al skip-month (N-1), plazo por plazo: CAGR mediano y capital mediano.
func ModelsTable(terms, skipTerms []model.Term) Table {
	t := Table{Headers: []string{"Plazo", "CAGR N-0", "CAGR N-1", "Dif. CAGR", "Capital N-0", "Capital N-1", "Gana"}}
	skipByMonths := map[int]model.Term{}
	for _, s := range skipTerms {
		if len(s.Calendars) > 0 {
			skipByMonths[s.Months] = s
		}
	}
	for _, term := range byMedianCAGR(withCalendars(terms)) {
		median, _, _ := term.CagrStats()
		capital, _, _ := term.CapitalStats()
		label := fmt.Sprintf("%d meses", term.Months)
		other, ok := skipByMonths[term.Months]
		if !ok {
			t.Rows = append(t.Rows, []string{label, numfmt.Pct(median), "sin datos", "-", numfmt.Comma(capital, 0), "-", "-"})
			continue
		}
		otherMedian, _, _ := other.CagrStats()
		otherCapital, _, _ := other.CapitalStats()
		winner := "empate"
		switch {
		case otherMedian > median:
			winner = "N-1"
		case median > otherMedian:
			winner = "N-0"
		}
		t.Rows = append(t.Rows, []string{label, numfmt.Pct(median), numfmt.Pct(otherMedian),
			numfmt.Signed(otherMedian-median, 1) + " pp", numfmt.Comma(capital, 0), numfmt.Comma(otherCapital, 0), winner})
	}
	return t
}
