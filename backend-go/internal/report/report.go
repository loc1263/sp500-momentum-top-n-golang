// Package report arma el JSON que lee el frontend (Top actual + backtest + ETFs + modelo por score) con los últimos
// resultados guardados en SQLite.
package report

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"top3/internal/actual"
	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/history"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/repo"
)

// EquityPoint es el capital en una fecha.
type EquityPoint struct {
	Date    string  `json:"date"`
	Capital float64 `json:"capital"`
}

// HistoryPick es una acción elegida en un rebalanceo.
type HistoryPick struct {
	Rank   int     `json:"rank"`
	Ticker string  `json:"ticker"`
	Return float64 `json:"return"`
}

// HistoryEntry es un rebalanceo con el capital al entrar y la ganancia acumulada.
type HistoryEntry struct {
	Date    string        `json:"date"`
	Picks   []HistoryPick `json:"picks"`
	Capital *float64      `json:"capital"`
	Gain    *float64      `json:"gain"`
}

// Calendar es un calendario de arranque: resumen de la corrida, curva de capital e historial de selecciones.
type Calendar struct {
	Offset         int            `json:"offset"`
	StartDate      string         `json:"start_date"`
	EndDate        string         `json:"end_date"`
	FinalCapital   float64        `json:"final_capital"`
	TotalReturn    float64        `json:"total_return"`
	CAGR           float64        `json:"cagr"`
	Rebalances     int            `json:"rebalances"`
	FirstRebalance *string        `json:"first_rebalance"`
	Equity         []EquityPoint  `json:"equity"`
	History        []HistoryEntry `json:"history"`
}

// Halves es el CAGR mediano de cada mitad del período.
type Halves struct {
	Split  string   `json:"split"`
	First  *float64 `json:"first"`
	Second *float64 `json:"second"`
}

// TermSummary resume los calendarios de un plazo.
type TermSummary struct {
	Calendars     int     `json:"calendars"`
	CagrMedian    float64 `json:"cagr_median"`
	CagrMin       float64 `json:"cagr_min"`
	CagrMax       float64 `json:"cagr_max"`
	CapitalMedian float64 `json:"capital_median"`
	CapitalMin    float64 `json:"capital_min"`
	CapitalMax    float64 `json:"capital_max"`
	Halves        Halves  `json:"halves"`
}

// Term es un plazo con sus calendarios.
type Term struct {
	Months    int         `json:"months"`
	Calendars []Calendar  `json:"calendars"`
	Summary   TermSummary `json:"summary"`
}

