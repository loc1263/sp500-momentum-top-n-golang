// Página «Modelo por score» (Quant): cómo se calcula, carteras de Markowitz con pesos por acción y por sector, y el ranking del
// S&P 500 con tres vistas de columnas (indicadores, salud, técnico) que table.js alterna sin recargar.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"topwx/internal/quant"
)

// quantData es el bloque «quant» de report.json. Los números van como *float64 para distinguir «sin dato» (null) de cero.
type quantData struct {
	AsOf     string         `json:"as_of"`
	Settings quant.Settings `json:"settings"`
	Coverage quant.Coverage `json:"coverage"`
	Ranking  []quantRow     `json:"ranking"`
	Mk       struct {
		Tickers []struct {
			Ticker    string   `json:"ticker"`
			ExpReturn *float64 `json:"exp_return"`
			Vol       *float64 `json:"vol"`
		} `json:"tickers"`
		Portfolios    []quant.Portfolio `json:"portfolios"`
		MinVol        float64           `json:"min_vol"`
		VolTarget     float64           `json:"vol_target"`
		WMax          float64           `json:"w_max"`
		SectorMax     float64           `json:"sector_max"`
		SectorRelaxed bool              `json:"sector_relaxed"`
		Main          string            `json:"main"`
	} `json:"markowitz"`
}

type quantRow struct {
	Rank            int                 `json:"rank"`
	Ticker          string              `json:"ticker"`
	Name            string              `json:"name"`
	Sector          string              `json:"sector"`
	Score           *float64            `json:"score"`
	Momentum        *float64            `json:"momentum"`
	Health          *float64            `json:"health"`
	Technical       *float64            `json:"technical"`
	Groups          int                 `json:"groups"`
	HealthGroups    map[string]*float64 `json:"health_groups"`
	TechnicalGroups map[string]*float64 `json:"technical_groups"`
	RSI             *float64            `json:"rsi"`
	MacdHist        *float64            `json:"macd_hist"`
	VsEma50         *float64            `json:"vs_ema50"`
	VsEma200        *float64            `json:"vs_ema200"`
	Mom121          *float64            `json:"mom_12_1"`
	Mom61           *float64            `json:"mom_6_1"`
	ROE             *float64            `json:"roe"`
	OperatingMargin *float64            `json:"operating_margin"`
	DebtToEquity    *float64            `json:"debt_to_equity"`
	CurrentRatio    *float64            `json:"current_ratio"`
	FcfMargin       *float64            `json:"fcf_margin"`
	FcfYield        *float64            `json:"fcf_yield"`
	CashConversion  *float64            `json:"cash_conversion"`
	FcfPositive     *float64            `json:"fcf_positive_years"`
}

var shortSector = map[string]string{
	"Basic Materials": "Materials", "Communication Services": "Communication", "Consumer Cyclical": "Cyclical",
	"Consumer Defensive": "Defensive", "Financial Services": "Financial", "Real Estate": "Realty",
}

