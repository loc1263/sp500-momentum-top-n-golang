// Package cleanup borra lo viejo para que el disco no se llene: reportes HTML, corridas del backtest y datos de caché vencidos.
//
// Qué NO se borra, a propósito: actual_picks/actual_alternates y etf_scores (de ahí salen el calendario de rebalanceo, los
// cambios contra la corrida anterior y el seguimiento de cada selección; pesan muy poco) y los precios dentro de la ventana
// de [data] history_years (el backtest los necesita).
package cleanup

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"top3/internal/config"
	"top3/internal/db"
	"top3/internal/logx"
	"top3/internal/repo"
)

// Los reportes terminan en _YYYYMMDD.html.
var fileDay = regexp.MustCompile(`_(\d{8})\.html$`)

// Report es lo que se borró.
type Report struct {
	Files []string
	// Rows son las filas borradas por concepto, en el orden en que se informan.
	Rows []Rows
}

// Rows es la cantidad de filas borradas de un concepto.
type Rows struct {
	Name string
	N    int64
}

// PruneOutputFiles borra los reportes HTML de dir (y sus subcarpetas) de hace más de keepDays días. Los días se cuentan desde
// el reporte más reciente (no desde hoy) y siempre se conserva al menos ese último día, para que dejar de correr el pipeline
// una semana no borre todos los reportes.
func PruneOutputFiles(dir string, keepDays int) ([]string, error) {
	type dated struct {
		day  time.Time
		path string
	}
	var files []dated
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			return nil
		}
		if m := fileDay.FindStringSubmatch(e.Name()); m != nil {
			if day, err := time.Parse("20060102", m[1]); err == nil {
				files = append(files, dated{day, path})
			}
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		return nil, err
	}
	newest := files[0].day
	for _, f := range files {
		if f.day.After(newest) {
			newest = f.day
		}
	}
	cutoff := newest.AddDate(0, 0, -(max(keepDays, 1) - 1))
	var doomed []string
	for _, f := range files {
		if f.day.Before(cutoff) {
			doomed = append(doomed, f.path)
		}
	}
	sort.Strings(doomed)
	for _, p := range doomed {
		if err := os.Remove(p); err != nil {
			return doomed, err
		}
	}
	return doomed, nil
}

// PruneStaleData borra de SQLite lo que ya no se va a usar: precios anteriores a la ventana del histórico y datos de caché
// (acciones en circulación, datos financieros, tickers sin datos) más viejos que su propia vigencia.
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

// Run es la limpieza completa: reportes HTML, corridas viejas del backtest y datos vencidos; luego compacta la base.
// keepDays = 0 usa los de la configuración.
func Run(cfg config.Config, keepDays int) (Report, error) {
	var rep Report
	files, err := PruneOutputFiles(cfg.Output.Dir, firstNonZero(keepDays, cfg.Output.KeepDays))
	rep.Files = files
	if err != nil {
		return rep, err
	}
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
			if _, err := d.Exec("VACUUM"); err != nil { // devuelve el espacio al disco; fuera de transacción
				return rep, err
			}
			break
		}
	}
	parts := make([]string, len(rep.Rows))
	for i, r := range rep.Rows {
		parts[i] = fmt.Sprintf("%s: %d", r.Name, r.N)
	}
	logx.Infof("Limpieza: %d reportes HTML borrados; %s", len(rep.Files), strings.Join(parts, ", "))
	return rep, nil
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
