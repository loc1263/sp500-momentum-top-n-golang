// Tipos de resultados del backtest (selecciones, rebalanceos, curva de capital, calendarios, plazos) y
// sus estadísticas.
//
// Fecha: 2026-11-19

package model

import (
	"sort"
	"time"
)

type Pick struct {
	Ticker string
	Return float64
}

type Rebalance struct {
	Date  string
	Picks []Pick
	Caps  map[string]float64
}

type EquityPoint struct {
	Date    time.Time
	Capital float64
}

type Result struct {
	Equity         []EquityPoint
	History        []Rebalance
	FinalCapital   float64
	TotalReturnPct float64
	CAGRPct        float64
}

type Calendar struct {
	Result
	Offset int
}

type Term struct {
	Months    int
	Calendars []Calendar
}

func (t Term) Base() *Calendar {
	for i := range t.Calendars {
		if t.Calendars[i].Offset == 0 {
			return &t.Calendars[i]
		}
	}
	return nil
}

func Median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func stats(xs []float64) (median, low, high float64) {
	low, high = xs[0], xs[0]
	for _, x := range xs {
		low, high = min(low, x), max(high, x)
	}
	return Median(xs), low, high
}

func (t Term) CagrStats() (median, low, high float64) {
	xs := make([]float64, len(t.Calendars))
	for i, c := range t.Calendars {
		xs[i] = c.CAGRPct
	}
	return stats(xs)
}

func (t Term) CapitalStats() (median, low, high float64) {
	xs := make([]float64, len(t.Calendars))
	for i, c := range t.Calendars {
		xs[i] = c.FinalCapital
	}
	return stats(xs)
}