var quantHelp = map[string]string{
	"score":              "Score final, de 0 a 100: promedio de los scores de momentum, salud financiera y técnico.",
	"momentum":           "Score de 0 a 100 según cuánto subió la acción en 12 y 6 meses, sin contar el último. 100 = la mejor.",
	"health":             "Score de 0 a 100 de la calidad de los últimos balances: rentabilidad, solvencia, liquidez y caja.",
	"technical":          "Score de 0 a 100 del estado del precio y el volumen: tendencia, impulso, volumen y posición.",
	"mom_12_1":           "Retorno de los últimos 12 meses sin contar el último mes.",
	"mom_6_1":            "Retorno de los últimos 6 meses sin contar el último mes.",
	"roe":                "Rentabilidad sobre patrimonio: utilidad neta dividida por el patrimonio de los accionistas.",
	"operating_margin":   "Margen operativo: qué parte de las ventas queda como utilidad operativa.",
	"debt_to_equity":     "Deuda sobre patrimonio. Más bajo = menos apalancada.",
	"current_ratio":      "Liquidez corriente: activos corrientes sobre pasivos corrientes. Mide si puede pagar sus deudas de corto plazo.",
	"fcf_margin":         "Flujo de caja libre (FCF) sobre ventas: cuánta caja sobra por cada dólar vendido.",
	"fcf_yield":          "Flujo de caja libre sobre la capitalización de mercado: la caja que genera respecto de lo que cuesta la empresa.",
	"cash_conversion":    "Flujo de caja libre sobre utilidad neta: qué parte de la utilidad se convierte en caja.",
	"fcf_positive_years": "Parte de los últimos 4 años con flujo de caja libre positivo.",
	"rsi":                "Índice de fuerza relativa (0-100). Alto = subió mucho últimamente; por encima de 70 suele indicar sobrecompra.",
	"vs_ema50":           "Distancia del precio a su media móvil exponencial de 50 ruedas. Positivo = por encima.",
	"vs_ema200":          "Distancia del precio a su media móvil exponencial de 200 ruedas. Positivo = por encima.",
	"macd_hist":          "Histograma MACD sobre el precio: positivo = impulso alcista, negativo = bajista.",
	"groups":             "Cantidad de grupos de indicadores financieros que se pudieron medir, de 4.",
	"hg_rentabilidad":    "Score de 0 a 100 de ROE, ROA y márgenes.",
	"hg_solvencia":       "Score de 0 a 100 de deuda/patrimonio, deuda neta/EBITDA y caja/deuda.",
	"hg_liquidez":        "Score de 0 a 100 de liquidez corriente y prueba ácida.",
	"hg_caja":            "Score de 0 a 100 del flujo libre y operativo, la conversión de utilidad y los años con flujo positivo.",
	"tg_tendencia":       "Score de 0 a 100 de las medias móviles de 20, 50 y 200 ruedas y el ADX.",
	"tg_impulso":         "Score de 0 a 100 de MACD, RSI, estocástico y Bollinger.",
	"tg_volumen":         "Score de 0 a 100 de OBV y MFI, que miden si el volumen acompaña al precio.",
	"tg_posicion":        "Score de 0 a 100 según el lugar del precio en su rango de 52 semanas.",
}

var portfolioHelp = map[string]string{
	"max_return": "Cartera con el mayor retorno esperado cuya volatilidad anual no supera el tope indicado.",
	"max_sharpe": "Cartera con el mejor Sharpe: la que más retorno da por cada unidad de riesgo asumido.",
	"min_var":    "Cartera de mínima varianza: la de menor volatilidad posible con estas acciones.",
}

// quantCol es una columna del ranking. Views dice en qué vistas se muestra (vacío = siempre): «ind», «sal» y / o «tec».
type quantCol struct{ Key, Label, Views string }

var quantCols = []quantCol{
	{"rank", "#", ""}, {"ticker", "Ticker", ""}, {"name", "Empresa", ""}, {"sector", "Sector", ""},
	{"score", "Score", ""}, {"momentum", "Momentum", ""}, {"health", "Salud", ""}, {"technical", "Técnico", ""},
	{"mom_12_1", "Mom. 12-1", "ind"}, {"mom_6_1", "Mom. 6-1", "ind"}, {"roe", "ROE", "ind"}, {"operating_margin", "Margen op.", "ind"},
	{"debt_to_equity", "Deuda/Patr.", "ind"}, {"current_ratio", "Liq. corr.", "ind"}, {"fcf_margin", "FCF/Ventas", "ind"},
	{"fcf_yield", "FCF/Cap.", "ind"}, {"cash_conversion", "FCF/Utilidad", "ind"}, {"fcf_positive_years", "Años FCF+", "ind"},
	{"rsi", "RSI", "ind tec"}, {"vs_ema50", "vs EMA50", "ind tec"}, {"vs_ema200", "vs EMA200", "ind tec"}, {"macd_hist", "MACD", "ind tec"},
	{"groups", "Grupos medidos", "sal"}, {"hg_rentabilidad", "Rentabilidad", "sal"}, {"hg_solvencia", "Solvencia", "sal"},
	{"hg_liquidez", "Liquidez", "sal"}, {"hg_caja", "Caja", "sal"},
	{"tg_tendencia", "Tendencia", "tec"}, {"tg_impulso", "Impulso", "tec"}, {"tg_volumen", "Volumen", "tec"}, {"tg_posicion", "Posición", "tec"},
}

