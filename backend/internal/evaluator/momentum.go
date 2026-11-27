// Indicadores de tendencia del precio que se muestran en Valoración: precio frente a su media de 200 ruedas, rendimiento de 3 y 6 meses
// (contra cero y contra el S&P 500) y distancia al máximo de 52 semanas. Salen del historial diario de un año que baja Yahoo.
//
// Fecha: 2026-11-20

package evaluator

import (
	"context"
	"database/sql"
	"math"
	"strings"

	"github.com/loc1263/gohoo"

	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/logx"
	"topwx/internal/market"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
)

const (
	benchmarkTicker = "^GSPC"
	smaDays         = 200
	// Distancia al máximo de 52 semanas: hasta nearHigh % es «Bien»; hasta farHigh %, «Atención».
	nearHigh = 10.0
	farHigh  = 15.0
)

// Benchmark es el rendimiento del índice de referencia (S&P 500) en los mismos plazos, en %. NaN si no se pudo bajar.
type Benchmark struct{ Ret3M, Ret6M float64 }

func fetchBars(ctx context.Context, y *yahoo.Client, ticker string) []gohoo.Bar {
	res, err := y.ChartRange(ctx, ticker, gohoo.ChartParams{Range: "1y"})
	if err != nil || res == nil {
		return nil
	}
	return res.Bars
}

// storedBars lee de SQLite el último año de cierres que el modelo por score guardó hoy junto con los precios, para no volver a
// pedirlos a Yahoo. Si el OHLCV no es de hoy devuelve vacío y cada empresa baja su historial.
func storedBars(d *sql.DB) map[string][]gohoo.Bar {
	if !db.FetchedToday(d, market.OHLCVFetchedKey) {
		return nil
	}
	closes, err := repo.OHLCVCloses(d, dateutil.Day(dateutil.Today().AddDate(-1, 0, 0)))
	if err != nil {
		logx.Debugf("Sin cierres guardados para el evaluador: %v", err)
		return nil
	}
	out := make(map[string][]gohoo.Bar, len(closes))
	for t, cs := range closes {
		bars := make([]gohoo.Bar, 0, len(cs))
		for _, c := range cs {
			day, err := dateutil.Parse(c.Date)
			if err != nil {
				continue
			}
			bars = append(bars, gohoo.Bar{Time: day, Close: c.Close})
		}
		out[t] = bars
	}
	return out
}

// FetchBenchmark baja el historial del S&P 500 una sola vez para toda la corrida.
func FetchBenchmark(ctx context.Context, y *yahoo.Client) Benchmark {
	b := Benchmark{nan, nan}
	if bars := fetchBars(ctx, y, benchmarkTicker); len(bars) > 0 {
		b.Ret3M, b.Ret6M = returnsOf(bars)
	}
	return b
}

// returnsOf calcula el rendimiento de precio de 3 y 6 meses hasta la última barra.
func returnsOf(bars []gohoo.Bar) (ret3, ret6 float64) {
	return returnSince(bars, 3), returnSince(bars, 6)
}

// returnSince es el cambio porcentual del cierre desde la última rueda a o antes de la fecha de hace months meses.
// NaN si el historial no llega tan atrás.
func returnSince(bars []gohoo.Bar, months int) float64 {
	if len(bars) == 0 {
		return nan
	}
	last := bars[len(bars)-1]
	target := last.Time.AddDate(0, -months, 0)
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].Time.After(target) {
			continue
		}
		if base := bars[i].Close; base > 0 && !isNaN(last.Close) {
			return (last.Close/base - 1) * 100
		}
		return nan
	}
	return nan
}

// sma es el promedio simple de los últimos n cierres; NaN si hay menos de n o falta alguno.
func sma(bars []gohoo.Bar, n int) float64 {
	if len(bars) < n {
		return nan
	}
	sum := 0.0
	for _, b := range bars[len(bars)-n:] {
		if isNaN(b.Close) {
			return nan
		}
		sum += b.Close
	}
	return sum / float64(n)
}

// applyBars completa los datos de tendencia de la empresa con su historial y el rendimiento del índice.
func (r *Raw) applyBars(bars []gohoo.Bar, bench Benchmark) {
	r.IdxRet3M, r.IdxRet6M = bench.Ret3M, bench.Ret6M
	r.Ret3M, r.Ret6M = returnsOf(bars)
	r.SMA200 = sma(bars, smaDays)
}

func comma(v float64, dec int) string { return strings.Replace(formatFloat(v, dec), ".", ",", 1) }

// returnLevel clasifica un rendimiento: «Bien» si es positivo y no queda bajo el índice; «Atención» si solo cumple una de las dos
// cosas (sube menos que el índice, o cae menos que él); «Alerta» si cae y además rinde menos que el índice. Sin índice, solo cuenta el signo.
func returnLevel(ret, idx float64) (Level, string) {
	switch {
	case isNaN(ret):
		return NA, ""
	case isNaN(idx) && ret > 0:
		return Good, "Positivo"
	case isNaN(idx):
		return Bad, "Negativo"
	case ret > 0 && ret >= idx:
		return Good, "Positivo"
	case ret > 0:
		return Warn, "Bajo el índice"
	case ret >= idx:
		return Warn, "Mejor que el índice"
	}
	return Bad, "Negativo"
}

func returnMetric(id string, ret, idx float64) Metric {
	mt := m(id, ret, NA)
	mt.Level, mt.Label = returnLevel(ret, idx)
	if !isNaN(idx) && !isNaN(ret) {
		mt.Note = "S&P 500 en el mismo plazo: " + signed(idx, 1) + " %."
	}
	return mt
}

// trendMetrics son los indicadores de tendencia del precio de Valoración.
func trendMetrics(r Raw) []Metric {
	vsSMA := nan
	if r.Price > 0 && r.SMA200 > 0 {
		vsSMA = (r.Price/r.SMA200 - 1) * 100
	}
	smaM := m("sma200", vsSMA, NA)
	switch {
	case isNaN(vsSMA):
	case vsSMA > 0:
		smaM.Level, smaM.Label = Good, "Sobre la SMA 200"
	default:
		smaM.Level, smaM.Label = Bad, "Bajo la SMA 200"
	}
	if !isNaN(vsSMA) {
		smaM.Note = "Media simple de los últimos 200 días: " + comma(r.SMA200, 2) + " " + r.Currency + "."
	}

	off := nan
	if r.W52High > 0 && !isNaN(r.Price) {
		off = math.Max(0, (r.W52High-r.Price)/r.W52High*100)
	}
	offM := m("off_high", off, ifOK(off, func() Level { return lo(off, nearHigh, farHigh) }))
	switch {
	case isNaN(off):
	case off <= nearHigh:
		offM.Label = "Cerca del máximo"
	case off <= farHigh:
		offM.Label = "A media distancia"
	default:
		offM.Label = "Lejos del máximo"
	}
	return []Metric{smaM, returnMetric("ret_3m", r.Ret3M, r.IdxRet3M), returnMetric("ret_6m", r.Ret6M, r.IdxRet6M), offM}
}
