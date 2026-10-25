package engine

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"top3/internal/dateutil"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/numfmt"
)

// MinTickers: con menos candidatos el ranking no es representativo y el rebalanceo se omite.
const MinTickers = 10

// Skipped son los rebalanceos omitidos, por motivo: fechas (se avisan juntos al final).
type Skipped map[errs.Code]map[string]bool

var skipText = map[string]string{
	errs.BktNoConstituents.ID: "Sin constituyentes",
	errs.BktFewTickers.ID:     "Pocos tickers disponibles",
	errs.BktShortWindow.ID:    "Ventana de precios insuficiente",
}

// skip anota un rebalanceo que se omite. Con skipped solo se anota la fecha (quien lo pasó avisa una vez con
// ReportSkipped); sin él (nil) se avisa en el momento.
func skip(code errs.Code, when time.Time, skipped Skipped, detail string) {
	if skipped == nil {
		logx.Warnf(code.Msg("%s en %s%s, se omite"), skipText[code.ID], dateutil.Day(when), detail)
		return
	}
	if skipped[code] == nil {
		skipped[code] = map[string]bool{}
	}
	skipped[code][dateutil.Day(when)] = true
	logx.Debugf("%s en %s%s, se omite", skipText[code.ID], dateutil.Day(when), detail)
}

// ReportSkipped da un solo aviso por motivo con el rango de fechas omitidas (en vez de uno por fecha y corrida).
func ReportSkipped(skipped Skipped, where string) {
	codes := make([]errs.Code, 0, len(skipped))
	for c := range skipped {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(a, b int) bool { return codes[a].ID < codes[b].ID })
	for _, code := range codes {
		days := make([]string, 0, len(skipped[code]))
		for d := range skipped[code] {
			days = append(days, d)
		}
		sort.Strings(days)
		logx.Warnf(code.Msg("%s en %d fechas de rebalanceo (de %s a %s), omitidas - %s"), skipText[code.ID],
			len(days), days[0], days[len(days)-1], where)
	}
}

// Members da los constituyentes del índice en una fecha (YYYY-MM-DD).
type Members interface {
	OnDate(day string) []string
}

// EligibleTickers son los constituyentes con precios y, si hay universo, con capitalización suficiente.
func EligibleTickers(now []string, m *Matrix, when time.Time, universe *market.Universe) []string {
	available := make([]string, 0, len(now))
	for _, t := range now {
		if m.Has(t) {
			available = append(available, t)
		}
	}
	logx.Debugf("%s: %d/%d constituyentes con precios", dateutil.Day(when), len(available), len(now))
	if universe != nil && len(available) > 0 {
		big := universe.Select(available, m.LastPrices(available, when))
		logx.Debugf("%s: filtro de capitalización (>= US$ %.0f B): %d -> %d tickers", dateutil.Day(when),
			universe.MinCapUSD()/1e9, len(available), len(big))
		return big
	}
	return available
}

// RankReturns es el retorno de cada ticker en lookback ruedas que terminan skipDays ruedas antes de when, de mayor a menor.
// ok = false si no hay ruedas suficientes.
func RankReturns(m *Matrix, tickers []string, when time.Time, lookback int, skipped Skipped, skipDays int) ([]Ranked, bool) {
	rows := m.RowsUntil(when)
	needed := lookback + skipDays + 1
	if rows < needed {
		skip(errs.BktShortWindow, when, skipped, fmt.Sprintf(" (solo %d filas de precios, <%d)", rows, needed))
		return nil, false
	}
	return m.Returns(tickers, when, lookback, skipDays)
}

// Params son los parámetros de una simulación.
type Params struct {
	LookbackDays   int
	StepMonths     int // rebalanceo cada N meses
	InitialCapital float64
	TopN           int
	Universe       *market.Universe
	Skipped        Skipped
	SkipDays       int     // el ranking ignora las últimas SkipDays ruedas (modelo skip-month)
	Commission     float64 // por lado (0.0005 = 0,05%)
	Memo           *Memo   // opcional: reutiliza los rankings entre corridas que comparten constituyentes y precios
}

// Memo guarda el resultado de armar el ranking de una fecha. Ese ranking depende solo de la fecha, el plazo, el skip y el
// universo (no del desfase ni de cada cuántos meses se rebalancea), así que las corridas de un barrido que caen en la
// misma fecha lo comparten. Vale mientras los constituyentes y la matriz de precios sean los mismos: lo crea quien los tiene.
// Es seguro entre goroutines. En un acierto no se repiten los DEBUG de candidatos de esa fecha.
type Memo struct {
	mu sync.Mutex
	m  map[memoKey]memoEntry
}

type memoKey struct {
	when           int64
	lookback, skip int
	minCap         float64 // capitalización mínima del universo (-1 = índice completo)
}

type memoEntry struct {
	skip   errs.Code // si no es el código vacío, el rebalanceo se omite por este motivo
	detail string
	ranked []Ranked
}

// NewMemo crea una memoria vacía.
func NewMemo() *Memo { return &Memo{m: map[memoKey]memoEntry{}} }

// step arma (o recupera) el ranking de una fecha. Si el rebalanceo se omite, lo anota en skipped igual que sin memoria.
func step(cons Members, m *Matrix, when time.Time, p Params) ([]Ranked, bool) {
	minCap := -1.0
	if p.Universe != nil {
		minCap = p.Universe.MinCapBillions
	}
	key := memoKey{when.UnixNano(), p.LookbackDays, p.SkipDays, minCap}
	if p.Memo != nil {
		p.Memo.mu.Lock()
		e, hit := p.Memo.m[key]
		p.Memo.mu.Unlock()
		if hit {
			return e.replay(when, p.Skipped)
		}
	}
	e := computeStep(cons, m, when, p)
	if p.Memo != nil {
		p.Memo.mu.Lock()
		p.Memo.m[key] = e
		p.Memo.mu.Unlock()
	}
	return e.replay(when, p.Skipped)
}