// Period es un período del backtest.
type Period struct {
	Years     float64 `json:"years"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Terms     []Term  `json:"terms"`
}

// Universe es un universo del backtest con sus períodos.
type Universe struct {
	MinCapBillions *float64 `json:"min_cap_billions"`
	Label          string   `json:"label"`
	Periods        []Period `json:"periods"`
}

// TrackPick es cómo le fue a una acción de una selección guardada.
type TrackPick struct {
	Ticker string  `json:"ticker"`
	Price  float64 `json:"price"`
	Now    float64 `json:"now"`
	Return float64 `json:"return"`
}

// Tracking es cómo le fue a una selección guardada del Top actual.
type Tracking struct {
	AsOf   string      `json:"as_of"`
	Until  string      `json:"until"`
	Days   int         `json:"days"`
	Closed bool        `json:"closed"`
	Picks  []TrackPick `json:"picks"`
	Return float64     `json:"return"`
}

// ActualPick es una acción del Top actual (o suplente).
type ActualPick struct {
	Rank      int      `json:"rank"`
	Ticker    string   `json:"ticker"`
	Return    float64  `json:"return"`
	Price     float64  `json:"price"`
	MarketCap *float64 `json:"market_cap"`
}

// Changes son los cambios contra la corrida anterior.
type Changes struct {
	Entered []string `json:"entered"`
	Exited  []string `json:"exited"`
	Kept    []string `json:"kept"`
}

// Actual es el último Top actual de un universo y un plazo.
type Actual struct {
	MinCapBillions *float64     `json:"min_cap_billions"`
	Label          string       `json:"label"`
	Months         int          `json:"months"`
	AsOf           string       `json:"as_of"`
	RunAt          string       `json:"run_at"`
	PreviousAsOf   *string      `json:"previous_as_of"`
	Changes        *Changes     `json:"changes"`
	Anchor         *string      `json:"anchor"`
	NextRebalance  string       `json:"next_rebalance"`
	Picks          []ActualPick `json:"picks"`
	Alternates     []ActualPick `json:"alternates"`
	Tracking       []Tracking   `json:"tracking"`
}

// EtfRow es un ETF del último ranking.
type EtfRow struct {
	Rank     int                `json:"rank"`
	Ticker   string             `json:"ticker"`
	Name     string             `json:"name"`
	Category string             `json:"category"`
	Score    float64            `json:"score"`
	Vol      float64            `json:"vol"`
	MaxDD    float64            `json:"max_dd"`
	SMADist  float64            `json:"sma_dist"`
	Returns  map[string]float64 `json:"returns"`
}

// Etf es el último ranking de ETFs de Vanguard.
type Etf struct {
	AsOf         string   `json:"as_of"`
	RunAt        string   `json:"run_at"`
	TopN         int      `json:"top_n"`
	Horizons     []int    `json:"horizons"`
	RiskFree     *string  `json:"risk_free"`
	PreviousAsOf *string  `json:"previous_as_of"`
	Changes      *Changes `json:"changes"`
	Rows         []EtfRow `json:"rows"`
}

// Settings son los parámetros con los que se corrió el backtest.
type Settings struct {
	TopN           int       `json:"top_n"`
	Months         int       `json:"months"`
	InitialCapital float64   `json:"initial_capital"`
	Commission     float64   `json:"commission"`
	Terms          []int     `json:"terms"`
	BacktestYears  float64   `json:"backtest_years"`
	MinCapBillions float64   `json:"min_cap_billions"`
	PeriodsYears   []float64 `json:"periods_years"`
}

// Report es todo lo que lee el frontend.
type Report struct {
	GeneratedAt  string          `json:"generated_at"`
	RunDate      *string         `json:"run_date"`
	Settings     Settings        `json:"settings"`
	Actual       []Actual        `json:"actual"`
	Etf          *Etf            `json:"etf"`
	Quant        json.RawMessage `json:"quant"`
	Etfq         json.RawMessage `json:"etfq"`
	Backtest     []Universe      `json:"backtest"`
	BacktestSkip []Universe      `json:"backtest_skip"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func capPtr(cap float64) *float64 {
	if cap == 0 {
		return nil
	}
	return &cap
}

// latestRuns es la última corrida (por id) de cada clave entre las de la fecha más reciente.
func latestRuns(d *sql.DB) (string, map[repo.RunKey]repo.RunRow, error) {
	lastDay, err := repo.LatestRunDay(d)
	if err != nil || lastDay == "" {
		return "", nil, err
	}
	rows, err := repo.RunsOfDay(d, lastDay)
	if err != nil {
		return "", nil, err
	}
	latest := map[repo.RunKey]repo.RunRow{}
	for _, r := range rows {
		var params map[string]any
		if json.Unmarshal([]byte(r.Params), &params) != nil {
			continue
		}
		if key, ok := repo.ParseRunKey(params); ok {
			latest[key] = r
		}
	}
	return lastDay, latest, nil
}

// runDetail son la curva de capital y las selecciones (con el capital al entrar y la ganancia acumulada) de una corrida.
func runDetail(d *sql.DB, runID int64, initialCapital float64) ([]EquityPoint, []HistoryEntry, error) {
	curve, err := repo.EquityCurve(d, runID)
	if err != nil {
		return nil, nil, err
	}
	equity := make([]EquityPoint, len(curve))
	capByDate := make(map[string]float64, len(curve))
	for i, e := range curve {
		equity[i] = EquityPoint{e.Date, e.Capital}
		capByDate[e.Date] = e.Capital
	}
	sels, err := repo.Selections(d, runID)
	if err != nil {
		return nil, nil, err
	}
	var dates []string
	byDate := map[string][]HistoryPick{}
	for _, s := range sels {
		if _, ok := byDate[s.Date]; !ok {
			dates = append(dates, s.Date)
		}
		byDate[s.Date] = append(byDate[s.Date], HistoryPick{s.Rank, s.Ticker, s.Return})
	}
	sort.Strings(dates)
	history := make([]HistoryEntry, 0, len(dates))
	for _, day := range dates {
		h := HistoryEntry{Date: day, Picks: byDate[day]}
		if c, ok := capByDate[day]; ok {
			cc, g := c, c/initialCapital-1
			h.Capital, h.Gain = &cc, &g
		}
		history = append(history, h)
	}
	return equity, history, nil
}

// minHalfDays: una mitad más corta que esto no da un CAGR con sentido.
const minHalfDays = 90

// halves es el CAGR mediano de cada mitad del período (por fecha), para ver si el resultado se sostiene en el tiempo.
//
// Se parte la curva de capital de cada calendario en la fecha media del período; el corte se hace en el punto de la curva más
// cercano a esa fecha (hay uno por rebalanceo). Una mitad sin datos suficientes queda en nil.
func halves(calendars []Calendar) (Halves, error) {
	base := calendars[0]
	for _, c := range calendars {
		if c.Offset == 0 {
			base = c
			break
		}
	}
	start, err := dateutil.Parse(base.StartDate)
	if err != nil {
		return Halves{}, err
	}
	end, err := dateutil.Parse(base.EndDate)
	if err != nil {
		return Halves{}, err
	}
	mid := start.AddDate(0, 0, dateutil.DaysBetween(start, end)/2)
	var firsts, seconds []float64
	cagr := func(v0 float64, d0 time.Time, v1 float64, d1 time.Time) *float64 {
		days := dateutil.DaysBetween(d0, d1)
		if days < minHalfDays || !(v0 > 0) {
			return nil
		}
		v := math.Pow(v1/v0, 365.25/float64(days)) - 1
		return &v
	}
	for _, c := range calendars {
		if len(c.Equity) == 0 {
			continue
		}
		type point struct {
			d time.Time
			v float64
		}
		pts := make([]point, len(c.Equity))
		for i, e := range c.Equity {
			day, err := dateutil.Parse(e.Date)
			if err != nil {
				return Halves{}, err
			}
			pts[i] = point{day, e.Capital}
		}
		best := pts[0]
		bestDist := absInt(dateutil.DaysBetween(mid, best.d))
		for _, p := range pts[1:] {
			if dist := absInt(dateutil.DaysBetween(mid, p.d)); dist < bestDist {
				best, bestDist = p, dist
			}
		}
		if v := cagr(pts[0].v, pts[0].d, best.v, best.d); v != nil {
			firsts = append(firsts, *v)
		}
		if v := cagr(best.v, best.d, pts[len(pts)-1].v, pts[len(pts)-1].d); v != nil {
			seconds = append(seconds, *v)
		}
	}
	h := Halves{Split: dateutil.Day(mid)}
	if len(firsts) > 0 {
		m := model.Median(firsts)
		h.First = &m
	}
	if len(seconds) > 0 {
		m := model.Median(seconds)
		h.Second = &m
	}
	return h, nil
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func termSummary(calendars []Calendar) (TermSummary, error) {
	cagrs, finals := make([]float64, len(calendars)), make([]float64, len(calendars))
	for i, c := range calendars {
		cagrs[i], finals[i] = c.CAGR, c.FinalCapital
	}
	h, err := halves(calendars)
	if err != nil {
		return TermSummary{}, err
	}
	return TermSummary{
		Calendars: len(calendars), CagrMedian: model.Median(cagrs), CagrMin: minOf(cagrs), CagrMax: maxOf(cagrs),
		CapitalMedian: model.Median(finals), CapitalMin: minOf(finals), CapitalMax: maxOf(finals), Halves: h,
	}, nil
}

func minOf(x []float64) float64 {
	m := x[0]
	for _, v := range x {
		m = math.Min(m, v)
	}
	return m
}

func maxOf(x []float64) float64 {
	m := x[0]
	for _, v := range x {
		m = math.Max(m, v)
	}
	return m
}

// backtest agrupa universo -> período (años) -> plazo -> calendarios de arranque, de un modelo (skipMonths = 0 el de siempre,
// 1 el skip-month).
func backtest(d *sql.DB, latest map[repo.RunKey]repo.RunRow, initialCapital float64, skipMonths int) ([]Universe, error) {
	keys := make([]repo.RunKey, 0, len(latest))
	for k := range latest {
		if k.Skip == skipMonths {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a].Less(keys[b]) })

	type termKey struct {
		cap, years float64
		months     int
	}
	grouped := map[termKey][]Calendar{}
	var caps []float64
	yearsOf := map[float64][]float64{}
	monthsOf := map[[2]float64][]int{}
	for _, k := range keys {
		row := latest[k]
		equity, history, err := runDetail(d, row.ID, initialCapital)
		if err != nil {
			return nil, err
		}
		cal := Calendar{Offset: k.Offset, StartDate: row.StartDate, EndDate: row.EndDate, FinalCapital: row.FinalCapital,
			TotalReturn: row.TotalReturnPct / 100, CAGR: row.CAGRPct / 100, Rebalances: len(history), Equity: equity, History: history}
		if len(history) > 0 {
			cal.FirstRebalance = &history[0].Date
		}
		tk := termKey{k.Cap, k.Years, k.Months}
		if _, seen := grouped[tk]; !seen {
			if len(yearsOf[k.Cap]) == 0 {
				caps = append(caps, k.Cap)
			}
			if !containsF(yearsOf[k.Cap], k.Years) {
				yearsOf[k.Cap] = append(yearsOf[k.Cap], k.Years)
			}
			pk := [2]float64{k.Cap, k.Years}
			monthsOf[pk] = append(monthsOf[pk], k.Months)
		}
		grouped[tk] = append(grouped[tk], cal)
	}
	sort.Float64s(caps)
	out := make([]Universe, 0, len(caps))
	for _, cap := range caps {
		years := yearsOf[cap]
		sort.Float64s(years)
		u := Universe{MinCapBillions: capPtr(cap), Label: market.Label(cap)}
		for _, y := range years {
			months := monthsOf[[2]float64{cap, y}]
			sort.Ints(months)
			p := Period{Years: y}
			for _, m := range months {
				cals := grouped[termKey{cap, y, m}]
				sort.SliceStable(cals, func(a, b int) bool { return cals[a].Offset < cals[b].Offset })
				sum, err := termSummary(cals)
				if err != nil {
					return nil, err
				}
				p.Terms = append(p.Terms, Term{Months: m, Calendars: cals, Summary: sum})
			}
			base := p.Terms[0].Calendars[0] // calendario base (desfase 0): define el período
			p.StartDate, p.EndDate = base.StartDate, base.EndDate
			u.Periods = append(u.Periods, p)
		}
		out = append(out, u)
	}
	return out, nil
}

func containsF(list []float64, v float64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// termEnd es el fin de la tenencia: el mismo día del mes, months meses después (un 29-31 en un mes más corto cae en el 28).
func termEnd(bought time.Time, months int) time.Time {
	total := bought.Year()*12 + int(bought.Month()) - 1 + months
	y, m := total/12, total%12+1
	if bought.Day() > time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return dateutil.Date(y, time.Month(m), 28)
	}
	return dateutil.Date(y, time.Month(m), bought.Day())
}

// tracking dice cómo le fue a cada selección guardada del Top actual (de la más nueva a la más vieja).
//
// Se mide con el precio guardado al elegirla y el último precio disponible, sin pasar de los months meses que dura la
// tenencia: si ya pasaron, la selección figura como cerrada y su retorno es el del plazo completo.
func tracking(d *sql.DB, cap float64, months int) ([]Tracking, error) {
	lastStr := repo.LatestPriceDate(d)
	if lastStr == "" {
		return []Tracking{}, nil
	}
	lastDay, err := dateutil.Parse(lastStr)
	if err != nil {
		return nil, err
	}
	byDate, err := repo.LatestPicksByDate(d, cap, months)
	if err != nil {
		return nil, err
	}
	out := []Tracking{}
	for i := len(byDate) - 1; i >= 0; i-- {
		entry := byDate[i]
		bought, err := dateutil.Parse(entry.AsOf)
		if err != nil {
			return nil, err
		}
		end := termEnd(bought, months)
		closed := !end.After(lastDay)
		if end.After(lastDay) {
			end = lastDay
		}
		if !end.After(bought) {
			continue
		}
		tickers := make([]string, len(entry.Picks))
		for k, p := range entry.Picks {
			tickers[k] = p.Ticker
		}
		closes, err := repo.ClosesOnOrBefore(d, tickers, dateutil.Day(end))
		if err != nil {
			return nil, err
		}
		var picks []TrackPick
		sum := 0.0
		for _, p := range entry.Picks {
			now, ok := closes[p.Ticker]
			if !ok || p.Price == 0 {
				continue
			}
			r := now/p.Price - 1
			picks = append(picks, TrackPick{p.Ticker, p.Price, now, r})
			sum += r
		}
		if len(picks) > 0 {
			out = append(out, Tracking{AsOf: entry.AsOf, Until: dateutil.Day(end), Days: dateutil.DaysBetween(bought, end),
				Closed: closed, Picks: picks, Return: sum / float64(len(picks))})
		}
	}
	return out, nil
}

func changesOf(c actual.Changes) *Changes {
	return &Changes{Entered: c.Entered, Exited: c.Exited, Kept: c.Kept}
}

// buildActual es el último Top actual de cada (universo, plazo) entre los de la fecha más reciente.
func buildActual(d *sql.DB, today time.Time) ([]Actual, error) {
	lastRunAt, err := repo.LatestPickRunAt(d)
	if err != nil || lastRunAt == "" {
		return []Actual{}, err
	}
	rows, err := repo.PicksOfDay(d, lastRunAt[:10])
	if err != nil {
		return nil, err
	}
	type key struct {
		cap    float64
		months int
	}
	latest := map[key]string{} // (universo, plazo) -> run_at de la última corrida
	for _, r := range rows {
		latest[key{r.Cap, r.Months}] = r.RunAt
	}
	keys := make([]key, 0, len(latest))
	for k := range latest {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].cap != keys[b].cap {
			return keys[a].cap < keys[b].cap
		}
		return keys[a].months < keys[b].months
	})
	out := []Actual{}
	for _, k := range keys {
		runAt := latest[k]
		var mine []repo.PickRow
		for _, r := range rows {
			if r.RunAt == runAt && r.Cap == k.cap && r.Months == k.months {
				mine = append(mine, r)
			}
		}
		prevAsOf, prev, err := repo.PreviousPicks(d, mine[0].AsOf, k.months, k.cap)
		if err != nil {
			return nil, err
		}
		anchor, err := repo.AnchorDate(d, k.months, k.cap)
		if err != nil {
			return nil, err
		}
		alts, err := repo.Alternates(d, runAt, k.months, k.cap)
		if err != nil {
			return nil, err
		}
		track, err := tracking(d, k.cap, k.months)
		if err != nil {
			return nil, err
		}
		a := Actual{MinCapBillions: capPtr(k.cap), Label: market.Label(k.cap), Months: k.months, AsOf: mine[0].AsOf, RunAt: runAt,
			PreviousAsOf: strPtr(prevAsOf), Anchor: strPtr(anchor), Tracking: track, Picks: []ActualPick{}, Alternates: []ActualPick{}}
		tickers := make([]string, len(mine))
		for i, r := range mine {
			tickers[i] = r.Ticker
			a.Picks = append(a.Picks, ActualPick{r.Rank, r.Ticker, r.ReturnLookback, r.Price, r.MarketCap})
		}
		if prevAsOf != "" {
			a.Changes = changesOf(actual.DiffPicks(prev, tickers))
		}
		if anchor != "" {
			next, err := actual.NextRebalance(anchor, k.months, today)
			if err != nil {
				return nil, err
			}
			a.NextRebalance = dateutil.Day(next)
		}
		for _, p := range alts {
			a.Alternates = append(a.Alternates, ActualPick{p.Rank, p.Ticker, p.ReturnLookback, p.Price, p.MarketCap})
		}
		out = append(out, a)
	}
	return out, nil
}

