// Cálculo de retornos y score de momentum por mes.
//
// Fecha: 2026-11-19

package quant

import (
	"top3/internal/model"
	"top3/internal/stats"
)

const Month = 21

func MomentumReturns(prices *model.Prices) (mom12, mom6 []float64) {
	n := prices.Cols()
	mom12, mom6 = make([]float64, n), make([]float64, n)
	for i := range mom12 {
		mom12[i], mom6[i] = nan, nan
	}
	window := prices.Tail(DayYear + 1)
	if window.Rows() < DayYear+1 {
		return mom12, mom6
	}
	rows := window.Rows()
	for j := 0; j < n; j++ {
		complete := true
		for i := 0; i < rows; i++ {
			if isNaN(window.At(i, j)) {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		skip := window.At(rows-1-Month, j)
		mom12[j] = skip/window.At(0, j) - 1
		mom6[j] = skip/window.At(rows-127, j) - 1
	}
	return mom12, mom6
}

func MomentumScore(mom12, mom6 []float64) []float64 {
	var idx []int
	for i := range mom12 {
		if !isNaN(mom12[i]) && !isNaN(mom6[i]) {
			idx = append(idx, i)
		}
	}
	a, b := make([]float64, len(idx)), make([]float64, len(idx))
	for k, i := range idx {
		a[k], b[k] = mom12[i], mom6[i]
	}
	ra, rb := stats.RankPct(a, true), stats.RankPct(b, true)
	avg := make([]float64, len(idx))
	for k := range avg {
		avg[k] = (ra[k] + rb[k]) / 2
	}
	out := make([]float64, len(mom12))
	for i := range out {
		out[i] = nan
	}
	for k, r := range stats.RankPct(avg, true) {
		out[idx[k]] = r * 100
	}
	return out
}
