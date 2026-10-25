package model

import (
	"sort"
	"time"
)

// Pick es una acción elegida en un rebalanceo con su retorno del ranking.
type Pick struct {
	Ticker string
	Return float64
}

// Rebalance es la selección de un día de rebalanceo. Caps (capitalización aprox. por ticker) se completa después.
type Rebalance struct {
	Date  string
	Picks []Pick
	Caps  map[string]float64
}

// EquityPoint es el capital en una fecha.
type EquityPoint struct {
	Date    time.Time
	Capital float64
}

// Result es el resultado de una corrida. Los retornos van en porcentaje (retorno total y CAGR).
type Result struct {
	Equity         []EquityPoint
	History        []Rebalance
	FinalCapital   float64
	TotalReturnPct float64
	CAGRPct        float64
}

// Calendar es una corrida de un plazo con el arranque desfasado Offset meses (0 = calendario base).
type Calendar struct {
	Result
	Offset int
}

// Term es un plazo de N meses con todos sus calendarios de arranque posibles.
type Term struct {
	Months    int
	Calendars []Calendar
}

// Base es el calendario base: el que arranca en el primer mes del período (nil si no hubo).
func (t Term) Base() *Calendar {
	for i := range t.Calendars {
		if t.Calendars[i].Offset == 0 {
			return &t.Calendars[i]
		}
	}
	return nil
}

// Median es la mediana de xs (el promedio de las dos centrales si hay cantidad par).
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

// CagrStats son (mediana, mínimo, máximo) del CAGR % entre los calendarios.
func (t Term) CagrStats() (median, low, high float64) {
	xs := make([]float64, len(t.Calendars))
	for i, c := range t.Calendars {
		xs[i] = c.CAGRPct
	}
	return stats(xs)
}

// CapitalStats son (mediana, mínimo, máximo) del capital final entre los calendarios.
func (t Term) CapitalStats() (median, low, high float64) {
	xs := make([]float64, len(t.Calendars))
	for i, c := range t.Calendars {
		xs[i] = c.FinalCapital
	}
	return stats(xs)
}