// buildEtf es el último ranking de ETFs de Vanguard guardado, con nombre y categoría de la lista (CSV).
func buildEtf(d *sql.DB, cfg config.Config) (*Etf, error) {
	asOf := repo.LatestEtfAsOf(d)
	if asOf == "" {
		return nil, nil
	}
	e := cfg.Etf
	type meta struct{ name, category string }
	info := map[string]meta{}
	if f, err := os.Open(e.CSV); err == nil {
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		if recs, err := r.ReadAll(); err == nil && len(recs) > 0 {
			col := map[string]int{}
			for i, h := range recs[0] {
				col[strings.TrimPrefix(h, "\ufeff")] = i
			}
			get := func(rec []string, name string) string {
				if i, ok := col[name]; ok && i < len(rec) {
					return strings.TrimSpace(rec[i])
				}
				return ""
			}
			for _, rec := range recs[1:] {
				info[strings.ToUpper(get(rec, "ticker"))] = meta{get(rec, "name"), get(rec, "category")}
			}
		}
		f.Close()
	}
	ranking, err := repo.EtfRanking(d, asOf)
	if err != nil {
		return nil, err
	}
	if len(ranking) == 0 {
		return nil, nil
	}
	out := &Etf{AsOf: asOf, RunAt: ranking[len(ranking)-1].RunAt, TopN: e.TopN, Rows: []EtfRow{}}
	riskFree := strings.ToUpper(e.RiskFreeTicker)
	hasRF := false
	for _, r := range ranking {
		m := info[r.Ticker]
		out.Rows = append(out.Rows, EtfRow{Rank: r.Rank, Ticker: r.Ticker, Name: m.name, Category: m.category, Score: r.Score,
			Vol: r.Vol, MaxDD: r.MaxDD, SMADist: r.SMADist, Returns: r.Returns})
		hasRF = hasRF || r.Ticker == riskFree
	}
	for h := range ranking[0].Returns {
		var v int
		if _, err := fmt.Sscan(h, &v); err == nil {
			out.Horizons = append(out.Horizons, v)
		}
	}
	sort.Ints(out.Horizons)
	if hasRF && riskFree != "" {
		out.RiskFree = &riskFree
	}
	prevAsOf, prev, err := repo.PreviousEtfTop(d, asOf, e.TopN)
	if err != nil {
		return nil, err
	}
	out.PreviousAsOf = strPtr(prevAsOf)
	if prevAsOf != "" {
		top := []string{}
		for _, r := range out.Rows[:min(e.TopN, len(out.Rows))] {
			top = append(top, r.Ticker)
		}
		out.Changes = changesOf(actual.DiffPicks(prev, top))
	}
	return out, nil
}