func (e memoEntry) replay(when time.Time, skipped Skipped) ([]Ranked, bool) {
	if e.skip != (errs.Code{}) {
		skip(e.skip, when, skipped, e.detail)
		return nil, false
	}
	return e.ranked, true
}

func computeStep(cons Members, m *Matrix, when time.Time, p Params) memoEntry {
	now := cons.OnDate(dateutil.Day(when))
	if len(now) == 0 {
		return memoEntry{skip: errs.BktNoConstituents}
	}
	candidates := EligibleTickers(now, m, when, p.Universe)
	if len(candidates) < MinTickers {
		return memoEntry{skip: errs.BktFewTickers}
	}
	rows := m.RowsUntil(when)
	if needed := p.LookbackDays + p.SkipDays + 1; rows < needed {
		return memoEntry{skip: errs.BktShortWindow, detail: fmt.Sprintf(" (solo %d filas de precios, <%d)", rows, needed)}
	}
	ranked, ok := m.Returns(candidates, when, p.LookbackDays, p.SkipDays)
	if !ok {
		return memoEntry{skip: errs.BktShortWindow}
	}
	return memoEntry{ranked: ranked}
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// Run simula la estrategia entre start y end, rebalanceando cada p.StepMonths meses (inicios de mes).
//
// Con SkipDays el ranking ignora las últimas SkipDays ruedas (modelo skip-month); las compras y el rendimiento siguen
// contándose desde la fecha de rebalanceo. Commission es la comisión por lado: se cobra al armar la cartera (compra de todo
// el capital) y en cada rebalanceo por las posiciones que cambian (se vende la que sale y se compra la que entra). No cobra
// el reequilibrio de pesos de las que se mantienen ni la venta final.
//
// Devuelve un error BKT-001 si no hubo ningún rebalanceo (fechas fuera del histórico, universo demasiado chico...).
func Run(cons Members, m *Matrix, start, end time.Time, p Params) (model.Result, error) {
	dates := dateutil.MonthStarts(start, end, p.StepMonths)
	first, last := "None", "None"
	if len(dates) > 0 {
		first, last = dateutil.Day(dates[0]), dateutil.Day(dates[len(dates)-1])
	}
	logx.Infof("Rebalanceos: %d (%s a %s, freq=%dMS)", len(dates), first, last, p.StepMonths)

	var (
		equity  []model.EquityPoint
		history []model.Rebalance
		capital = p.InitialCapital
		held    []string
		entry   time.Time
		started bool
	)
	for _, when := range dates {
		ranked, ok := step(cons, m, when, p)
		if !ok {
			continue
		}
		top := ranked[:min(p.TopN, len(ranked))]
		picks := make([]model.Pick, len(top))
		newHeld := make([]string, len(top))
		for i, r := range top {
			picks[i] = model.Pick{Ticker: r.Ticker, Return: round4(r.Return)}
			newHeld[i] = r.Ticker
		}
		history = append(history, model.Rebalance{Date: dateutil.Day(when), Picks: picks})

		if started { // rendimiento del período anterior
			if ret, withData, _, ok := m.HoldReturn(held, entry, when); ok {
				logx.Debugf("Período %s→%s: tickers=%v retorno=%.4f", dateutil.Day(entry), dateutil.Day(when), withData, ret)
				capital *= 1 + ret
			} else {
				logx.Warnf(errs.BktNoPrices.Msg("Período %s→%s: sin datos para %v"), dateutil.Day(entry), dateutil.Day(when), held)
			}
		}

		if !started {
			capital *= 1 - p.Commission // compra inicial de toda la cartera
		} else {
			changed := float64(countNew(newHeld, held)) / float64(len(newHeld))
			capital *= 1 - 2*p.Commission*changed // vende lo que sale y compra lo que entra
		}
		held, entry, started = newHeld, when, true
		equity = append(equity, model.EquityPoint{Date: when, Capital: capital})
		logx.Infof("%s → Top %d: %v | Capital: $%s", dateutil.Day(when), p.TopN, held, numfmt.Comma(capital, 0))
	}

	if started { // último tramo hasta el final
		if ret, withData, lastDay, ok := m.HoldReturn(held, entry, end); ok {
			logx.Debugf("Último tramo %s→%s: tickers=%v retorno=%.4f", dateutil.Day(entry), dateutil.Day(lastDay), withData, ret)
			capital *= 1 + ret
			equity = append(equity, model.EquityPoint{Date: lastDay, Capital: capital})
		}
	}
	if len(equity) == 0 {
		return model.Result{}, errs.BktEmpty.Errorf("El backtest no generó ningún rebalanceo; revisa fechas/histórico en la configuración.")
	}
	years := math.Max(float64(dateutil.DaysBetween(equity[0].Date, equity[len(equity)-1].Date))/365.25, 0.1)
	return model.Result{
		Equity:         equity,
		History:        history,
		FinalCapital:   capital,
		TotalReturnPct: (capital/p.InitialCapital - 1) * 100,
		CAGRPct:        (math.Pow(capital/p.InitialCapital, 1/years) - 1) * 100,
	}, nil
}

// countNew cuenta los tickers de now que no están en before.
func countNew(now, before []string) int {
	have := make(map[string]bool, len(before))
	for _, t := range before {
		have[t] = true
	}
	n := 0
	for _, t := range now {
		if !have[t] {
			n++
		}
	}
	return n
}
