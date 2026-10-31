package evaluator

import (
	"math"
	"sort"
	"strings"
)

// Level es la evaluación de una métrica: good (bien), warn (atención), bad (alerta), info (solo informativa) o na (sin datos).
type Level string

const (
	Good Level = "good"
	Warn Level = "warn"
	Bad  Level = "bad"
	Info Level = "info"
	NA   Level = "na"
)

var nan = math.NaN()

func isNaN(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }

// ptr es el número para el JSON: null si falta.
func ptr(v float64) *float64 {
	if isNaN(v) {
		return nil
	}
	v = math.Round(v*1e6) / 1e6
	return &v
}

// hi y lo evalúan "más alto es mejor" y "más bajo es mejor" con dos umbrales (bien, atención).
func hi(v, good, warn float64) Level {
	switch {
	case v >= good:
		return Good
	case v >= warn:
		return Warn
	}
	return Bad
}

func lo(v, good, warn float64) Level {
	switch {
	case v <= good:
		return Good
	case v <= warn:
		return Warn
	}
	return Bad
}

// Metric es una métrica evaluada. Su nombre, unidad y explicación están en Payload.Metrics (por ID) para no repetirlos en cada empresa.
type Metric struct {
	ID     string     `json:"id"`
	Value  *float64   `json:"v"`
	Level  Level      `json:"l"`
	Label  string     `json:"lb,omitempty"` // texto del distintivo cuando no alcanza con el color (p. ej. «Barato»)
	Text   string     `json:"t,omitempty"`  // valor que no es un número (consenso de analistas)
	Note   string     `json:"n,omitempty"`  // explicación propia de esta empresa
	Series []*float64 `json:"s,omitempty"`  // evolución anual (tendencia)
	Low    *float64   `json:"lo,omitempty"` // extremos del rango de 52 semanas
	High   *float64   `json:"hi,omitempty"`
}

// Category es un grupo de métricas con su veredicto.
type Category struct {
	ID      string     `json:"id"`
	Verdict Level      `json:"verdict"`
	Metrics []Metric   `json:"metrics,omitempty"`
	Years   []string   `json:"years,omitempty"` // años fiscales de las series de la tendencia
	Peers   *PeerTable `json:"peers,omitempty"`
}

// Company es el análisis de una empresa.
type Company struct {
	Ticker      string     `json:"ticker"`
	Name        string     `json:"name"`
	Universe    string     `json:"universe"`
	Sector      string     `json:"sector,omitempty"`
	Industry    string     `json:"industry,omitempty"`
	Currency    string     `json:"currency,omitempty"`
	FinCurrency string     `json:"fin_currency,omitempty"` // moneda de los estados financieros, si es distinta de la del precio
	Bank        bool       `json:"bank,omitempty"`         // banco o aseguradora: caja y deuda no aplican
	Price       *float64   `json:"price"`
	MarketCap   *float64   `json:"market_cap"`
	AsOf        string     `json:"as_of,omitempty"` // fecha del último estado financiero trimestral
	Verdict     Level      `json:"verdict"`
	Categories  []Category `json:"categories"`

	peerVals map[string]float64
}

// Verdict resume una lista de niveles: «good» si el promedio es alto y ninguno es malo, «bad» si es bajo.
func Verdict(levels []Level) Level {
	sum, n := 0, 0
	anyBad := false
	for _, l := range levels {
		switch l {
		case Good:
			sum += 2
		case Warn:
			sum++
		case Bad:
			anyBad = true
		default:
			continue
		}
		n++
	}
	if n == 0 {
		return NA
	}
	avg := float64(sum) / float64(n)
	switch {
	case avg >= 1.5 && !anyBad:
		return Good
	case avg >= 0.9:
		return Warn
	}
	return Bad
}

func metricsVerdict(ms []Metric) Level {
	ls := make([]Level, len(ms))
	for i, m := range ms {
		ls[i] = m.Level
	}
	return Verdict(ls)
}

