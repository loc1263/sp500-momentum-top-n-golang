// Páginas del Lab (backtest): una por vista (lab-<vista>.html) que Alpine pinta con los datos de lab-datos.js, ya armados por Go
// para cada universo y período. El universo y el período elegidos van en el hash (lab-backtest.html#200-10); los selectores solo
// lo cambian.
//
// Fecha: 2026-11-20

package site

import (
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"

	"topwx/internal/report"
)

var labViews = []struct{ ID, Label, Title string }{
	{"backtest", "Backtest", "Backtest"},
	{"sweep", "Plazos y sensibilidad", "Plazos y sensibilidad"},
	{"compare", "N-0 vs N-1", "N-0 vs N-1"},
}

var seriesColors = []string{"var(--s1)", "var(--s2)", "var(--s3)", "var(--s4)", "var(--s5)", "var(--s6)"}

// labKey identifica un universo y período dentro de lab-datos.js y en el hash de las páginas del Lab.
func labKey(cap string, years float64) string { return cap + "-" + fnum(years) }

func labPage(view string) string { return "lab-" + view + ".html" }

// labCtx es el universo y período de una página del Lab.
type labCtx struct {
	Key      string
	Universe report.Universe
	Period   report.Period
	Skip     *report.Period
	Settings report.Settings
}

type rangeView struct {
	Start, End, YearsText, First string
	Late                         bool
}

// newRange arma la leyenda «Período: desde a hasta (N años)» y avisa si el primer rebalanceo efectivo llegó tarde.
func newRange(start, end string, years float64, first string) rangeView {
	r := rangeView{Start: start, End: end, First: first}
	if years != 0 {
		unit := "años"
		if years == 1 {
			unit = "año"
		}
		r.YearsText = "(" + fnum(years) + " " + unit + ")"
	}
	if first != "" {
		a, okA := days(first)
		b, okB := days(start)
		r.Late = okA && okB && a-b > 62
	}
	return r
}

func periodRange(p report.Period, first string) rangeView {
	return newRange(p.StartDate, p.EndDate, p.Years, first)
}

func baseCalendar(t report.Term) report.Calendar {
	for _, c := range t.Calendars {
		if c.Offset == 0 {
			return c
		}
	}
	return t.Calendars[0]
}

func equityPoints(c report.Calendar) []chartPoint {
	pts := make([]chartPoint, len(c.Equity))
	for i, e := range c.Equity {
		pts[i] = chartPoint{Date: e.Date, Value: e.Capital}
	}
	return pts
}

// labBar son los selectores comunes de período y universo.
type labBar struct {
	Periods, Universes []option
}

func newLabBar(r *report.Report, ctx labCtx) labBar {
	var bar labBar
	for _, p := range ctx.Universe.Periods {
		bar.Periods = append(bar.Periods, option{
			Value: labKey(ctx.Key, p.Years), Selected: p.Years == ctx.Period.Years,
			Label: fmt.Sprintf("%s años (%s–%s)", fnum(p.Years), p.StartDate[:4], p.EndDate[:4]),
		})
	}
	for _, u := range r.Backtest {
		years := defaultYears(u, ctx.Period.Years, r.Settings)
		bar.Universes = append(bar.Universes, option{Value: labKey(capKeyOf(u.MinCapBillions), years), Label: u.Label, Selected: capKeyOf(u.MinCapBillions) == ctx.Key})
	}
	return bar
}

// defaultYears elige el período de un universo: el pedido si existe, si no el de la configuración y, si no, el último.
func defaultYears(u report.Universe, wanted float64, s report.Settings) float64 {
	for _, want := range []float64{wanted, s.BacktestYears} {
		for _, p := range u.Periods {
			if p.Years == want {
				return want
			}
		}
	}
	return u.Periods[len(u.Periods)-1].Years
}

// ---- Backtest ----

type btPage struct {
	Bar   labBar
	Terms []btTerm
}

type btTerm struct {
	Months                                                                                          int
	Selected                                                                                        bool
	Sub                                                                                             string
	Cap0, Cap1, First, Total, TotalClass, CAGR, CAGRClass, Median, MedianClass, Calendars, Min, Max string
	Range1, Range2, Range3                                                                          rangeView
	Chart                                                                                           template.HTML
	HistHeads                                                                                       []int
	History                                                                                         []trow
	HistCols                                                                                        int
	CalRows                                                                                         []trow
	CalCount                                                                                        int
}

