// Package model tiene los tipos de datos compartidos (precios en matriz, resultados del backtest) sin depender de
// ningún otro paquete del proyecto, para que los usen tanto los repositorios SQLite como el motor.
package model

import (
	"math"
	"sort"
	"time"
)

// NaN marca un precio faltante.
var NaN = math.NaN()

// PriceRow es un cierre ajustado en formato largo.
type PriceRow struct {
	Ticker string
	Date   string // YYYY-MM-DD
	Close  float64
}

// Prices es la matriz de cierres fecha x ticker. Las fechas van de menor a mayor y los huecos son NaN.
type Prices struct {
	Dates   []time.Time
	Tickers []string
	Px      []float64 // fila a fila: Px[i*len(Tickers)+j]
	col     map[string]int
}

// NewPrices arma una matriz vacía (todo NaN) de las fechas y tickers dados.
func NewPrices(dates []time.Time, tickers []string) *Prices {
	p := &Prices{Dates: dates, Tickers: tickers, Px: make([]float64, len(dates)*len(tickers))}
	for i := range p.Px {
		p.Px[i] = math.NaN()
	}
	p.index()
	return p
}

func (p *Prices) index() {
	p.col = make(map[string]int, len(p.Tickers))
	for j, t := range p.Tickers {
		p.col[t] = j
	}
}

// FromRows arma la matriz desde filas largas: fechas y tickers ordenados.
func FromRows(rows []PriceRow) *Prices {
	dateSet := map[string]struct{}{}
	tickerSet := map[string]struct{}{}
	for _, r := range rows {
		dateSet[r.Date] = struct{}{}
		tickerSet[r.Ticker] = struct{}{}
	}
	days := sortedKeys(dateSet)
	tickers := sortedKeys(tickerSet)
	dates := make([]time.Time, len(days))
	rowOf := make(map[string]int, len(days))
	for i, d := range days {
		dates[i], _ = time.Parse("2006-01-02", d)
		rowOf[d] = i
	}
	p := NewPrices(dates, tickers)
	for _, r := range rows {
		p.Px[rowOf[r.Date]*len(tickers)+p.col[r.Ticker]] = r.Close
	}
	return p
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Rows y Cols son las dimensiones de la matriz.
func (p *Prices) Rows() int { return len(p.Dates) }
func (p *Prices) Cols() int { return len(p.Tickers) }

// At es el precio de la fila i y la columna j.
func (p *Prices) At(i, j int) float64 { return p.Px[i*len(p.Tickers)+j] }

// Col devuelve la columna del ticker y si existe.
func (p *Prices) Col(ticker string) (int, bool) {
	j, ok := p.col[ticker]
	return j, ok
}

// Has dice si el ticker es una columna.
func (p *Prices) Has(ticker string) bool { _, ok := p.col[ticker]; return ok }

// Series copia la columna del ticker.
func (p *Prices) Series(ticker string) []float64 {
	j, ok := p.col[ticker]
	if !ok {
		return nil
	}
	out := make([]float64, p.Rows())
	for i := range out {
		out[i] = p.At(i, j)
	}
	return out
}

// Select devuelve una matriz nueva con solo los tickers pedidos que existen, en el orden pedido.
func (p *Prices) Select(tickers []string) *Prices {
	keep := make([]string, 0, len(tickers))
	idx := make([]int, 0, len(tickers))
	for _, t := range tickers {
		if j, ok := p.col[t]; ok {
			keep = append(keep, t)
			idx = append(idx, j)
		}
	}
	out := NewPrices(p.Dates, keep)
	for i := 0; i < p.Rows(); i++ {
		for k, j := range idx {
			out.Px[i*len(keep)+k] = p.At(i, j)
		}
	}
	return out
}

// Reindex es Select pero con una columna (toda NaN) por cada ticker pedido, existan o no.
func (p *Prices) Reindex(tickers []string) *Prices {
	out := NewPrices(p.Dates, append([]string(nil), tickers...))
	for k, t := range tickers {
		if j, ok := p.col[t]; ok {
			for i := 0; i < p.Rows(); i++ {
				out.Px[i*len(tickers)+k] = p.At(i, j)
			}
		}
	}
	return out
}

// Tail devuelve las últimas n filas (o todas si hay menos).
func (p *Prices) Tail(n int) *Prices {
	start := max(p.Rows()-n, 0)
	return p.Slice(start, p.Rows())
}

// Slice devuelve las filas [from, to) como matriz nueva (comparte el índice de tickers).
func (p *Prices) Slice(from, to int) *Prices {
	out := &Prices{Dates: p.Dates[from:to], Tickers: p.Tickers, Px: p.Px[from*len(p.Tickers) : to*len(p.Tickers)], col: p.col}
	return out
}

// Since devuelve las filas con fecha >= day.
func (p *Prices) Since(day time.Time) *Prices {
	i := sort.Search(len(p.Dates), func(i int) bool { return !p.Dates[i].Before(day) })
	return p.Slice(i, p.Rows())
}

// DropAllNaN quita las columnas sin ningún precio.
func (p *Prices) DropAllNaN() *Prices {
	var keep []string
	for j, t := range p.Tickers {
		for i := 0; i < p.Rows(); i++ {
			if !math.IsNaN(p.At(i, j)) {
				keep = append(keep, t)
				break
			}
		}
	}
	return p.Select(keep)
}

// CompleteCols son los tickers sin ningún hueco (NaN) en toda la matriz.
func (p *Prices) CompleteCols() []string {
	var out []string
	for j, t := range p.Tickers {
		ok := true
		for i := 0; i < p.Rows(); i++ {
			if math.IsNaN(p.At(i, j)) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

// LastValid es el último precio no NaN del ticker hasta la fila row inclusive (NaN si no hay).
func (p *Prices) LastValid(j, row int) float64 {
	for i := min(row, p.Rows()-1); i >= 0; i-- {
		if v := p.At(i, j); !math.IsNaN(v) {
			return v
		}
	}
	return math.NaN()
}

// MergeColumns junta matrices por columnas (unión de fechas, ordenadas); si un ticker está en varias, gana la primera.
func MergeColumns(parts ...*Prices) *Prices {
	dateSet := map[int64]time.Time{}
	var tickers []string
	seen := map[string]bool{}
	for _, p := range parts {
		if p == nil {
			continue
		}
		for _, d := range p.Dates {
			dateSet[d.Unix()] = d
		}
		for _, t := range p.Tickers {
			if !seen[t] {
				seen[t] = true
				tickers = append(tickers, t)
			}
		}
	}
	keys := make([]int64, 0, len(dateSet))
	for k := range dateSet {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })
	dates := make([]time.Time, len(keys))
	rowOf := make(map[int64]int, len(keys))
	for i, k := range keys {
		dates[i] = dateSet[k]
		rowOf[k] = i
	}
	out := NewPrices(dates, tickers)
	done := map[string]bool{}
	for _, p := range parts {
		if p == nil {
			continue
		}
		for j, t := range p.Tickers {
			if done[t] {
				continue
			}
			done[t] = true
			k := out.col[t]
			for i, d := range p.Dates {
				out.Px[rowOf[d.Unix()]*len(tickers)+k] = p.At(i, j)
			}
		}
	}
	return out
}
