package quant

import (
	"top3/internal/model"
	"top3/internal/stats"
)

// Score de momentum de las acciones del S&P 500: promedio del ranking en el retorno de 12 y de 6 meses, sin contar el último
// mes (momentum 12-1 y 6-1). Solo usa precios hasta la fecha.

// Month son las ruedas de un mes.
const Month = 21

// MomentumReturns son los momentum 12-1 y 6-1 de cada ticker de prices en su última rueda (alineados con prices.Tickers).
// Un ticker con huecos en el último año o sin un año de historia queda en NaN, y todos si hay menos de un año de ruedas.
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
		skip := window.At(rows-1-Month, j) // precio de hace un mes: el último mes no cuenta
		mom12[j] = skip/window.At(0, j) - 1
		mom6[j] = skip/window.At(rows-127, j) - 1
	}
	return mom12, mom6
}

// MomentumScore es el score 0-100: la posición de cada acción (100 = la mejor) según el promedio de su ranking porcentual en
// el momentum 12-1 y 6-1 entre todas las que tienen ambos. Las demás quedan en NaN.
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
