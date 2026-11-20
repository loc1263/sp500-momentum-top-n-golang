// Package cleanup borra lo viejo para que el disco no se llene: corridas del backtest y datos de caché vencidos.
//
// Qué NO se borra, a propósito: actual_picks/actual_alternates y etf_scores (de ahí salen el calendario de rebalanceo, los
// cambios contra la corrida anterior y el seguimiento de cada selección; pesan muy poco) y los precios dentro de la ventana
// de [data] history_years (el backtest los necesita).
//
// Fecha: 2026-11-19
package cleanup

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"topwx/internal/config"
	"topwx/internal/db"
	"topwx/internal/logx"
	"topwx/internal/repo"
)

type Report struct {
	Rows []Rows
}

type Rows struct {
	Name string
	N    int64
}

func PruneStaleData(d *sql.DB, cfg config.Config, today time.Time) ([]Rows, error) {
	data := cfg.Data
	before := func(days int) string { return today.AddDate(0, 0, -days).Format("2006-01-02") }
	steps := []struct {
		name string
		run  func() (int64, error)
	}{
		{"precios fuera de la ventana", func() (int64, error) { return repo.DeletePricesBefore(d, data.HistoryStart(today)) }},
		{"acciones en circulación vencidas", func() (int64, error) { return repo.DeleteSharesBefore(d, before(data.SharesTTLDays)) }},
		{"datos financieros vencidos", func() (int64, error) { return repo.DeleteFundamentalsBefore(d, before(cfg.Quant.FundamentalsTTLDays)) }},
		{"tickers sin datos vencidos", func() (int64, error) { return repo.ForgetNoDataBefore(d, before(data.NoDataTTLDays)) }},
	}
	var out []Rows
	for _, s := range steps {
		n, err := s.run()
		if err != nil {
			return out, err
		}
		out = append(out, Rows{s.name, n})
	}
	return out, nil
}

func Run(cfg config.Config, keepDays int) (Report, error) {
	var rep Report
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return rep, err
	}
	defer d.Close()
	runs, err := repo.PruneRuns(d, firstNonZero(keepDays, cfg.Data.RunsKeepDays))
	if err != nil {
		return rep, err
	}
	rep.Rows = append(rep.Rows, Rows{"corridas del backtest", int64(runs)})
	stale, err := PruneStaleData(d, cfg, time.Now())
	rep.Rows = append(rep.Rows, stale...)
	if err != nil {
		return rep, err
	}
	for _, r := range rep.Rows {
		if r.N > 0 {
			if _, err := d.Exec("VACUUM"); err != nil {
				return rep, err
			}
			break
		}
	}
	parts := make([]string, len(rep.Rows))
	for i, r := range rep.Rows {
		parts[i] = fmt.Sprintf("%s: %d", r.Name, r.N)
	}
	logx.Infof("Limpieza: %s", strings.Join(parts, ", "))
	return rep, nil
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
