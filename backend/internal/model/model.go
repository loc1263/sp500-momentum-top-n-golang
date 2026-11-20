// Package model tiene los tipos de datos compartidos (precios en matriz, resultados del backtest) sin depender de
// ningún otro paquete del proyecto, para que los usen tanto los repositorios SQLite como el motor.
//
// Fecha: 2026-11-19
package model

import (
	"math"
	"sort"
	"time"
)

var NaN = math.NaN()

type PriceRow struct {
	Ticker string
	Date   string
	Close  float64
}

type Prices struct {
	Dates   []time.Time
	Tickers []string
	Px      []float64
	col     map[string]int
}

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

func FromRows(rows []PriceRow) *Prices {
	parsed := map[string]time.Time{}
	tickerSet := map[string]struct{}{}
	valid := rows[:0:0]
	for _, r := range rows {
		if _, ok := parsed[r.Date]; !ok {
			t, err := time.Parse("2006-01-02", r.Date)
			if err != nil {
				continue
			}
			parsed[r.Date] = t
		}
		tickerSet[r.Ticker] = struct{}{}
		valid = append(valid, r)
	}
	rows = valid
	dateSet := make(map[string]struct{}, len(parsed))
	for d := range parsed {
		dateSet[d] = struct{}{}
	}
	days := sortedKeys(dateSet)
	tickers := sortedKeys(tickerSet)
	dates := make([]time.Time, len(days))
	rowOf := make(map[string]int, len(days))
	for i, d := range days {
		dates[i] = parsed[d]
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

func (p *Prices) Rows() int { return len(p.Dates) }
func (p *Prices) Cols() int { return len(p.Tickers) }

func (p *Prices) At(i, j int) float64 { return p.Px[i*len(p.Tickers)+j] }

func (p *Prices) Col(ticker string) (int, bool) {
	j, ok := p.col[ticker]
	return j, ok
}

func (p *Prices) Has(ticker string) bool { _, ok := p.col[ticker]; return ok }

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

func (p *Prices) Tail(n int) *Prices {
	start := max(p.Rows()-n, 0)
	return p.Slice(start, p.Rows())
}

func (p *Prices) Slice(from, to int) *Prices {
	out := &Prices{Dates: p.Dates[from:to], Tickers: p.Tickers, Px: p.Px[from*len(p.Tickers) : to*len(p.Tickers)], col: p.col}
	return out
}

func (p *Prices) TrimEmptyTail() *Prices {
	end := p.Rows()
	for end > 0 {
		priced := 0
		for j := 0; j < p.Cols(); j++ {
			if !math.IsNaN(p.At(end-1, j)) {
				priced++
			}
		}
		if priced*2 >= p.Cols() {
			break
		}
		end--
	}
	return p.Slice(0, end)
}

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

func (p *Prices) LastValid(j, row int) float64 {
	for i := min(row, p.Rows()-1); i >= 0; i-- {
		if v := p.At(i, j); !math.IsNaN(v) {
			return v
		}
	}
	return math.NaN()
}

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
