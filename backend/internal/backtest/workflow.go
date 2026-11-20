// Flujo de trabajo del backtest: trabajos simples, barridos y offsets, resolución de períodos y
// niveles, y poda posterior.
//
// Fecha: 2026-11-19

package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"topwx/internal/config"
	"topwx/internal/console"
	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/engine"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/market"
	"topwx/internal/model"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
)

type Job struct {
	Sweep          bool
	Offsets        bool
	Months         int
	Years          float64
	MinCapBillions *float64
	LargeCap       bool
	UseCache       bool
}

func (c *Context) RunSingle(ctx context.Context, months int, u *market.Universe) error {
	logx.Infof("Ejecutando backtest Point-in-Time (plazo %d meses)...", months)
	term, err := c.RunCalendars(months, u, nil, 0)
	if err != nil {
		return err
	}
	if term.Base() == nil {
		return errs.BktEmpty.Errorf("El backtest no generó ningún rebalanceo; revisa fechas/histórico en la configuración.")
	}
	if err := c.AttachMarketCaps(ctx, []model.Term{term}, u); err != nil {
		return err
	}
	console.Backtest(c.consoleRun(), term, minCap(u))
	return nil
}

func (c *Context) RunSweep(ctx context.Context, u *market.Universe) error {
	terms, err := c.RunTerms(u, nil, 0)
	if err != nil {
		return err
	}
	if err := c.AttachMarketCaps(ctx, terms, u); err != nil {
		return err
	}
	r, cap := c.consoleRun(), minCap(u)
	console.Sweep(r, terms, cap)
	console.Offsets(r, terms, cap)
	skipTerms, err := c.RunTerms(u, nil, SkipMonths)
	if err != nil {
		return err
	}
	console.Models(r, terms, skipTerms, cap)
	return nil
}

func (c *Context) RunOffsets(u *market.Universe) error {
	terms, err := c.RunTerms(u, nil, 0)
	if err != nil {
		return err
	}
	console.Offsets(c.consoleRun(), terms, minCap(u))
	return nil
}

type Period struct {
	Years float64
	Start time.Time
}

func ResolvePeriods(bt config.Backtest, end time.Time, years float64) ([]Period, error) {
	if bt.StartDate != "" {
		start, err := dateutil.Parse(bt.StartDate)
		if err != nil {
			return nil, fmt.Errorf("[backtest] start_date: %w", err)
		}
		y := math.Round(dateutil.Years(start, end)*10) / 10
		return []Period{{y, start}}, nil
	}
	list := bt.PeriodsYears
	if years != 0 {
		list = []float64{years}
	}
	list = append([]float64(nil), list...)
	sort.Float64s(list)
	out := make([]Period, len(list))
	for i, y := range list {
		out[i] = Period{y, dateutil.AddYears(end, -int(y))}
	}
	return out, nil
}

func ResolveLevels(job Job, cfg config.Config) []float64 {
	if cap, ok := market.ExplicitMinCap(cfg, job.LargeCap, job.MinCapBillions); ok {
		return []float64{cap}
	}
	if job.Sweep {
		return append([]float64(nil), cfg.Universe.SweepMinCapBillions...)
	}
	return []float64{0}
}

func RunJob(ctx context.Context, cfg config.Config, y *yahoo.Client, job Job) error {
	if job.UseCache {
		logx.Infof("Caché: activada (se reutiliza lo descargado hoy)")
	} else {
		logx.Infof("Caché: DESACTIVADA (--no-cache)")
	}
	bt := cfg.Backtest
	today := dateutil.Today()
	historyStart := cfg.Data.HistoryStart(today)

	end := today
	if bt.EndDate != "" {
		var err error
		if end, err = dateutil.Parse(bt.EndDate); err != nil {
			return fmt.Errorf("[backtest] end_date: %w", err)
		}
	}
	periods, err := ResolvePeriods(bt, end, job.Years)
	if err != nil {
		return err
	}
	for _, p := range periods {
		if !p.Start.Before(end) {
			return errs.CfgBadDates.Errorf("start_date (%s) debe ser anterior a end_date (%s)", dateutil.Day(p.Start), dateutil.Day(end))
		}
		logx.Infof("Histórico: %s a %s | Backtest de %g años: %s a %s", historyStart, dateutil.Day(today), p.Years,
			dateutil.Day(p.Start), dateutil.Day(end))
		if dateutil.Day(p.Start) < historyStart {
			logx.Warnf(errs.CfgShortHistory.Msg("El inicio del backtest de %g años (%s) es anterior al histórico descargado (%s); subí [data] history_years"),
				p.Years, dateutil.Day(p.Start), historyStart)
		}
	}

	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	md, err := dl.Refresh(ctx, d, cfg, job.UseCache, today)
	if err != nil {
		return err
	}
	matrix := engine.NewMatrix(md.Prices)

	known := map[string]float64{}
	builder := &market.UniverseBuilder{D: d, DL: dl, Tickers: md.Prices.Tickers, UseCache: job.UseCache, Settings: cfg.Data, KnownShares: known}
	levels := ResolveLevels(job, cfg)
	months := job.Months
	if months == 0 {
		months = bt.Months
	}

	memo := engine.NewMemo()
	for _, p := range periods {
		pcfg := cfg
		pcfg.Backtest.BacktestYears = p.Years
		c := &Context{DB: d, DL: dl, Cfg: pcfg, Cons: md.Constituents, Prices: md.Prices, Matrix: matrix,
			Start: p.Start, End: end, UseCache: job.UseCache, KnownShares: known, memo: memo}
		logx.Infof("########## Período: %g años (%s a %s) ##########", p.Years, dateutil.Day(p.Start), dateutil.Day(end))
		for _, level := range levels {
			u, err := builder.Build(ctx, level)
			if err != nil {
				return err
			}
			switch {
			case job.Sweep:
				logx.Infof("##### Sweep - universo: %s #####", market.Label(level))
				err = c.RunSweep(ctx, u)
			case job.Offsets:
				err = c.RunOffsets(u)
			default:
				err = c.RunSingle(ctx, months, u)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func PruneAfter(cfg config.Config) error {
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = repo.MaybePruneRuns(d, cfg.Data.RunsKeepDays, false)
	return err
}
