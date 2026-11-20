// Páginas del evaluador de empresas: el listado (filtros, orden y veredicto por categoría) y una página por empresa
// (evaluador/<TICKER>.html) con sus métricas, minigráficos y comparación con pares.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"topwx/internal/evaluator"
)

type pillView struct{ Level, Text string }

type evaluatorList struct {
	Total      int
	Universes  string
	Date       string
	Categories int
	UniverseOp []option
	Sectors    []string
	Verdicts   []option
	Cols       []tcol
	CatCols    []catCol
	Rows       []trow
	ColSpan    int
}

type catCol struct{ Title, Why string }

func evalDate(generatedAt string) string {
	if len(generatedAt) >= 10 {
		return generatedAt[:10]
	}
	return generatedAt
}

func newEvaluatorList(p evaluator.Payload) evaluatorList {
	var parts []string
	var universeOpts []option
	for i, u := range p.Universes {
		parts = append(parts, fmt.Sprintf("%d de %s", u.Count, u.Label))
		universeOpts = append(universeOpts, option{Value: u.ID, Label: u.Label, Selected: i == 0})
	}
	l := evaluatorList{
		Total: len(p.Companies), Universes: strings.Join(parts, " · "), Date: evalDate(p.GeneratedAt), Categories: len(p.Sections), UniverseOp: universeOpts,
	}
	seen := map[string]bool{}
	for _, c := range p.Companies {
		if c.Sector != "" && !seen[c.Sector] {
			seen[c.Sector] = true
			l.Sectors = append(l.Sectors, c.Sector)
		}
	}
	sort.Slice(l.Sectors, func(i, j int) bool { return collateEs(l.Sectors[i], l.Sectors[j]) < 0 })
	for _, v := range []evaluator.Level{evaluator.Good, evaluator.Warn, evaluator.Bad, evaluator.NA} {
		l.Verdicts = append(l.Verdicts, option{Value: string(v), Label: verdictText(v)})
	}
	l.Cols = []tcol{
		{Label: "Ticker", Dir: "asc"}, {Label: "Empresa", Dir: "asc"}, {Label: "Sector", Dir: "asc"},
		{Label: "Capitalización", Num: true, Dir: "desc", Initial: "desc"}, {Label: "Veredicto", Num: true, Dir: "asc"},
	}
	l.Cols = newCols(l.Cols)
	for _, s := range p.Sections {
		l.CatCols = append(l.CatCols, catCol{Title: shortTitle(s.Title), Why: s.Why})
	}
	l.ColSpan = len(l.Cols) + len(l.CatCols)

	companies := append([]evaluator.Company(nil), p.Companies...)
	sort.SliceStable(companies, func(i, j int) bool { return capOrder(companies[i].MarketCap, companies[j].MarketCap) })
	for _, c := range companies {
		l.Rows = append(l.Rows, companyRow(c, p))
	}
	return l
}

// capOrder ordena de mayor a menor capitalización dejando los datos faltantes al final.
func capOrder(a, b *float64) bool {
	if a == nil || b == nil {
		return b == nil && a != nil
	}
	return *a > *b
}

func shortTitle(t string) string {
	return strings.NewReplacer(" (5 años)", "", "Vs. competidores", "Pares").Replace(t)
}

func companyRow(c evaluator.Company, p evaluator.Payload) trow {
	row := trow{
		Href:   "evaluador/" + companyFile(c.Ticker),
		Search: strings.ToLower(c.Ticker + " " + c.Name),
		Attrs:  map[string]string{"universe": c.Universe, "sector": c.Sector, "verdict": string(c.Verdict)},
	}
	row.Cells = []cellView{
		{Text: c.Ticker, Href: "evaluador/" + companyFile(c.Ticker), Strong: true},
		{Text: c.Name},
		{Class: "muted", Text: c.Sector},
		{Class: "r", Text: money(c.MarketCap, c.Currency), V: vOf(c.MarketCap)},
		{PillLevel: string(c.Verdict), Text: verdictText(c.Verdict), V: strconv.Itoa(levelOrder[c.Verdict])},
	}
	for _, s := range p.Sections {
		v := evaluator.NA
		for _, k := range c.Categories {
			if k.ID == s.ID {
				v = k.Verdict
			}
		}
		row.Cells = append(row.Cells, cellView{Class: "cat", Dot: string(v), Title: s.Title + ": " + verdictText(v), Text: verdictText(v)})
	}
	return row
}

var unsafeFile = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// companyFile es el nombre del archivo de una empresa: el ticker en mayúsculas, con lo raro reemplazado por «_».
func companyFile(ticker string) string {
	return unsafeFile.ReplaceAllString(strings.ToUpper(ticker), "_") + ".html"
}

// ---- Página de una empresa ----

type companyPage struct {
	Ticker, Name string
	TickerHref   string
	Verdict      pillView
	Facts        string
	Bank         bool
	Sections     []catView
}

