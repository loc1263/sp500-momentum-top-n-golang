// Package etf calcula el score de momentum ajustado por riesgo de un ETF (lo usa «ETFs por gestora», package etfq).
//
// Para cada ETF y cada plazo se calcula el retorno del plazo dividido por la volatilidad del plazo (anualizada y llevada a la
// escala del plazo): cuánto subió por cada unidad de riesgo que corrió. El score es el promedio de esos cocientes.
//
// Fecha: 2026-11-19
package etf

import (
	"fmt"
	"math"
	"sort"

	"topwx/internal/model"
	"topwx/internal/stats"
)

const (
	TradingDaysYear = 252
	SMADays         = 200
)

type Score struct {
	Ticker  string
	Rank    int
	Score   float64
	Returns map[int]float64
	Vol     float64
	MaxDD   float64
	SMADist float64
}

type Scores struct {
	Rows     []Score
	RiskFree string
	CashLike []string
}

func ScoreEtfs(prices *model.Prices, horizons []int, tradingDaysPerMonth int, riskFree string, minVol float64) (Scores, map[string]string) {
	hs := append([]int(nil), horizons...)
	sort.Ints(hs)
	longest := hs[len(hs)-1] * tradingDaysPerMonth
	window := prices.Tail(longest + 1)
	excluded := map[string]string{}
	if window.Rows() < longest+1 {
		for _, t := range prices.Tickers {
			excluded[t] = fmt.Sprintf("solo hay %d ruedas, se necesitan %d", window.Rows(), longest+1)
		}
		return Scores{}, excluded
	}
	var keep []string
	for _, t := range window.Tickers {
		j, _ := window.Col(t)
		gap := false
		for i := 0; i < window.Rows(); i++ {
			if math.IsNaN(window.At(i, j)) {
				gap = true
				break
			}
		}
		if gap {
			excluded[t] = "huecos o historia menor al plazo más largo"
		} else {
			keep = append(keep, t)
		}
	}
	window = window.Select(keep)
	if !window.Has(riskFree) {
		riskFree = ""
	}

	type row struct {
		ticker           string
		rets             map[int]float64
		ratios           []float64
		vol, maxDD, smaD float64
	}
	n := window.Rows()
	rows := make([]row, window.Cols())
	for j, t := range window.Tickers {
		series := window.Series(t)
		rows[j] = row{ticker: t, rets: map[int]float64{}}
		rows[j].vol = stats.Std(stats.PctChange(series)) * math.Sqrt(TradingDaysYear)
		peak, dd := series[0], 0.0
		for _, p := range series {
			peak = math.Max(peak, p)
			dd = math.Min(dd, p/peak-1)
		}
		rows[j].maxDD = dd
		tail := series[max(n-SMADays, 0):]
		rows[j].smaD = series[n-1]/stats.Mean(tail) - 1
	}
	for _, h := range hs {
		w := window.Tail(h*tradingDaysPerMonth + 1)
		rets := make([]float64, window.Cols())
		for j, t := range window.Tickers {
			s := w.Series(t)
			rets[j] = s[len(s)-1]/s[0] - 1
			rows[j].rets[h] = rets[j]
		}
		rf := 0.0
		if riskFree != "" {
			j, _ := window.Col(riskFree)
			rf = rets[j]
		}
		for j, t := range window.Tickers {
			vol := stats.Std(stats.PctChange(w.Series(t))) * math.Sqrt(TradingDaysYear)
			scaled := vol * math.Sqrt(float64(h)/12)
			ratio := math.NaN()
			if scaled > 0 {
				ratio = (rets[j] - rf) / scaled
			}
			rows[j].ratios = append(rows[j].ratios, ratio)
		}
	}

	var ranked []Score
	var cashLike []string
	for _, r := range rows {
		score := stats.Mean(r.ratios)
		switch {
		case math.IsNaN(score):
			excluded[r.ticker] = "volatilidad nula"
		case r.vol < minVol:
			cashLike = append(cashLike, r.ticker)
		default:
			ranked = append(ranked, Score{Ticker: r.ticker, Score: score, Returns: r.rets, Vol: r.vol, MaxDD: r.maxDD, SMADist: r.smaD})
		}
	}
	sort.SliceStable(ranked, func(a, b int) bool { return ranked[a].Score > ranked[b].Score })
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	sort.Strings(cashLike)
	return Scores{Rows: ranked, RiskFree: riskFree, CashLike: cashLike}, excluded
}