// m arma una métrica: sin valor, el nivel es «na».
func m(id string, v float64, level Level) Metric {
	if isNaN(v) {
		return Metric{ID: id, Level: NA}
	}
	return Metric{ID: id, Value: ptr(v), Level: level}
}

// Analyze evalúa a una empresa con sus datos. Las categorías son: rentabilidad, crecimiento, caja, deuda y liquidez, valoración
// y tendencia; la comparación con competidores la agrega AttachPeers cuando ya están todas las empresas del universo.
func Analyze(r Raw, universe string) Company {
	c := Company{Ticker: r.Ticker, Name: r.Name, Universe: universe, Sector: r.Sector, Industry: r.Industry, Currency: r.Currency,
		Price: ptr(r.Price), MarketCap: ptr(r.MarketCap), AsOf: r.AsOf, Bank: r.isBank()}
	if !r.sameCurrency() {
		c.FinCurrency = r.FinCurrency
	}
	if c.Bank {
		r = r.withoutCashAndDebt()
	}

	pe := r.PE
	if isNaN(pe) && r.EPS > 0 && r.Price > 0 {
		pe = r.Price / r.EPS
	}
	netMargin := nan
	if !isNaN(r.NetIncome) && r.Revenue > 0 {
		netMargin = r.NetIncome / r.Revenue * 100
	}
	opMargin := nan
	if !isNaN(r.QOpIncome) && r.QRevenue > 0 {
		opMargin = r.QOpIncome / r.QRevenue * 100
	}
	roe := r.ROE
	if isNaN(roe) && !isNaN(r.NetIncome) && r.QEquity > 0 {
		roe = r.NetIncome / r.QEquity * 100
	}

	// Rentabilidad
	prof := []Metric{
		m("net_margin", netMargin, ifOK(netMargin, func() Level { return hi(netMargin, 15, 5) })),
		m("op_margin", opMargin, ifOK(opMargin, func() Level { return hi(opMargin, 15, 5) })),
		m("roe", roe, ifOK(roe, func() Level { return hi(roe, 15, 8) })),
	}
	if roe > 60 { // un ROE tan alto casi siempre es un patrimonio muy pequeño (recompras o pérdidas), no eficiencia: no cuenta como «bien»
		prof[2].Level, prof[2].Label = Warn, "Muy alto"
	}

	// Crecimiento
	quality := nan
	if pe > 0 && r.FwdPE > 0 {
		quality = r.FwdPE / pe
	}
	grow := []Metric{
		m("rev_growth", r.RevGrowth, ifOK(r.RevGrowth, func() Level { return hi(r.RevGrowth, 10, 3) })),
		m("eps_growth", r.EpsGrowth, ifOK(r.EpsGrowth, func() Level { return hi(r.EpsGrowth, 10, 0) })),
		m("earnings_quality", quality, Good),
	}
	if quality > 1.15 {
		grow[2].Level = Warn
		grow[2].Note = "El Forward P/E es mayor que el P/E: los analistas esperan una utilidad menor que la de los últimos 12 meses. Suele indicar ganancias puntuales que no se repetirán."
	} else if quality < 0.85 {
		grow[2].Note = "El Forward P/E es menor que el P/E: se espera que la utilidad siga creciendo."
	} else if !isNaN(quality) {
		grow[2].Note = "Forward P/E parecido al P/E: se espera una utilidad estable."
	}

	// Caja. El capex se deduce de flujo operativo − flujo libre, que es lo que Yahoo informa.
	fcf, ocf := r.FCF, r.OCF
	capex := nan
	if !isNaN(ocf) && !isNaN(fcf) {
		capex = ocf - fcf
	}
	fcfMargin := nan
	if !isNaN(fcf) && r.Revenue > 0 {
		fcfMargin = fcf / r.Revenue * 100
	}
	cover := nan
	if !isNaN(ocf) && capex > 0 {
		cover = ocf / capex
	}
	fcfYield := nan
	if !isNaN(fcf) && r.MarketCap > 0 && r.sameCurrency() { // si el balance está en otra moneda que el precio, el cociente no vale
		fcfYield = fcf / r.MarketCap * 100
	}
	fcfLevel := Bad
	if fcf > 0 {
		fcfLevel = Good
	}
	cash := []Metric{
		m("fcf", fcf, fcfLevel),
		m("fcf_margin", fcfMargin, ifOK(fcfMargin, func() Level { return hi(fcfMargin, 8, 0) })),
		m("capex_cover", cover, ifOK(cover, func() Level { return hi(cover, 1.1, 0.8) })),
		m("fcf_yield", fcfYield, ifOK(fcfYield, func() Level { return hi(fcfYield, 3, 0) })),
	}

	// Deuda y liquidez
	netDebt := nan
	if !isNaN(r.Debt) && !isNaN(r.Cash) {
		netDebt = r.Debt - r.Cash
	}
	netDebtYears := nan
	if !isNaN(netDebt) && ocf > 0 {
		netDebtYears = math.Max(netDebt, 0) / ocf
	}
	interestCover := nan
	if !isNaN(r.QOpIncome) && r.QInterest > 0 {
		interestCover = r.QOpIncome / r.QInterest
	}
	debtEquity := nan
	switch {
	case !isNaN(r.Debt) && r.QEquity > 0:
		debtEquity = r.Debt / r.QEquity
	case !isNaN(r.DebtToEquity) && r.DebtToEquity >= 0: // Yahoo lo informa en %
		debtEquity = r.DebtToEquity / 100
	}
	ndLevel := NA
	if !isNaN(netDebt) {
		switch {
		case netDebt <= 0:
			ndLevel = Good
		case isNaN(netDebtYears):
			ndLevel = Bad
		default:
			ndLevel = lo(netDebtYears, 1, 3)
		}
	}
	nd := m("net_debt", netDebt, ndLevel)
	if netDebt < 0 {
		nd.Label = "Caja neta"
	}
	debt := []Metric{
		nd,
		m("net_debt_years", netDebtYears, ifOK(netDebtYears, func() Level { return lo(netDebtYears, 1, 3) })),
		m("interest_cover", interestCover, ifOK(interestCover, func() Level { return hi(interestCover, 8, 3) })),
		m("current_ratio", r.CurrentRatio, ifOK(r.CurrentRatio, func() Level { return hi(r.CurrentRatio, 1.2, 1) })),
		m("debt_equity", debtEquity, ifOK(debtEquity, func() Level { return lo(debtEquity, 0.5, 1) })),
	}

	// Valoración
	upside := nan
	if !isNaN(r.Target) && r.Price > 0 {
		upside = (r.Target/r.Price - 1) * 100
	}
	pos := nan
	if r.W52High > r.W52Low && !isNaN(r.Price) {
		pos = math.Min(100, math.Max(0, (r.Price-r.W52Low)/(r.W52High-r.W52Low)*100))
	}
	peM, fpeM := m("pe", pe, NA), m("fwd_pe", r.FwdPE, NA)
	peM.Level, peM.Label = valuation(pe)
	fpeM.Level, fpeM.Label = valuation(r.FwdPE)
	upM := m("target_upside", upside, ifOK(upside, func() Level { return hi(upside, 15, 0) }))
	switch {
	case isNaN(upside):
	case upside >= 15:
		upM.Label = "Alza"
	case upside >= 0:
		upM.Label = "Moderado"
	default:
		upM.Label = "Bajo el precio"
	}
	posM := m("range_52w", pos, Info)
	if !isNaN(pos) {
		posM.Low, posM.High = ptr(r.W52Low), ptr(r.W52High)
		switch {
		case pos > 80:
			posM.Note = "Cerca del máximo del año."
		case pos < 20:
			posM.Note = "Cerca del mínimo del año."
		default:
			posM.Note = "En la zona media del rango del año."
		}
	}
	betaM := m("beta", r.Beta, ifOK(r.Beta, func() Level { return lo(r.Beta, 1, 1.5) }))
	switch {
	case isNaN(r.Beta):
	case r.Beta <= 1:
		betaM.Label = "Estable"
	case r.Beta <= 1.5:
		betaM.Label = "Algo volátil"
	default:
		betaM.Label = "Muy volátil"
	}
	divM := m("dividend", r.Dividend, Info)
	if isNaN(r.Dividend) || r.Dividend == 0 {
		divM = Metric{ID: "dividend", Level: Info, Text: "No paga"}
	}
	ratingM := Metric{ID: "rating", Level: NA}
	if r.Rating != "" {
		ratingM = Metric{ID: "rating", Level: Info, Text: r.Rating}
	}
	val := []Metric{peM, fpeM, upM, posM, betaM, divM, ratingM}

	cats := []Category{
		{ID: "prof", Metrics: prof}, {ID: "grow", Metrics: grow}, {ID: "cash", Metrics: cash},
		{ID: "debt", Metrics: debt}, {ID: "val", Metrics: val}, trend(r),
	}
	for i := range cats {
		cats[i].Verdict = metricsVerdict(cats[i].Metrics)
	}
	c.Categories = cats
	c.peerVals = map[string]float64{"pe": pe, "fwd_pe": r.FwdPE, "rev_growth": r.RevGrowth, "net_margin": netMargin,
		"op_margin": opMargin, "roe": roe, "fcf_margin": fcfMargin}
	c.Verdict = overall(c.Categories)
	return c
}

