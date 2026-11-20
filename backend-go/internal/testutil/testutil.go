// Package testutil tiene los datos de prueba compartidos por los tests de varios paquetes.
//
// Fecha: 2026-11-19
package testutil

import (
	"fmt"
	"math"
	"time"

	"top3/internal/dateutil"
	"top3/internal/model"
)

func BDays(start, end string) []time.Time {
	var out []time.Time
	for d := dateutil.MustParse(start); !d.After(dateutil.MustParse(end)); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

func BDaysN(start string, n int) []time.Time {
	out := make([]time.Time, 0, n)
	for d := dateutil.MustParse(start); len(out) < n; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

func MakePrices(start, end string, n int) *model.Prices {
	dates := BDays(start, end)
	tickers := make([]string, n)
	for i := range tickers {
		tickers[i] = fmt.Sprintf("T%02d", i)
	}
	p := model.NewPrices(dates, tickers)
	for i := range tickers {
		for k := range dates {
			p.Px[k*n+i] = 100 * math.Pow(1+0.0005*float64(i+1), float64(k))
		}
	}
	return p
}

func DefaultPrices() *model.Prices { return MakePrices("2023-01-02", "2024-12-31", 12) }

type Members struct{ Tickers []string }

func (m Members) OnDate(string) []string { return m.Tickers }
