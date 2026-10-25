// Package testutil tiene los datos de prueba compartidos por los tests de varios paquetes.
package testutil

import (
	"fmt"
	"math"
	"time"

	"top3/internal/dateutil"
	"top3/internal/model"
)

// BDays son los días hábiles (lunes a viernes) de start a end inclusive, como pd.bdate_range.
func BDays(start, end string) []time.Time {
	var out []time.Time
	for d := dateutil.MustParse(start); !d.After(dateutil.MustParse(end)); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

// BDaysN son n días hábiles desde start.
func BDaysN(start string, n int) []time.Time {
	out := make([]time.Time, 0, n)
	for d := dateutil.MustParse(start); len(out) < n; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

// MakePrices son n tickers T00..T{n-1} que crecen a ritmos distintos: T{n-1} siempre es el que más sube.
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

// DefaultPrices es MakePrices("2023-01-02", "2024-12-31", 12).
func DefaultPrices() *model.Prices { return MakePrices("2023-01-02", "2024-12-31", 12) }

// Members son los mismos constituyentes en todas las fechas.
type Members struct{ Tickers []string }

// OnDate devuelve siempre los mismos tickers.
func (m Members) OnDate(string) []string { return m.Tickers }