// ifOK evalúa f solo si v es un número.
func ifOK(v float64, f func() Level) Level {
	if isNaN(v) {
		return NA
	}
	return f()
}

// valuation es el nivel y el texto del P/E: bajo 15 barato, hasta 30 medio, más caro; sin utilidad es una alerta.
func valuation(v float64) (Level, string) {
	switch {
	case isNaN(v):
		return NA, ""
	case v <= 0:
		return Bad, "Sin utilidad"
	case v < 15:
		return Good, "Barato"
	case v <= 30:
		return Warn, "Medio"
	}
	return Bad, "Caro"
}

// overall resume los veredictos de las categorías (la comparación con competidores incluida, si ya está).
func overall(cats []Category) Level {
	ls := make([]Level, 0, len(cats))
	for _, c := range cats {
		ls = append(ls, c.Verdict)
	}
	return Verdict(ls)
}

// ---- Tendencia ----

// firstLast son los extremos con dato de una serie: posiciones y valores.
func firstLast(a []float64) (i, j int, first, last float64, ok bool) {
	i = -1
	for k, v := range a {
		if !isNaN(v) {
			if i < 0 {
				i = k
			}
			j = k
		}
	}
	if i < 0 || j == i {
		return 0, 0, 0, 0, false
	}
	return i, j, a[i], a[j], true
}

