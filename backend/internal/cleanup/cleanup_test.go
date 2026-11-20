// Pruebas de la limpieza: poda de corridas y de datos vencidos.
//
// Fecha: 2026-11-19

package cleanup

import (
	"path/filepath"
	"testing"
	"time"

	"topwx/internal/config"
	"topwx/internal/db"
	"topwx/internal/repo"
)

func TestPruneStaleDataDeletesOnlyWhatExpired(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cfg := config.Default()
	today := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	must := func(q string, args ...any) {
		if _, err := d.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	must("INSERT INTO prices VALUES ('A', '2010-01-04', 1), ('A', '2026-10-01', 2)")
	must("INSERT INTO shares VALUES ('A', 1, '2026-08-01'), ('B', 1, '2026-10-20')")
	must("INSERT INTO fundamentals VALUES ('A', '2026-10-01', '{}'), ('B', '2026-10-22', '{}')")
	must("INSERT INTO no_data_tickers VALUES ('X', '2026-09-01'), ('Y', '2026-10-20')")
	rows, err := PruneStaleData(d, cfg, today)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{1, 1, 1, 1} {
		if rows[i].N != want {
			t.Errorf("%s: %d filas, esperadas %d", rows[i].Name, rows[i].N, want)
		}
	}
	if day, _ := repo.LatestPriceDate(d); day != "2026-10-01" {
		t.Fatalf("quedó %s", day)
	}
}

func TestRunPrunesRunsAndStaleData(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.DBPath = filepath.Join(dir, "t.db")
	rep, err := Run(cfg, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Rows) != 5 || rep.Rows[0].Name != "corridas del backtest" {
		t.Fatalf("informe: %+v", rep)
	}
}