func newBacktestPage(r *report.Report, ctx labCtx) btPage {
	pg := btPage{Bar: newLabBar(r, ctx)}
	wanted := ctx.Settings.Months
	chosen := ctx.Period.Terms[0].Months
	for _, t := range ctx.Period.Terms {
		if t.Months == wanted {
			chosen = wanted
		}
	}
	for _, t := range ctx.Period.Terms {
		pg.Terms = append(pg.Terms, newBtTerm(t, ctx, t.Months == chosen))
	}
	return pg
}

func newBtTerm(t report.Term, ctx labCtx, selected bool) btTerm {
	base := baseCalendar(t)
	s := t.Summary
	first := ""
	if base.FirstRebalance != nil {
		first = *base.FirstRebalance
	}
	v := btTerm{
		Months: t.Months, Selected: selected,
		Sub:  fmt.Sprintf("%s a %s (%s años) · ranking %d meses, rebalanceo cada %d meses · %s", ctx.Period.StartDate, ctx.Period.EndDate, fnum(ctx.Period.Years), t.Months, t.Months, ctx.Universe.Label),
		Cap0: uf(ctx.Settings.InitialCapital), Cap1: uf(base.FinalCapital), First: first,
		Total: pf(base.TotalReturn, 1), TotalClass: sg(base.TotalReturn), CAGR: pf(base.CAGR, 1), CAGRClass: sg(base.CAGR),
		Median: pf(s.CagrMedian, 1), MedianClass: sg(s.CagrMedian), Calendars: strconv.Itoa(s.Calendars), Min: pf(s.CagrMin, 0), Max: pf(s.CagrMax, 0),
		Range1: periodRange(ctx.Period, first), Range2: newRange(base.StartDate, base.EndDate, ctx.Period.Years, first), Range3: periodRange(ctx.Period, ""),
		CalCount: len(t.Calendars),
	}
	series := []chartSeries{}
	for _, c := range t.Calendars {
		if c.Offset != base.Offset {
			series = append(series, chartSeries{Name: fmt.Sprintf("Arranque +%d", c.Offset), Color: "var(--muted)", Width: 1, Opacity: 0.45, Points: equityPoints(c)})
		}
	}
	series = append(series, chartSeries{Name: "Calendario base", Color: "var(--accent)", Width: 2.5, Points: equityPoints(base)})
	v.Chart = chartHTML(series, "Capital (US$)")

	for _, h := range base.History {
		v.HistCols = max(v.HistCols, len(h.Picks))
	}
	for i := 1; i <= v.HistCols; i++ {
		v.HistHeads = append(v.HistHeads, i)
	}
	picked := map[string]bool{}
	for _, h := range base.History {
		picked[h.Date] = true
		cells := []cellView{{Text: h.Date}}
		for i := 0; i < v.HistCols; i++ {
			if i < len(h.Picks) {
				p := h.Picks[i]
				cells = append(cells, cellView{HTML: template.HTML(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer" title="Ver %s en stockanalysis.com"><strong>%s</strong></a> <span class="%s">%s</span>`,
					esc(tickerURL("acciones", p.Ticker)), esc(p.Ticker), esc(p.Ticker), sg(p.Return), pf(p.Return, 1)))})
			} else {
				cells = append(cells, cellView{})
			}
		}
		cells = append(cells, cellView{Class: "r", Text: usd(h.Capital)}, cellView{Class: numClass(h.Gain), Text: pctFrac(h.Gain, 1)})
		v.History = append(v.History, trow{Cells: cells})
	}
	if n := len(base.Equity); n > 0 && !picked[base.Equity[n-1].Date] {
		last := base.Equity[n-1]
		gain := last.Capital/ctx.Settings.InitialCapital - 1
		cells := []cellView{{Text: last.Date}}
		for i := 0; i < v.HistCols; i++ {
			cells = append(cells, cellView{})
		}
		cells = append(cells, cellView{Class: "r", Text: uf(last.Capital)}, cellView{Class: numClassF(last.Capital - ctx.Settings.InitialCapital), Text: pf(gain, 1)})
		v.History = append(v.History, trow{Cells: cells})
	}
	for _, c := range t.Calendars {
		fr := ""
		if c.FirstRebalance != nil {
			fr = *c.FirstRebalance
		}
		v.CalRows = append(v.CalRows, trow{Cells: []cellView{
			{Text: c.StartDate}, {Text: c.EndDate}, {Text: fr}, {Class: "r", Text: strconv.Itoa(c.Rebalances)}, {Class: "r", Text: uf(c.FinalCapital)},
			{Class: numClassF(c.TotalReturn), Text: pf(c.TotalReturn, 1)}, {Class: numClassF(c.CAGR), Text: pf(c.CAGR, 1)},
		}})
	}
	return v
}

