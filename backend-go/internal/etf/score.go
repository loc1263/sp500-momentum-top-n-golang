// Package etf calcula el score de momentum ajustado por riesgo de un ETF (lo usa «ETFs por gestora», package etfq).
//
// Para cada ETF y cada plazo se calcula el retorno del plazo dividido por la volatilidad del plazo (anualizada y llevada a la
// escala del plazo): cuánto subió por cada unidad de riesgo que corrió. El score es el promedio de esos cocientes.
package etf

import (
	"fmt"
	"math"
	"sort"

	"top3/internal/model"
	"top3/internal/stats"
)

const (
	TradingDaysYear = 252
	SMADays         = 200 // la tendencia se mide contra la media simple de 200 ruedas
)

// Score es la fila del ranking de un ETF.
type Score struct {
	Ticker  string
	Rank    int
	Score   float64
	Returns map[int]float64 // retorno de cada plazo, por meses
	Vol     float64         // volatilidad anualizada del plazo más largo
	MaxDD   float64         // caída máxima del plazo más largo
	SMADist float64         // distancia a la media de 200 ruedas
}

// Scores es el ranking de mayor a menor score.
type Scores struct {
	Rows []Score
	// RiskFree es la referencia libre de riesgo realmente usada ("" si no estaba disponible).
	RiskFree string
	// CashLike son los ETFs de tipo efectivo que quedaron fuera por su baja volatilidad.
	CashLike []string
}

// ScoreEtfs arma el ranking por retorno ajustado por riesgo. prices son los precios ajustados (fecha x ticker).
//
// Con riskFree (un ticker de la propia tabla, p. ej. un ETF de bonos ultracortos) el retorno de cada plazo es el exceso sobre
// el de esa referencia: sin eso, un ETF casi sin riesgo siempre ganaría. Si la referencia no está disponible se calcula sin
// ella y Scores.RiskFree queda vacío.
//
// Con minVol (volatilidad anual) los ETFs de tipo efectivo (letras del Tesoro, bonos ultracortos) quedan fuera del ranking:
// con tan poco riesgo cualquier exceso diminuto sobre la referencia da un score enorme. Se listan en Scores.CashLike (la
// referencia libre de riesgo, aunque no rankee, sigue restándose).
//
// Devuelve también {ticker: motivo} de los excluidos: los ETFs con huecos de precios en la ventana o con menos historia que
// el plazo más largo, y los de volatilidad nula.
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
			scaled := vol * math.Sqrt(float64(h)/12) // la volatilidad anual llevada a la escala del plazo
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
		score := stats.Mean(r.ratios) // NaN si algún plazo no tiene volatilidad
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