type quantPage struct {
	AsOf, Ranked, Universe, Steps string
	TopN                          int
	WeightsNote                   string
	Constraints                   string
	Estimation                    string
	RelaxedNote, NoMaxReturnNote  string
	MkHead                        []mkHead
	MkRows                        []mkRow
	MkFoot                        []mkFoot
	SectorHead                    []mkHead
	SectorRows                    []sectorRow
	SectorCap                     string
	RankingNote                   string
	Sectors                       []string
	Views                         []option
	Cols                          []tcol
	Rows                          []trow
	Total                         int
	ColSpan                       int
}

type mkHead struct{ Label, Help string }

type mkRow struct {
	Ticker, Href, Name, Sector, SectorTitle, Score, ExpRet, ExpClass, Vol string
	Weights                                                               []mkCell
}

type mkCell struct {
	Text  string
	Class string
}

type mkFoot struct {
	Label string
	Help  string
	Cells []mkCell
}

type sectorRow struct {
	Name  string
	Cells []mkCell
}

func newQuantPage(q quantData) quantPage {
	s, cov, mk := q.Settings, q.Coverage, q.Mk
	pg := quantPage{AsOf: q.AsOf, Ranked: strconv.Itoa(cov.Ranked), Universe: strconv.Itoa(cov.Universe), TopN: s.TopN, Total: len(q.Ranking)}
	wDigits := 1
	if s.WStep >= 0.01 {
		wDigits = 0
	}
	if len(s.Weights) > 0 {
		distinct := map[float64]bool{}
		parts := make([]string, len(s.Weights))
		for i, w := range s.Weights {
			distinct[w] = true
			parts[i] = fnum(w)
		}
		if len(distinct) > 1 {
			pg.WeightsNote = " (pesos " + strings.Join(parts, " / ") + ")"
		}
	}
	pg.Constraints = fmt.Sprintf("Tres carteras sin posiciones cortas, con máximo de %s por acción y %s por sector", share(s.WMax, 0), share(mk.SectorMax, 0))
	if s.WMin != 0 {
		pg.Constraints += fmt.Sprintf("; cada posición pesa 0 o al menos %s", share(s.WMin, 0))
	}
	pg.Constraints += ". Los pesos dependen sobre todo del riesgo: tómalos como orientativos."
	pg.Estimation = fmt.Sprintf("El retorno esperado de cada acción mezcla su retorno de 12 meses (peso %s) con un retorno de largo plazo de %s (tasa libre de riesgo más prima de mercado), "+
		"porque el retorno pasado es muy ruidoso. La covarianza sale de los últimos 12 meses (Ledoit-Wolf).", share(s.MuShrink, 0), share(s.Prior, 0))
	if mk.SectorRelaxed {
		pg.RelaxedNote = fmt.Sprintf("Con los sectores de estas %d acciones no se puede invertir todo con un máximo de %s por sector: el tope se subió a %s.", s.TopN, share(s.SectorMax, 0), share(mk.SectorMax, 0))
	}
	hasMax := false
	for _, p := range mk.Portfolios {
		hasMax = hasMax || p.Key == "max_return"
	}
	if !hasMax {
		pg.NoMaxReturnNote = fmt.Sprintf("No hay cartera de máximo retorno: con estas %d acciones la volatilidad mínima posible es %s, por encima del tope de %s.", s.TopN, share(mk.MinVol, 1), share(s.VolTarget, 0))
	}

	// Carteras de Markowitz
	inputs := map[string]struct{ ret, vol *float64 }{}
	for _, t := range mk.Tickers {
		inputs[t.Ticker] = struct{ ret, vol *float64 }{t.ExpReturn, t.Vol}
	}
	for _, p := range mk.Portfolios {
		pg.MkHead = append(pg.MkHead, mkHead{Label: p.Label, Help: portfolioHelp[p.Key]})
	}
	pg.SectorHead = pg.MkHead
	weightOf := func(p quant.Portfolio, tk string) float64 {
		for _, w := range p.Weights {
			if w.Ticker == tk {
				return w.Weight
			}
		}
		return 0
	}
	sectorWeight := func(p quant.Portfolio, sector string) float64 {
		for _, w := range p.Sectors {
			if w.Sector == sector {
				return w.Weight
			}
		}
		return 0
	}
	top := q.Ranking
	if len(top) > s.TopN {
		top = top[:s.TopN]
	}
	for _, r := range top {
		in := inputs[r.Ticker]
		row := mkRow{
			Ticker: r.Ticker, Href: tickerURL("acciones", r.Ticker), Name: r.Name, Sector: sectorShort(r.Sector), Score: score(r.Score),
			ExpRet: pctFrac(in.ret, 1), ExpClass: signClass(in.ret), Vol: share0(in.vol),
		}
		if short := shortSector[r.Sector]; short != "" {
			row.SectorTitle = r.Sector
		}
		for _, p := range mk.Portfolios {
			c := mkCell{}
			if w := weightOf(p, r.Ticker); w != 0 {
				c.Text = share(w, wDigits)
			}
			if p.Key == mk.Main {
				c.Class = "main"
			}
			row.Weights = append(row.Weights, c)
		}
		pg.MkRows = append(pg.MkRows, row)
	}
	foot := mkFoot{Label: "Retorno esperado de la cartera"}
	vol := mkFoot{Label: "Volatilidad anual"}
	sharpe := mkFoot{Label: fmt.Sprintf("Sharpe (tasa libre de riesgo de %s)", share(s.RiskFree, 0)),
		Help: "Retorno por unidad de riesgo: (retorno esperado − tasa libre de riesgo) ÷ volatilidad. Más alto = mejor retorno para el riesgo asumido."}
	for _, p := range mk.Portfolios {
		foot.Cells = append(foot.Cells, mkCell{Text: pctFrac(&p.ExpReturn, 1), Class: signClass(&p.ExpReturn)})
		vol.Cells = append(vol.Cells, mkCell{Text: share(p.Vol, 1)})
		sharpe.Cells = append(sharpe.Cells, mkCell{Text: esNum(p.Sharpe, 2)})
	}
	pg.MkFoot = []mkFoot{foot, vol, sharpe}

	seen := map[string]bool{}
	var names []string
	for _, p := range mk.Portfolios {
		for _, x := range p.Sectors {
			if !seen[x.Sector] {
				seen[x.Sector] = true
				names = append(names, x.Sector)
			}
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		if len(mk.Portfolios) > 0 {
			a, b := sectorWeight(mk.Portfolios[0], names[i]), sectorWeight(mk.Portfolios[0], names[j])
			if a != b {
				return a > b
			}
		}
		return collateEs(names[i], names[j]) < 0
	})
	for _, n := range names {
		row := sectorRow{Name: n}
		for _, p := range mk.Portfolios {
			c := mkCell{}
			if w := sectorWeight(p, n); w != 0 {
				c.Text = share(w, wDigits)
				if w >= mk.SectorMax-1e-4 {
					c.Class = "atcap"
				}
			}
			row.Cells = append(row.Cells, c)
		}
		pg.SectorRows = append(pg.SectorRows, row)
	}
	pg.SectorCap = share(mk.SectorMax, 0)

	// Ranking
	note := fmt.Sprintf("Las %d acciones con los tres scores, de mejor a peor score final.", cov.Ranked)
	for _, x := range []struct {
		prefix string
		list   []string
	}{
		{"Sin un año de precios (no tienen momentum): ", cov.Excluded.NoMomentum},
		{"Sin datos financieros: ", cov.Excluded.NoFundamentals},
		{"Sin datos técnicos: ", cov.Excluded.NoTechnical},
	} {
		if len(x.list) > 0 {
			note += " " + x.prefix + strings.Join(x.list, ", ") + "."
		}
	}
	pg.RankingNote = note
	secSeen := map[string]bool{}
	for _, r := range q.Ranking {
		if !secSeen[r.Sector] {
			secSeen[r.Sector] = true
			pg.Sectors = append(pg.Sectors, r.Sector)
		}
	}
	sort.Slice(pg.Sectors, func(i, j int) bool { return collateEs(pg.Sectors[i], pg.Sectors[j]) < 0 })
	pg.Views = []option{{Value: "ind", Label: "Indicadores", Selected: true}, {Value: "sal", Label: "Salud: score por grupo"}, {Value: "tec", Label: "Técnico: score por grupo"}}

	for _, c := range quantCols {
		col := tcol{Label: c.Label, Views: c.Views, Help: quantHelp[c.Key], Dir: "asc", AriaSort: "none"}
		if c.Key != "rank" && c.Key != "ticker" && c.Key != "name" && c.Key != "sector" {
			col.Num, col.Dir = true, "desc"
		}
		if c.Key == "rank" {
			col.Initial, col.AriaSort, col.Arrow = "asc", "ascending", " ▲"
		}
		pg.Cols = append(pg.Cols, col)
	}
	pg.ColSpan = len(pg.Cols)
	for _, r := range q.Ranking {
		row := trow{Search: strings.ToLower(r.Ticker + " " + r.Name), Attrs: map[string]string{"sector": r.Sector}}
		if r.Rank <= s.TopN {
			row.Class = "top"
		}
		for _, c := range quantCols {
			cell := r.cell(c.Key)
			cell.Views = c.Views
			row.Cells = append(row.Cells, cell)
		}
		pg.Rows = append(pg.Rows, row)
	}
	return pg
}