// ---- Plazos y sensibilidad ----

type swPage struct {
	Bar        labBar
	HasBest    bool
	Steady     swBest
	TopBase    swBest
	SameWinner bool
	Capital    string
	Range1     rangeView
	Rows       []trow
	Note       string
	Bars       []swBar
	AxisLo     string
	AxisHi     string
	Chart      template.HTML
}

type swBest struct {
	Months                      int
	Median, MedianClass         string
	Calendars                   int
	CapMin, CapMax, BaseCapital string
}

type swBar struct {
	Months                 int
	Style, MedianLeft, Dot template.CSS
	MedianTitle, DotTitle  string
	Label                  string
}

type swRow struct {
	months int
	base   report.Calendar
	s      report.TermSummary
}

func newSweepPage(r *report.Report, ctx labCtx) swPage {
	pg := swPage{Bar: newLabBar(r, ctx), Capital: nf(ctx.Settings.InitialCapital)}
	var rows []swRow
	for _, t := range ctx.Period.Terms {
		rows = append(rows, swRow{months: t.Months, base: baseCalendar(t), s: t.Summary})
	}
	if len(rows) == 0 {
		return pg
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].s.CagrMedian > rows[j].s.CagrMedian })
	steady := rows[0]
	topBase := rows[0]
	for _, x := range rows {
		if x.base.FinalCapital > topBase.base.FinalCapital {
			topBase = x
		}
	}
	pg.HasBest = true
	pg.SameWinner = steady.months == topBase.months
	best := func(x swRow) swBest {
		return swBest{Months: x.months, Median: pf(x.s.CagrMedian, 1), MedianClass: sg(x.s.CagrMedian), Calendars: x.s.Calendars,
			CapMin: nf(x.s.CapitalMin), CapMax: nf(x.s.CapitalMax), BaseCapital: nf(x.base.FinalCapital)}
	}
	pg.Steady, pg.TopBase = best(steady), best(topBase)

	first := ""
	for _, x := range rows {
		if x.base.FirstRebalance != nil && *x.base.FirstRebalance > first {
			first = *x.base.FirstRebalance
		}
	}
	pg.Range1 = periodRange(ctx.Period, first)

	var counts []string
	for _, x := range rows {
		badges := ""
		if x.months == steady.months {
			badges += ` <span class="badge good">más consistente</span>`
		}
		if x.months == topBase.months && !pg.SameWinner {
			badges += ` <span class="badge">mayor retorno base</span>`
		}
		pg.Rows = append(pg.Rows, trow{Cells: []cellView{
			{HTML: template.HTML(fmt.Sprintf("<strong>%d meses</strong>%s", x.months, badges))},
			{Class: numClassF(x.s.CagrMedian) + " typical", Text: pf(x.s.CagrMedian, 1)}, {Class: "r typical", Text: nf(x.s.CapitalMedian)},
			{Class: numClassF(x.s.CagrMin), Text: pf(x.s.CagrMin, 1)}, {Class: "r", Text: nf(x.s.CapitalMin)},
			{Class: numClassF(x.s.CagrMax), Text: pf(x.s.CagrMax, 1)}, {Class: "r", Text: nf(x.s.CapitalMax)},
			{Class: numClassF(x.base.CAGR), Text: pf(x.base.CAGR, 1)}, {Class: "r", Text: nf(x.base.FinalCapital)},
		}})
		counts = append(counts, fmt.Sprintf("%dm: %d", x.months, x.s.Calendars))
	}
	pg.Note = "«Anual» = CAGR. «Capital final» = en qué se convirtieron US$ " + pg.Capital + ". Arranques probados: " + strings.Join(counts, " · ") + "."

	lo, hi := rows[0].s.CagrMin, rows[0].s.CagrMax
	for _, x := range rows {
		lo, hi = min(lo, x.s.CagrMin), max(hi, x.s.CagrMax)
	}
	span := hi - lo
	if span == 0 {
		span = 1
	}
	at := func(v float64) string { return strconv.FormatFloat((v-lo)/span*100, 'f', 2, 64) + "%" }
	for _, x := range rows {
		pg.Bars = append(pg.Bars, swBar{
			Months: x.months, Label: pf(x.s.CagrMin, 0) + " a " + pf(x.s.CagrMax, 0),
			Style:      template.CSS("left:" + at(x.s.CagrMin) + ";width:calc(" + at(x.s.CagrMax) + " - " + at(x.s.CagrMin) + ")"),
			MedianLeft: template.CSS("left:" + at(x.s.CagrMedian)), Dot: template.CSS("left:" + at(x.base.CAGR)),
			MedianTitle: "Típico " + pf(x.s.CagrMedian, 1), DotTitle: "Arranque base " + pf(x.base.CAGR, 1),
		})
	}
	pg.AxisLo, pg.AxisHi = pf(lo, 0), pf(hi, 0)

	var series []chartSeries
	for i, t := range ctx.Period.Terms {
		series = append(series, chartSeries{Name: fmt.Sprintf("%d meses", t.Months), Color: seriesColors[i%len(seriesColors)], Points: equityPoints(baseCalendar(t))})
	}
	pg.Chart = chartHTML(series, "Capital (US$)")
	return pg
}