// cagr es el crecimiento anual compuesto en %; nan si algún extremo no es positivo.
func cagr(a, b float64, years int) float64 {
	if a > 0 && b > 0 && years > 0 {
		return (math.Pow(b/a, 1/float64(years)) - 1) * 100
	}
	return nan
}

func series(a []float64) []*float64 {
	out := make([]*float64, len(a))
	for i, v := range a {
		out[i] = ptr(v)
	}
	return out
}

// trend evalúa la evolución de los últimos años fiscales: ingresos, utilidad, flujo libre, margen operativo y deuda.
func trend(r Raw) Category {
	cat := Category{ID: "trend"}
	t := r.Trend
	if len(t.Years) == 0 {
		return cat
	}
	cat.Years = t.Years
	add := func(id string, vals []float64, value float64, level Level, note string) {
		mt := m(id, value, level)
		mt.Series, mt.Note = series(vals), note
		cat.Metrics = append(cat.Metrics, mt)
	}
	span := func(i, j int) string { return " entre " + t.Years[i] + " y " + t.Years[j] }

	var revCagr float64 = nan
	if i, j, a, b, ok := firstLast(t.Revenue); ok {
		revCagr = cagr(a, b, j-i)
		add("tr_revenue", t.Revenue, b, ifOK(revCagr, func() Level { return hi(revCagr, 10, 3) }), "Crecimiento anual compuesto de las ventas"+span(i, j)+".")
		cat.Metrics[len(cat.Metrics)-1].Label = pctLabel(revCagr)
	}
	if i, j, a, b, ok := firstLast(t.NetIncome); ok {
		c := cagr(a, b, j-i)
		lv := Bad
		if isNaN(c) {
			if b > 0 {
				lv = Warn
			}
		} else {
			lv = hi(c, 10, 3)
		}
		add("tr_net_income", t.NetIncome, b, lv, "Crecimiento anual compuesto de la ganancia"+span(i, j)+".")
		cat.Metrics[len(cat.Metrics)-1].Label = pctLabel(c)
	}
	if i, j, a, b, ok := firstLast(t.FCF); ok {
		c := cagr(a, b, j-i)
		lv := Bad
		switch {
		case b <= 0:
		case isNaN(c):
			lv = Warn
			if b > a {
				lv = Good
			}
		case c >= 5:
			lv = Good
		case c >= -5:
			lv = Warn
		}
		note := "Cómo evoluciona el efectivo que queda después de invertir" + span(i, j) + "."
		if b <= 0 {
			note = "El último año la empresa gasta más caja de la que genera."
		}
		add("tr_fcf", t.FCF, b, lv, note)
		cat.Metrics[len(cat.Metrics)-1].Label = pctLabel(c)
	}
	if i, j, a, b, ok := firstLast(t.OpMargin); ok {
		d := b - a
		lv := Bad
		switch {
		case d >= 1:
			lv = Good
		case d > -2:
			lv = Warn
		}
		add("tr_op_margin", t.OpMargin, b, lv, "Si el margen sube, la empresa gana más por cada dólar vendido"+span(i, j)+".")
		cat.Metrics[len(cat.Metrics)-1].Label = signed(d, 1) + " pp"
	}
	if i, j, a, b, ok := firstLast(t.Debt); ok {
		c := cagr(a, b, j-i)
		lv := Warn
		switch {
		case b <= a:
			lv = Good
		case !isNaN(revCagr) && !isNaN(c):
			switch {
			case c <= revCagr:
				lv = Good
			case c <= revCagr+10:
				lv = Warn
			default:
				lv = Bad
			}
		}
		add("tr_debt", t.Debt, b, lv, "La deuda debería crecer menos que el negocio"+span(i, j)+".")
		cat.Metrics[len(cat.Metrics)-1].Label = pctLabel(c)
	}
	return cat
}

