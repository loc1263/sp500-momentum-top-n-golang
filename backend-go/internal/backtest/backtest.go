// Package backtest arma las corridas del backtest (por plazo y por calendario de arranque), las guarda en SQLite y las
// muestra por consola.
package backtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"top3/internal/config"
	"top3/internal/console"
	"top3/internal/dateutil"
	"top3/internal/engine"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/repo"
)

// SkipMonths: el modelo skip-month (N-1) ignora el último mes al armar el ranking.
const SkipMonths = 1

// Context es todo lo que comparten las corridas de un período: datos, fechas y configuración.
type Context struct {
	DB          *sql.DB
	DL          *market.Downloader
	Cfg         config.Config
	Cons        *market.Constituents
	Prices      *model.Prices
	Matrix      *engine.Matrix
	Start, End  time.Time
	UseCache    bool
	KnownShares map[string]float64 // acciones ya bajadas en esta corrida
	memo        *engine.Memo       // rankings por fecha; quien reparte un mismo Cons y Matrix entre contextos comparte uno
}

// rankMemo crea la memoria de rankings si no se la dieron: vale mientras Cons y Matrix no cambien.
func (c *Context) rankMemo() *engine.Memo {
	if c.memo == nil {
		c.memo = engine.NewMemo()
	}
	return c.memo
}

func (c *Context) lookbackDays(months int) int { return months * c.Cfg.Backtest.TradingDaysPerMonth }

// consoleRun son los datos de la corrida que muestra la consola.
func (c *Context) consoleRun() console.Run {
	return console.Run{Start: c.Start, End: c.End, Years: c.Cfg.Backtest.BacktestYears,
		InitialCapital: c.Cfg.Backtest.InitialCapital, TopN: c.Cfg.Backtest.TopN}
}

func minCap(u *market.Universe) float64 {
	if u == nil {
		return 0
	}
	return u.MinCapBillions
}

func (c *Context) where(u *market.Universe) string {
	return fmt.Sprintf("%s, backtest %s a %s", market.Label(minCap(u)), dateutil.Day(c.Start), dateutil.Day(c.End))
}

// runParams son los parámetros que se guardan con cada corrida: la configuración completa más lo que identifica la
// corrida. El informe los lee de ahí (ver repo.ParseRunKey).
func (c *Context) runParams(months, offset int, u *market.Universe, skipMonths int) (map[string]any, error) {
	raw, err := json.Marshal(c.Cfg)
	if err != nil {
		return nil, err
	}
	params := map[string]any{}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	params["sweep_months"] = months
	params["offset_months"] = offset
	params["min_cap_billions"] = nil
	if u != nil {
		params["min_cap_billions"] = u.MinCapBillions
	}
	params["skip_months"] = skipMonths
	return params, nil
}

// RunCalendars corre un plazo de N meses con sus N calendarios posibles (arranque desfasado 0..N-1 meses). Guarda cada
// corrida en SQLite; los calendarios que fallan se omiten. Los rebalanceos omitidos se avisan juntos al final (o, si quien
// llama pasa skipped, cuando él lo decida). Con skipMonths es el modelo skip-month.
func (c *Context) RunCalendars(months int, u *market.Universe, skipped engine.Skipped, skipMonths int) (model.Term, error) {
	own := skipped == nil
	if own {
		skipped = engine.Skipped{}
	}
	term := model.Term{Months: months}
	bt := c.Cfg.Backtest
	for offset := 0; offset < months; offset++ {
		start := dateutil.AddMonths(c.Start, offset)
		logx.Infof("=== Plazo %d-%d meses, desfase %d/%d (arranque desde %s) ===", months, skipMonths, offset, months-1, dateutil.Day(start))
		res, err := engine.Run(c.Cons, c.Matrix, start, c.End, engine.Params{
			LookbackDays: c.lookbackDays(months), StepMonths: months, InitialCapital: bt.InitialCapital, TopN: bt.TopN,
			Universe: u, Skipped: skipped, SkipDays: skipMonths * bt.TradingDaysPerMonth, Commission: bt.Commission,
			Memo: c.rankMemo(),
		})
		if err != nil {
			logx.Warnf(errs.BktTermFailed.Msg("Plazo %d meses, desfase %d sin resultado: %v"), months, offset, err)
			continue
		}
		params, err := c.runParams(months, offset, u, skipMonths)
		if err != nil {
			return term, err
		}
		if _, err := repo.SaveRun(c.DB, params, dateutil.Day(start), dateutil.Day(c.End), res); err != nil {
			return term, err
		}
		term.Calendars = append(term.Calendars, model.Calendar{Result: res, Offset: offset})
	}
	if own {
		engine.ReportSkipped(skipped, c.where(u))
	}
	return term, nil
}

// RunTerms corre cada plazo (por defecto los de [simulation] months) con todos sus calendarios.
func (c *Context) RunTerms(u *market.Universe, months []int, skipMonths int) ([]model.Term, error) {
	if months == nil {
		months = c.Cfg.Simulation.Months
	}
	skipped := engine.Skipped{} // todos los plazos y desfases juntos: un solo aviso por motivo
	terms := make([]model.Term, 0, len(months))
	for _, m := range months {
		t, err := c.RunCalendars(m, u, skipped, skipMonths)
		if err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	engine.ReportSkipped(skipped, c.where(u))
	return terms, nil
}

// AttachMarketCaps agrega la capitalización aprox. de cada selección del calendario base (solo informativa).
func (c *Context) AttachMarketCaps(ctx context.Context, terms []model.Term, u *market.Universe) error {
	var histories [][]model.Rebalance
	for i := range terms {
		if base := terms[i].Base(); base != nil {
			histories = append(histories, base.History)
		}
	}
	if len(histories) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, h := range histories {
		for _, r := range h {
			for _, p := range r.Picks {
				set[p.Ticker] = true
			}
		}
	}
	picked := make([]string, 0, len(set))
	for t := range set {
		picked = append(picked, t)
	}
	sort.Strings(picked)
	known := map[string]float64{}
	if u != nil {
		for t, v := range u.Shares {
			known[t] = v
		}
	}
	for t, v := range c.KnownShares {
		known[t] = v
	}
	shares, err := c.DL.EnsureShares(ctx, c.DB, picked, c.UseCache, c.Cfg.Data, known)
	if err != nil {
		return err
	}
	for t, v := range shares { // el período siguiente no vuelve a pedir lo mismo, tampoco con --no-cache
		c.KnownShares[t] = v
	}
	market.AddMarketCaps(histories, c.Prices, shares)
	return nil
}
