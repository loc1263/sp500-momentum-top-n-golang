// Package visor es la tabla de rendimientos, riesgo y comparación contra SPY de las acciones grandes (mega y super large cap) y de los
// ETFs de mayor AUM de EE. UU.: rendimientos de 1 a 12 meses y de 1 a 5 años, Sharpe, beta, drawdown, distancia al máximo de 52
// semanas, precio contra la media de 200 ruedas y dividend yield.
//
// Es una sección aparte: arma su propio universo con el screener de Yahoo (gohoo) y escribe su propio JSON para el sitio. No usa
// los scores ni las tablas de los otros modelos.
//
// Fecha: 2026-11-19
package visor

import (
	"math"
	"slices"
	"time"

	"github.com/loc1263/gohoo"

	"top/internal/stats"
)

const (
	TradingDays = 252
	MonthDays   = 21
	maDays      = 200
)

var (
	Months   = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	Years    = []int{1, 2, 3, 5}
	RelYears = []int{1, 3}
)

type Series struct {
	Dates []string
	Close []float64
}

func SeriesOf(bars []gohoo.Bar) Series {
	var s Series
	for _, b := range bars {
		if math.IsNaN(b.AdjClose) || b.AdjClose <= 0 {
			continue
		}
		s.Dates = append(s.Dates, b.Time.Format(time.DateOnly))
		s.Close = append(s.Close, b.AdjClose)
	}
	return s
}

type Returns struct {
	Dates []string
	R     []float64
}

func ReturnsOf(s Series, last int) Returns {
	if len(s.Close) < 2 {
		return Returns{}
	}
	r := Returns{Dates: s.Dates[1:], R: stats.PctChange(s.Close)}
	if last > 0 && len(r.R) > last {
		r.Dates, r.R = r.Dates[len(r.Dates)-last:], r.R[len(r.R)-last:]
	}
	return r
}

func PeriodReturn(p []float64, n int) *float64 {
	if len(p) <= n {
		return nil
	}
	return ptr((p[len(p)-1]/p[len(p)-1-n] - 1) * 100)
}

func YTD(s Series) *float64 {
	if len(s.Close) == 0 {
		return nil
	}
	year := s.Dates[len(s.Dates)-1][:4]
	for i, d := range slices.Backward(s.Dates) {
		if d[:4] < year {
			return ptr((s.Close[len(s.Close)-1]/s.Close[i] - 1) * 100)
		}
	}
	return nil
}

func MaxDrawdown(p []float64) float64 {
	worst, peak := 0.0, math.Inf(-1)
	for _, x := range p {
		peak = math.Max(peak, x)
		worst = math.Min(worst, x/peak-1)
	}
	return worst * 100
}

type Metrics struct {
	YTD         *float64
	RetMonths   map[int]*float64
	RetYears    map[int]*float64
	Volatility  *float64
	Beta        *float64
	CAGR3       *float64
	MaxDrawdown *float64
	DistMax52   *float64
	VsMA200     *float64
	DivYield    *float64
}

func Compute(s Series, divs map[string]float64, spy Returns) (m Metrics, ok bool) {
	n := len(s.Close)
	if n < 2 {
		return m, false
	}
	p, last := s.Close, s.Close[n-1]
	m.YTD = YTD(s)
	m.DistMax52 = ptr((last/maxOf(tail(p, TradingDays)) - 1) * 100)
	if n >= maDays {
		m.VsMA200 = ptr((last/stats.Mean(tail(p, maDays)) - 1) * 100)
	}
	m.MaxDrawdown = ptr(MaxDrawdown(tail(p, TradingDays*5+1)))
	m.DivYield = ptr(dividendYield(s, divs))
	m.RetMonths, m.RetYears = map[int]*float64{}, map[int]*float64{}
	for _, k := range Months {
		m.RetMonths[k] = PeriodReturn(p, MonthDays*k)
	}
	for _, y := range Years {
		m.RetYears[y] = PeriodReturn(p, TradingDays*y)
	}
	if n > TradingDays*3 {
		r3 := ReturnsOf(s, TradingDays*3)
		m.Volatility = ptr(stats.Std(r3.R) * math.Sqrt(TradingDays) * 100)
		m.CAGR3 = ptr((math.Pow(last/p[n-TradingDays*3-1], 1.0/3) - 1) * 100)
		m.Beta = beta(r3, spy)
	}
	return m, true
}

func dividendYield(s Series, divs map[string]float64) float64 {
	end, err := time.Parse(time.DateOnly, s.Dates[len(s.Dates)-1])
	if err != nil {
		return 0
	}
	from := end.AddDate(0, 0, -365).Format(time.DateOnly)
	sum := 0.0
	for d, v := range divs {
		if d > from {
			sum += v
		}
	}
	return sum / s.Close[len(s.Close)-1] * 100
}

func beta(r, spy Returns) *float64 {
	bySpy := make(map[string]float64, len(spy.Dates))
	for i, d := range spy.Dates {
		bySpy[d] = spy.R[i]
	}
	var x, y []float64
	for i, d := range r.Dates {
		if v, ok := bySpy[d]; ok {
			x, y = append(x, r.R[i]), append(y, v)
		}
	}
	if len(x) <= TradingDays || !(stats.Std(spy.R) > 0) {
		return nil
	}
	mx, my := stats.Mean(x), stats.Mean(y)
	cov, vy := 0.0, 0.0
	for i := range x {
		cov += (x[i] - mx) * (y[i] - my)
		vy += (y[i] - my) * (y[i] - my)
	}
	if vy == 0 {
		return nil
	}
	return ptr(cov / vy)
}

func tail(p []float64, n int) []float64 {
	if len(p) <= n {
		return p
	}
	return p[len(p)-n:]
}

func maxOf(p []float64) float64 {
	m := math.Inf(-1)
	for _, x := range p {
		m = math.Max(m, x)
	}
	return m
}

func ptr(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