type catView struct {
	ID, Title, Why string
	Wide           bool
	Pill           pillView
	Kind           string // «peers», «peers-none», «empty» o «metrics»
	Intro          string
	Peers          *peersView
	Metrics        []metricView
}

type metricView struct {
	Name, Why, Value string
	Pill             pillView
	Spark            template.HTML
	HasRange         bool
	Left, Lo, Hi     string
	Note             string
}

type peersView struct {
	Ticker string
	Heads  []peerHead
	Rows   []peerRowView
}

type peerHead struct{ Ticker, Name string }

type peerRowView struct {
	Name, Why, Mine string
	MineBest        bool
	Values          []peerCell
	Median          string
	Pill            pillView
}

type peerCell struct {
	Text string
	Best bool
}

func newCompanyPage(c evaluator.Company, p evaluator.Payload) companyPage {
	defs := map[string]evaluator.SectionInfo{}
	for _, s := range p.Sections {
		defs[s.ID] = s
	}
	fin := c.FinCurrency
	if fin == "" {
		fin = c.Currency
	}
	sector := c.Sector
	if sector == "" {
		sector = "Sin sector"
	}
	facts := sector
	if c.Industry != "" {
		facts += " · " + c.Industry
	}
	facts += " · Precio " + price(c.Price, c.Currency) + " · Capitalización " + money(c.MarketCap, c.Currency)
	if c.AsOf != "" {
		facts += " · Últimos resultados al " + c.AsOf
	}
	if c.FinCurrency != "" {
		facts += " · Estados financieros en " + c.FinCurrency
	}
	pg := companyPage{
		Ticker: c.Ticker, Name: c.Name, TickerHref: tickerURL("acciones", c.Ticker), Verdict: pillView{string(c.Verdict), verdictText(c.Verdict)},
		Facts: facts, Bank: c.Bank,
	}
	for _, cat := range c.Categories {
		def := defs[cat.ID]
		cv := catView{
			ID: cat.ID, Title: def.Title, Why: def.Why, Wide: cat.ID == "peers" || cat.ID == "trend",
			Pill: pillView{string(cat.Verdict), pillText(cat.Verdict, "", def.Labels)},
		}
		switch {
		case cat.ID == "peers" && cat.Peers != nil:
			cv.Kind, cv.Intro = "peers", def.Intro
			cv.Peers = newPeers(c.Ticker, cat.Peers, p)
		case cat.ID == "peers":
			cv.Kind = "peers-none"
		case len(cat.Metrics) == 0:
			cv.Kind = "empty"
		default:
			cv.Kind = "metrics"
			for _, m := range cat.Metrics {
				cv.Metrics = append(cv.Metrics, newMetric(m, cat, def, p, fin))
			}
		}
		pg.Sections = append(pg.Sections, cv)
	}
	return pg
}

func newMetric(m evaluator.Metric, cat evaluator.Category, def evaluator.SectionInfo, p evaluator.Payload, fin string) metricView {
	md := p.Metrics[m.ID]
	mv := metricView{
		Name: md.Name, Why: md.Why, Value: formatMetric(m, md, fin), Pill: pillView{string(m.Level), pillText(m.Level, m.Label, def.Labels)}, Note: m.Note,
	}
	if m.Series != nil {
		mv.Spark = template.HTML(sparkSVG(m.Series, cat.Years, md.Name))
	}
	if m.Low != nil && m.High != nil && m.Value != nil {
		left := *m.Value
		if left < 0 {
			left = 0
		}
		if left > 100 {
			left = 100
		}
		mv.HasRange, mv.Left, mv.Lo, mv.Hi = true, fnum(left), esNum(*m.Low, 2), esNum(*m.High, 2)
	}
	return mv
}

func newPeers(ticker string, t *evaluator.PeerTable, p evaluator.Payload) *peersView {
	pv := &peersView{Ticker: ticker}
	for i, tk := range t.Tickers {
		name := tk
		if i < len(t.Names) {
			name = t.Names[i]
		}
		pv.Heads = append(pv.Heads, peerHead{Ticker: tk, Name: name})
	}
	labels := map[evaluator.Level]string{evaluator.Good: "Mejor", evaluator.Warn: "Similar", evaluator.Bad: "Peor"}
	value := func(id string, v *float64) string {
		if v == nil {
			return "—"
		}
		return formatMetric(evaluator.Metric{ID: id, Value: v}, p.Metrics[id], "")
	}
	for _, r := range t.Rows {
		md := p.Metrics[r.ID]
		row := peerRowView{Name: md.Name, Why: md.Why, Mine: value(r.ID, r.Mine), MineBest: r.Best == -1, Median: value(r.ID, r.Median)}
		for i, v := range r.Values {
			row.Values = append(row.Values, peerCell{Text: value(r.ID, v), Best: r.Best == i})
		}
		row.Pill = pillView{string(r.Level), "—"}
		if r.Level != evaluator.NA {
			row.Pill.Text = labels[r.Level]
		}
		pv.Rows = append(pv.Rows, row)
	}
	return pv
}