// ---- N-0 vs N-1 ----

type cmpPage struct {
	Bar        labBar
	Empty      bool
	Headline   string
	AvgDiff    string
	AvgClass   string
	Range1     rangeView
	Rows       []trow
	Capital    string
	HasHalves  bool
	HalvesText string
	Split      string
	HalfRows   []trow
	Charts     []cmpChart
}

type cmpChart struct {
	Months   int
	Selected bool
	Chart    template.HTML
}

// termPair junta un plazo del modelo actual (N-0) con el mismo plazo del skip-month (N-1).
type termPair struct {
	months int
	a, b   report.Term
	diff   float64 // mediana anual de N-1 menos la de N-0
	winner string  // «N-0», «N-1» o "" si empatan
}

func pairTerms(ctx labCtx) []termPair {
	var pairs []termPair
	if ctx.Skip == nil {
		return nil
	}
	for _, t := range ctx.Period.Terms {
		for _, x := range ctx.Skip.Terms {
			if x.Months != t.Months {
				continue
			}
			d := x.Summary.CagrMedian - t.Summary.CagrMedian
			w := ""
			switch {
			case d > 0:
				w = "N-1"
			case d < 0:
				w = "N-0"
			}
			pairs = append(pairs, termPair{months: t.Months, a: t, b: x, diff: d, winner: w})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].months < pairs[j].months })
	return pairs
}

func newComparePage(r *report.Report, ctx labCtx) cmpPage {
	pg := cmpPage{Bar: newLabBar(r, ctx), Capital: nf(ctx.Settings.InitialCapital)}
	pairs := pairTerms(ctx)
	if len(pairs) == 0 {
		pg.Empty = true
		return pg
	}
	first := ""
	for _, x := range pairs {
		if fr := baseCalendar(x.a).FirstRebalance; fr != nil && *fr > first {
			first = *fr
		}
	}
	pg.Range1 = periodRange(ctx.Period, first)
	pg.fillSummary(pairs)
	pg.fillTerms(pairs)
	pg.fillHalves(pairs)
	pg.fillCharts(pairs, ctx.Settings.Months)
	return pg
}

// fillSummary arma el titular: qué modelo gana en más plazos y la diferencia media del resultado típico.
func (pg *cmpPage) fillSummary(pairs []termPair) {
	winsSkip, winsBase, sum := 0, 0, 0.0
	for _, x := range pairs {
		sum += x.diff
		switch x.winner {
		case "N-1":
			winsSkip++
		case "N-0":
			winsBase++
		}
	}
	switch {
	case winsSkip == winsBase:
		pg.Headline = fmt.Sprintf("Empate: cada modelo gana en %d de %d plazos.", winsBase, len(pairs))
	case winsSkip > winsBase:
		pg.Headline = fmt.Sprintf("El skip-month (N-1) gana en %d de %d plazos.", winsSkip, len(pairs))
	default:
		pg.Headline = fmt.Sprintf("El modelo actual (N-0) gana en %d de %d plazos.", winsBase, len(pairs))
	}
	avg := sum / float64(len(pairs))
	pg.AvgDiff, pg.AvgClass = pf(avg, 1), sg(avg)
}