func signed(v float64, dec int) string {
	p := math.Pow(10, float64(dec))
	s := math.Round(v*p) / p
	out := strings.Replace(formatFloat(s, dec), ".", ",", 1) // coma decimal, como el resto del sitio
	if s >= 0 {
		return "+" + out
	}
	return out
}

// pctLabel es el crecimiento anual para el distintivo de la tendencia, o vacío si no se puede calcular.
func pctLabel(c float64) string {
	if isNaN(c) {
		return ""
	}
	return signed(c, 1) + " %/año"
}

// ---- Competidores ----

// PeerRow es una métrica comparada con la de los competidores.
type PeerRow struct {
	ID     string     `json:"id"`
	Mine   *float64   `json:"mine"`
	Values []*float64 `json:"values"` // una por competidor, en el orden de PeerTable.Tickers
	Median *float64   `json:"median"`
	Level  Level      `json:"l"`
	Best   int        `json:"best"` // -1 = la empresa, 0.. = competidor con el mejor valor; -2 si no hay
}

// PeerTable es la comparación con los competidores.
type PeerTable struct {
	Tickers []string  `json:"tickers"`
	Names   []string  `json:"names"`
	Rows    []PeerRow `json:"rows"`
}

// peerMetrics son las métricas que se comparan; lowerBetter marca las que se prefieren bajas (P/E).
var peerMetrics = []struct {
	ID          string
	LowerBetter bool
}{{"pe", true}, {"fwd_pe", true}, {"rev_growth", false}, {"net_margin", false}, {"op_margin", false}, {"roe", false}, {"fcf_margin", false}}