// Build arma el reporte con lo último que hay en SQLite.
func Build(d *sql.DB, cfg config.Config, today time.Time) (*Report, error) {
	bt := cfg.Backtest
	runDay, latest, err := latestRuns(d)
	if err != nil {
		return nil, err
	}
	if bt.StartDate == "" { // solo los períodos configurados (ignora corridas sueltas de otros años)
		wanted := map[float64]bool{}
		for _, y := range bt.PeriodsYears {
			wanted[y] = true
		}
		for k := range latest {
			if !wanted[k.Years] {
				delete(latest, k)
			}
		}
	}
	bk, err := backtest(d, latest, bt.InitialCapital, 0)
	if err != nil {
		return nil, err
	}
	skip, err := backtest(d, latest, bt.InitialCapital, 1)
	if err != nil {
		return nil, err
	}
	var years []float64
	for _, u := range bk {
		for _, p := range u.Periods {
			if !containsF(years, p.Years) {
				years = append(years, p.Years)
			}
		}
	}
	sort.Float64s(years)
	act, err := buildActual(d, today)
	if err != nil {
		return nil, err
	}
	etf, err := buildEtf(d, cfg)
	if err != nil {
		return nil, err
	}
	quant, err := repo.LatestQuant(d)
	if err != nil {
		return nil, err
	}
	if quant == nil {
		quant = json.RawMessage("null")
	}
	etfq, err := repo.LatestEtfq(d)
	if err != nil {
		return nil, err
	}
	if etfq == nil {
		etfq = json.RawMessage("null")
	}
	return &Report{
		GeneratedAt: time.Now().Format("2006-01-02T15:04:05"), RunDate: strPtr(runDay),
		Settings: Settings{TopN: bt.TopN, Months: bt.Months, InitialCapital: bt.InitialCapital, Commission: bt.Commission,
			Terms: cfg.Simulation.Months, BacktestYears: bt.BacktestYears, MinCapBillions: cfg.Universe.MinCapBillions, PeriodsYears: years},
		Actual: act, Etf: etf, Quant: quant, Etfq: etfq, Backtest: nonNil(bk), BacktestSkip: nonNil(skip),
	}, nil
}

func nonNil(u []Universe) []Universe {
	if u == nil {
		return []Universe{}
	}
	return u
}

// Export escribe el JSON para el frontend ([output] frontend_data) y devuelve la ruta.
func Export(cfg config.Config) (string, error) {
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return "", err
	}
	defer d.Close()
	if err := history.Save(cfg, d); err != nil {
		return "", err
	}
	rep, err := Build(d, cfg, dateutil.Today())
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rep); err != nil {
		return "", err
	}
	path := cfg.Output.FrontendData
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, bytes.TrimRight(buf.Bytes(), "\n"), 0o644); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(path)
	logx.Infof("JSON del frontend guardado: %s (%d universos de backtest, %d Top actual)", abs, len(rep.Backtest), len(rep.Actual))
	return path, nil
}