func sectorShort(s string) string {
	if short := shortSector[s]; short != "" {
		return short
	}
	return s
}

// score y share0 formatean con coma decimal; n/d si falta el dato.
func score(p *float64) string { return dec(p, 1) }

func share0(p *float64) string {
	return derefOr(p, func(v float64) string { return share(v, 1) })
}

func (r quantRow) value(key string) *float64 {
	switch {
	case strings.HasPrefix(key, "hg_"):
		return r.HealthGroups[key[3:]]
	case strings.HasPrefix(key, "tg_"):
		return r.TechnicalGroups[key[3:]]
	}
	m := map[string]*float64{
		"score": r.Score, "momentum": r.Momentum, "health": r.Health, "technical": r.Technical, "rsi": r.RSI, "macd_hist": r.MacdHist,
		"vs_ema50": r.VsEma50, "vs_ema200": r.VsEma200, "mom_12_1": r.Mom121, "mom_6_1": r.Mom61, "roe": r.ROE, "operating_margin": r.OperatingMargin,
		"debt_to_equity": r.DebtToEquity, "current_ratio": r.CurrentRatio, "fcf_margin": r.FcfMargin, "fcf_yield": r.FcfYield,
		"cash_conversion": r.CashConversion, "fcf_positive_years": r.FcfPositive,
	}
	return m[key]
}

