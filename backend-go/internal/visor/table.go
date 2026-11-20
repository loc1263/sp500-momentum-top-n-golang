// Arma la tabla del Visor: perfiles de columnas, reglas de color, formato y secciones de instrumentos.
//
// Fecha: 2026-11-19

package visor

import (
	"fmt"
	"sort"
	"strings"

	"top3/internal/config"
)

const (
	Stocks = "acciones"
	ETFs   = "etf"
)

type Profile struct {
	ID, Label string
}

func (p Profile) IsETF() bool { return p.ID == ETFs }

var Profiles = []Profile{{Stocks, "Acciones"}, {ETFs, "ETF"}}

type Rule struct {
	Op    string
	Limit float64
}

var GreenRules = map[string]Rule{
	"sharpe":         {">", 1},
	"max_drawdown":   {">=", -25},
	"dist_max52":     {">=", -10},
	"dividend_yield": {">=", 2},
	"expense_ratio":  {"<=", 0.20},
}

const (
	Beats    = "SUPERA"
	NotBeats = "NO SUPERA"
)

func IsSignCol(col string) bool {
	return (strings.HasPrefix(col, "ret_") || strings.HasPrefix(col, "ytd") || strings.HasPrefix(col, "rel_spy")) && col != "ytd_spy" || col == "vs_ma200"
}

func Format(col string) string {
	switch {
	case strings.HasPrefix(col, "ret_"), strings.HasPrefix(col, "ytd"), strings.HasPrefix(col, "rel_spy"),
		col == "max_drawdown", col == "dist_max52", col == "vs_ma200", col == "dividend_yield":
		return "pct"
	case col == "sharpe", col == "beta", col == "expense_ratio":
		return "dec"
	case col == "market_cap_B", col == "aum_B":
		return "int"
	}
	return ""
}

func (r Rule) ok(v float64) bool {
	switch r.Op {
	case ">":
		return v > r.Limit
	case ">=":
		return v >= r.Limit
	default:
		return v <= r.Limit
	}
}

type Instrument struct {
	Ticker   string
	Sector   string
	Category string
	Static
	Leveraged bool
	Metrics
}

type Ref struct {
	YTD  float64
	Rets map[int]*float64
}

type Section struct {
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Cols   []string          `json:"cols"`
	Rows   [][]any           `json:"rows"`
	Fmt    map[string]string `json:"fmt"`
	Sign   []string          `json:"sign"`
	Thr    map[string][2]any `json:"thr"`
	Search []int             `json:"search"`
}

var textCols = map[string]bool{"ticker": true, "nombre": true, "sector": true, "categoria": true, "vs_spy": true}

func columns(p Profile) []string {
	cols := []string{"ticker", "nombre"}
	if p.IsETF() {
		cols = append(cols, "categoria", "aum_B", "expense_ratio")
	} else {
		cols = append(cols, "sector", "categoria", "market_cap_B")
	}
	for _, m := range Months {
		cols = append(cols, fmt.Sprintf("ret_%dm", m))
	}
	for _, y := range Years {
		cols = append(cols, fmt.Sprintf("ret_%dy", y))
	}
	cols = append(cols, "sharpe", "beta", "max_drawdown", "dist_max52", "vs_ma200")
	for _, y := range RelYears {
		cols = append(cols, fmt.Sprintf("rel_spy_%dy", y))
	}
	return append(cols, "dividend_yield", "ytd", "ytd_spy", "ytd_vs_spy", "vs_spy", "verdes")
}

func Build(p Profile, ins []Instrument, ref Ref, cfg config.Visor, explicit map[string]bool) Section {
	cols := columns(p)
	sec := Section{ID: p.ID, Label: p.Label, Cols: cols, Rows: [][]any{}, Fmt: map[string]string{}, Sign: []string{}, Thr: map[string][2]any{}}
	for i, c := range cols {
		if f := Format(c); f != "" {
			sec.Fmt[c] = f
		}
		if IsSignCol(c) {
			sec.Sign = append(sec.Sign, c)
		}
		if r, ok := GreenRules[c]; ok {
			sec.Thr[c] = [2]any{r.Op, r.Limit}
		}
		if textCols[c] {
			sec.Search = append(sec.Search, i)
		}
	}
	type sized struct {
		size float64
		row  []any
	}
	var rows []sized
	for _, in := range ins {
		var size *float64
		if p.IsETF() {
			if in.Leveraged && !explicit[in.Ticker] && !cfg.IncludeLeveraged {
				continue
			}
			size = in.AUM
		} else {
			if in.MarketCap == nil || *in.MarketCap < cfg.CapMinBillions*1e9 {
				continue
			}
			size = in.MarketCap
		}
		v := values(p, in, ref, cfg)
		v["verdes"] = greens(v)
		row := make([]any, len(cols))
		for i, c := range cols {
			row[i] = v[c]
		}
		s := -1.0
		if size != nil {
			s = *size
		}
		rows = append(rows, sized{s, row})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].size > rows[j].size })
	for _, r := range rows {
		sec.Rows = append(sec.Rows, r.row)
	}
	return sec
}

func values(p Profile, in Instrument, ref Ref, cfg config.Visor) map[string]any {
	v := map[string]any{"ticker": in.Ticker, "nombre": truncate(in.Name, 40)}
	if p.IsETF() {
		v["categoria"] = in.Category
		v["aum_B"] = scaled(in.AUM, 1e9)
		v["expense_ratio"] = val(in.ExpenseRatio)
	} else {
		v["sector"] = nilIfEmpty(in.Sector)
		if *in.MarketCap >= cfg.CapMegaBillions*1e9 {
			v["categoria"] = "Mega cap"
		} else {
			v["categoria"] = "Super large cap"
		}
		v["market_cap_B"] = scaled(in.MarketCap, 1e9)
	}
	for _, m := range Months {
		v[fmt.Sprintf("ret_%dm", m)] = val(in.RetMonths[m])
	}
	for _, y := range Years {
		v[fmt.Sprintf("ret_%dy", y)] = val(in.RetYears[y])
	}
	if in.CAGR3 != nil && in.Volatility != nil && *in.Volatility != 0 {
		v["sharpe"] = (*in.CAGR3 - cfg.Rf) / *in.Volatility
	}
	v["beta"], v["max_drawdown"], v["dist_max52"] = val(in.Beta), val(in.MaxDrawdown), val(in.DistMax52)
	v["vs_ma200"], v["dividend_yield"], v["ytd"] = val(in.VsMA200), val(in.DivYield), val(in.YTD)
	for _, y := range RelYears {
		if r, spy := in.RetYears[y], ref.Rets[y]; r != nil && spy != nil {
			v[fmt.Sprintf("rel_spy_%dy", y)] = *r - *spy
		}
	}
	v["ytd_spy"] = ref.YTD
	if in.YTD != nil {
		diff := *in.YTD - ref.YTD
		v["ytd_vs_spy"] = diff
		v["vs_spy"] = NotBeats
		if diff > 0 {
			v["vs_spy"] = Beats
		}
	}
	return v
}

func greens(v map[string]any) int {
	n := 0
	for c, x := range v {
		switch {
		case IsSignCol(c):
			if f, ok := x.(float64); ok && f > 0 {
				n++
			}
		case c == "vs_spy":
			if x == Beats {
				n++
			}
		default:
			if r, ok := GreenRules[c]; ok {
				if f, ok := x.(float64); ok && r.ok(f) {
					n++
				}
			}
		}
	}
	return n
}

func val(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func scaled(p *float64, div float64) any {
	if p == nil {
		return nil
	}
	return *p / div
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
