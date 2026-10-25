// Package engine es el motor del backtest: cada N meses se compran, con ponderación igual, las N acciones que más subieron.
package engine

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"top3/internal/model"
)

// Matrix son los precios (fecha x ticker) con lo necesario para consultar ventanas sin copiar historia en cada llamada.
//
// El backtest consulta miles de veces "retorno de estos tickers en las últimas N ruedas hasta tal fecha" y "retorno de
// mantenerlos entre dos fechas". Aquí se calcula una sola vez la cantidad acumulada de huecos (NaN) para saber en O(1) si
// una ventana está completa, y los precios con relleno hacia adelante a pedido; cada consulta es indexar filas.
//
// Las reglas son las de siempre: una ventana vale solo para los tickers sin ningún hueco en ella.
type Matrix struct {
	*model.Prices
	// nanCum[k*cols+j] = cantidad de NaN del ticker j en las filas [0, k): una ventana [a, b) está completa si
	// nanCum[b*cols+j] == nanCum[a*cols+j]
	nanCum  []int32
	once    sync.Once
	ffilled []float64
}

// NewMatrix prepara la matriz de precios (con las fechas ordenadas de menor a mayor).
func NewMatrix(p *model.Prices) *Matrix {
	rows, cols := p.Rows(), p.Cols()
	nan := make([]int32, (rows+1)*cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			n := nan[i*cols+j]
			if math.IsNaN(p.Px[i*cols+j]) {
				n++
			}
			nan[(i+1)*cols+j] = n
		}
	}
	return &Matrix{Prices: p, nanCum: nan}
}

// RowsUntil es la cantidad de ruedas con fecha <= when (la última es la fila RowsUntil(when) - 1).
func (m *Matrix) RowsUntil(when time.Time) int {
	return sort.Search(m.Rows(), func(i int) bool { return m.Dates[i].After(when) })
}

func (m *Matrix) complete(from, to, col int) bool {
	c := m.Cols()
	return m.nanCum[to*c+col] == m.nanCum[from*c+col]
}

// Ranked es un ticker con su retorno.
type Ranked struct {
	Ticker string
	Return float64
}

// Returns es el retorno de cada ticker en lookback ruedas que terminan skip ruedas antes de when (skip-month: se ignora el
// último mes), de mayor a menor. Solo entran los tickers sin huecos en la ventana de lookback + 1 ruedas y que existen en
// la matriz. Exige que haya al menos lookback + skip + 1 ruedas hasta when (ok = false si no).
func (m *Matrix) Returns(tickers []string, when time.Time, lookback, skip int) (out []Ranked, ok bool) {
	end := m.RowsUntil(when) - skip
	start := end - (lookback + 1)
	if start < 0 {
		return nil, false
	}
	for _, t := range tickers {
		j, found := m.Col(t)
		if !found || !m.complete(start, end, j) {
			continue
		}
		first, last := m.At(start, j), m.At(end-1, j)
		if r := last/first - 1; !math.IsNaN(r) {
			out = append(out, Ranked{t, r})
		}
	}
	slices.SortStableFunc(out, func(a, b Ranked) int { return cmp.Compare(b.Return, a.Return) })
	return out, true
}

// HoldReturn es el retorno medio (ponderación igual) de mantener los tickers entre start y end (ambos inclusive): (retorno,
// tickers con datos, última rueda). ok = false si hay una rueda o menos, o si ninguno tiene la ventana completa.
func (m *Matrix) HoldReturn(tickers []string, start, end time.Time) (ret float64, kept []string, lastDay time.Time, ok bool) {
	lo := sort.Search(m.Rows(), func(i int) bool { return !m.Dates[i].Before(start) })
	hi := m.RowsUntil(end)
	if hi-lo <= 1 {
		return 0, nil, time.Time{}, false
	}
	sum := 0.0
	for _, t := range tickers {
		j, found := m.Col(t)
		if !found || !m.complete(lo, hi, j) {
			continue
		}
		sum += m.At(hi-1, j)/m.At(lo, j) - 1
		kept = append(kept, t)
	}
	if len(kept) == 0 {
		return 0, nil, time.Time{}, false
	}
	return sum / float64(len(kept)), kept, m.Dates[hi-1], true
}

// LastPrices es el último precio disponible de cada ticker hasta when (NaN si todavía no cotizaba o no existe).
func (m *Matrix) LastPrices(tickers []string, when time.Time) []float64 {
	m.once.Do(func() {
		rows, cols := m.Rows(), m.Cols()
		ff := make([]float64, len(m.Px))
		copy(ff, m.Px)
		for i := 1; i < rows; i++ {
			for j := 0; j < cols; j++ {
				if math.IsNaN(ff[i*cols+j]) {
					ff[i*cols+j] = ff[(i-1)*cols+j]
				}
			}
		}
		m.ffilled = ff
	})
	out := make([]float64, len(tickers))
	row := m.RowsUntil(when) - 1
	for k, t := range tickers {
		j, found := m.Col(t)
		if !found || row < 0 {
			out[k] = math.NaN()
			continue
		}
		out[k] = m.ffilled[row*m.Cols()+j]
	}
	return out
}