func (r quantRow) cell(key string) cellView {
	v := r.value(key)
	num := func(text, class string) cellView {
		return cellView{Class: strings.TrimSpace("r " + class), Text: text, V: vOf(v)}
	}
	switch key {
	case "rank":
		return cellView{Text: strconv.Itoa(r.Rank), V: strconv.Itoa(r.Rank)}
	case "ticker":
		return cellView{Text: r.Ticker, Href: tickerURL("acciones", r.Ticker), Ext: true}
	case "name":
		return cellView{Text: r.Name}
	case "sector":
		c := cellView{Class: "muted", Text: sectorShort(r.Sector)}
		if shortSector[r.Sector] != "" {
			c.Title = r.Sector
		}
		return c
	case "score":
		return num(score(v), "strong")
	case "groups":
		c := num(strconv.Itoa(r.Groups)+"/4", "")
		c.V, c.Title = strconv.Itoa(r.Groups), "grupos de indicadores financieros medidos, de 4"
		return c
	case "mom_12_1", "mom_6_1", "roe", "operating_margin", "fcf_margin", "vs_ema50", "vs_ema200", "fcf_yield", "cash_conversion":
		return num(pctFrac(v, 0), signClass(v))
	case "fcf_positive_years":
		c := num(derefOr(v, func(x float64) string { return share(x, 0) }), "")
		c.Title = "parte de los últimos 4 años con flujo de caja libre positivo"
		return c
	case "macd_hist":
		c := num(pctFrac(v, 1), signClass(v))
		c.Title = "histograma MACD sobre el precio: positivo = impulso alcista"
		return c
	case "debt_to_equity":
		return num(derefOr(v, func(x float64) string { return share(x/100, 0) }), "")
	case "current_ratio":
		return num(dec(v, 2), "")
	}
	return num(score(v), "")
}