func median(a []float64) float64 {
	if len(a) == 0 {
		return nan
	}
	s := append([]float64(nil), a...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// compare dice si mine es mejor, parecido o peor que la mediana de los competidores (más de 10 % de diferencia).
func compare(mine, med float64, lowerBetter bool) Level {
	tol := math.Abs(med) * 0.1
	d := mine - med
	if lowerBetter {
		d = -d
	}
	switch {
	case d > tol:
		return Good
	case d >= -tol:
		return Warn
	}
	return Bad
}

// peerTable compara a c con los competidores peers.
func peerTable(c Company, peers []Company) *PeerTable {
	t := &PeerTable{}
	for _, p := range peers {
		t.Tickers = append(t.Tickers, p.Ticker)
		t.Names = append(t.Names, p.Name)
	}
	for _, pm := range peerMetrics {
		mine := valueOr(c.peerVals, pm.ID)
		row := PeerRow{ID: pm.ID, Mine: ptr(mine), Level: NA, Best: -2}
		var have []float64
		vals := make([]float64, len(peers))
		for i, p := range peers {
			vals[i] = valueOr(p.peerVals, pm.ID)
			row.Values = append(row.Values, ptr(vals[i]))
			if !isNaN(vals[i]) {
				have = append(have, vals[i])
			}
		}
		med := median(have)
		row.Median = ptr(med)
		if !isNaN(mine) && !isNaN(med) {
			row.Level = compare(mine, med, pm.LowerBetter)
		}
		// el mejor valor del grupo (en P/E solo cuenta el positivo)
		cand := append([]float64{mine}, vals...)
		best, n := -2, 0
		for i, v := range cand {
			if isNaN(v) || (pm.LowerBetter && v <= 0) {
				continue
			}
			n++
			if best == -2 || (pm.LowerBetter && v < cand[best+1]) || (!pm.LowerBetter && v > cand[best+1]) {
				best = i - 1
			}
		}
		if n > 1 {
			row.Best = best
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func valueOr(m map[string]float64, k string) float64 {
	if v, ok := m[k]; ok {
		return v
	}
	return nan
}

// AttachPeers agrega a cada empresa la comparación con hasta n competidores del mismo universo: los de su misma industria (o, si
// no hay, de su sector) con la capitalización más cercana.
func AttachPeers(cs []Company, n int) {
	for i := range cs {
		peers := pickPeers(cs, i, n)
		cat := Category{ID: "peers"}
		if len(peers) > 0 {
			cat.Peers = peerTable(cs[i], peers)
			ls := make([]Level, len(cat.Peers.Rows))
			for k, r := range cat.Peers.Rows {
				ls[k] = r.Level
			}
			cat.Verdict = Verdict(ls)
		} else {
			cat.Verdict = NA
		}
		cs[i].Categories = append(cs[i].Categories, cat)
		cs[i].Verdict = overall(cs[i].Categories)
	}
}

func pickPeers(cs []Company, self, n int) []Company {
	me := cs[self]
	var pool []Company
	for _, level := range []func(Company) bool{
		func(o Company) bool { return me.Industry != "" && o.Industry == me.Industry },
		func(o Company) bool { return me.Sector != "" && o.Sector == me.Sector },
	} {
		pool = pool[:0]
		for i, o := range cs {
			if i != self && o.Universe == me.Universe && level(o) {
				pool = append(pool, o)
			}
		}
		if len(pool) > 0 {
			break
		}
	}
	dist := func(o Company) float64 {
		if me.MarketCap == nil || o.MarketCap == nil || *me.MarketCap <= 0 || *o.MarketCap <= 0 {
			return math.MaxFloat64
		}
		return math.Abs(math.Log(*o.MarketCap / *me.MarketCap))
	}
	sort.SliceStable(pool, func(a, b int) bool { return dist(pool[a]) < dist(pool[b]) })
	if len(pool) > n {
		pool = pool[:n]
	}
	return append([]Company(nil), pool...)
}
