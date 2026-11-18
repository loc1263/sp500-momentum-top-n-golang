package visor

import (
	"fmt"
	"sort"
	"strings"

	"top3/internal/config"
)

// Tipos de instrumento (id de cada tabla en el JSON).
const (
	Stocks = "acciones"
	ETFs   = "etf"
)

// Profile es un tipo de instrumento: su id y el nombre que se muestra en la pestaña.
type Profile struct {
	ID, Label string
}

// IsETF dice si el perfil es el de los ETFs.
func (p Profile) IsETF() bool { return p.ID == ETFs }

// Profiles son los tipos de instrumento, en el orden de las pestañas.
var Profiles = []Profile{{Stocks, "Acciones"}, {ETFs, "ETF"}}

// Rule es el criterio de color verde de una columna: la celda es verde si `valor <Op> Limit`.
type Rule struct {
	Op    string
	Limit float64
}

// GreenRules son los criterios de las columnas con umbral (el color por signo y vs_spy se tratan aparte).
var GreenRules = map[string]Rule{
	"sharpe":         {">", 1},
	"max_drawdown":   {">=", -25},
	"dist_max52":     {">=", -10},
	"dividend_yield": {">=", 2},
	"expense_ratio":  {"<=", 0.20}, // solo ETFs
}

// Para ETFs y acciones, SUPERA / NO SUPERA compara el YTD contra el de SPY.
const (
	Beats    = "SUPERA"
	NotBeats = "NO SUPERA"
)

// IsSignCol dice si la columna es verde cuando es > 0 y roja cuando es < 0.
func IsSignCol(col string) bool {
	return (strings.HasPrefix(col, "ret_") || strings.HasPrefix(col, "ytd") || strings.HasPrefix(col, "rel_spy")) && col != "ytd_spy" || col == "vs_ma200"
}

// Format es el tipo de formato de una columna: "pct" (con %), "dec" (2 decimales), "int" (miles) o "" (texto).
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
	default: // "<="
		return v <= r.Limit
	}
}

// Instrument es un instrumento con todo lo que hace falta para armar su fila.
type Instrument struct {
	Ticker   string
	Sector   string // acciones
	Category string // ETFs
	Static
	Leveraged bool
	Metrics
}

// Ref es la referencia de mercado (SPY): su YTD y sus rendimientos a 1 y 3 años.
type Ref struct {
	YTD  float64
	Rets map[int]*float64
}

// Section es una tabla del JSON: columnas, filas (null = sin dato) y cómo se formatea y se colorea cada columna.
type Section struct {
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Cols   []string          `json:"cols"`
	Rows   [][]any           `json:"rows"`
	Fmt    map[string]string `json:"fmt"`
	Sign   []string          `json:"sign"`
	Thr    map[string][2]any `json:"thr"`
	Search []int             `json:"search"` // columnas de texto donde busca el buscador
}

var textCols = map[string]bool{"ticker": true, "nombre": true, "sector": true, "categoria": true, "vs_spy": true}

// columns son las columnas de una tabla, en orden.
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

// Build arma la tabla de un tipo de instrumento: filtra, calcula Sharpe y relativos contra SPY, cuenta los verdes y ordena por
// tamaño (AUM o capitalización). explicit son los ETFs pedidos a mano, que se muestran aunque sean apalancados.
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
				continue // del universo se descartan; los pedidos a mano se muestran
			}
			size = in.AUM
		} else {
			if in.MarketCap == nil || *in.MarketCap < cfg.CapMinBillions*1e9 {
				continue // acciones preferentes (sin capitalización propia) o bajo el umbral
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
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].size > rows[j].size }) // sin tamaño al final
	for _, r := range rows {
		sec.Rows = append(sec.Rows, r.row)
	}
	return sec
}

// values son los valores de cada columna de una fila (nil = sin dato; los porcentajes ya están en %).
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

// greens cuenta las celdas verdes de una fila, con las mismas reglas que usa el frontend para pintar.
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

// truncate corta s a n caracteres.
func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