func monthsCell(months int) cellView {
	return cellView{HTML: template.HTML(fmt.Sprintf("<strong>%d meses</strong>", months))}
}

func badgeCell(text string) cellView {
	return cellView{HTML: template.HTML(`<span class="badge good">` + text + `</span>`)}
}

// fillTerms arma la tabla plazo por plazo: resultado típico, peor y mejor arranque, capital final y ganador.
func (pg *cmpPage) fillTerms(pairs []termPair) {
	for _, x := range pairs {
		sa, sb := x.a.Summary, x.b.Summary
		winner := cellView{Text: "-"}
		if x.winner != "" {
			winner = badgeCell(x.winner)
		}
		pg.Rows = append(pg.Rows, trow{Cells: []cellView{
			monthsCell(x.months),
			{Class: numClassF(sa.CagrMedian) + " typical", Text: pf(sa.CagrMedian, 1)}, {Class: numClassF(sb.CagrMedian) + " typical", Text: pf(sb.CagrMedian, 1)},
			{Class: numClassF(x.diff), Text: pf(x.diff, 1)},
			{Class: "r", Text: pf(sa.CagrMin, 0) + " a " + pf(sa.CagrMax, 0)}, {Class: "r", Text: pf(sb.CagrMin, 0) + " a " + pf(sb.CagrMax, 0)},
			{Class: "r", Text: nf(sa.CapitalMedian)}, {Class: "r", Text: nf(sb.CapitalMedian)},
			winner,
		}})
	}
}

// halfWinner dice qué modelo rindió más en una mitad del período; "" si falta un dato o empatan.
func halfWinner(a, b *float64) string {
	if a == nil || b == nil || *a == *b {
		return ""
	}
	if *b > *a {
		return "N-1"
	}
	return "N-0"
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// fillHalves arma la tabla de las dos mitades del período y cuenta los plazos en que un modelo gana en ambas.
func (pg *cmpPage) fillHalves(pairs []termPair) {
	steadyBase, steadySkip := 0, 0
	for _, x := range pairs {
		sa, sb := x.a.Summary, x.b.Summary
		f := halfWinner(sa.Halves.First, sb.Halves.First)
		s := halfWinner(sa.Halves.Second, sb.Halves.Second)
		pg.HasHalves = pg.HasHalves || f != "" || s != ""
		steady := ""
		if f != "" && f == s {
			steady = f
		}
		switch steady {
		case "N-0":
			steadyBase++
		case "N-1":
			steadySkip++
		}
		if pg.Split == "" {
			pg.Split = sa.Halves.Split
		}
		held := cellView{HTML: `<span class="muted">no</span>`}
		if steady != "" {
			held = badgeCell(steady + " en las dos")
		}
		pg.HalfRows = append(pg.HalfRows, trow{Cells: []cellView{
			monthsCell(x.months),
			{Class: numClass(sa.Halves.First), Text: pctFrac(sa.Halves.First, 1)}, {Class: numClass(sb.Halves.First), Text: pctFrac(sb.Halves.First, 1)},
			{Text: dash(f)},
			{Class: numClass(sa.Halves.Second), Text: pctFrac(sa.Halves.Second, 1)}, {Class: numClass(sb.Halves.Second), Text: pctFrac(sb.Halves.Second, 1)},
			{Text: dash(s)},
			held,
		}})
	}
	pg.HalvesText = fmt.Sprintf("N-0 gana las dos mitades en %d de %d plazos, N-1 en %d; el resto se reparte.", steadyBase, len(pairs), steadySkip)
}

// fillCharts arma un gráfico por plazo (los dos modelos, arranque base); queda elegido el plazo de la configuración.
func (pg *cmpPage) fillCharts(pairs []termPair, wanted int) {
	chosen := pairs[0].months
	for _, x := range pairs {
		if x.months == wanted {
			chosen = x.months
		}
	}
	for _, x := range pairs {
		series := []chartSeries{
			{Name: fmt.Sprintf("%d-0 (actual)", x.months), Color: "var(--accent)", Width: 2.5, Points: equityPoints(baseCalendar(x.a))},
			{Name: fmt.Sprintf("%d-1 (skip-month)", x.months), Color: "var(--s2)", Width: 2.5, Points: equityPoints(baseCalendar(x.b))},
		}
		pg.Charts = append(pg.Charts, cmpChart{Months: x.months, Selected: x.months == chosen, Chart: chartHTML(series, "Capital (US$)")})
	}
}
