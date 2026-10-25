package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"top3/internal/config"
	"top3/internal/db"
	"top3/internal/repo"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestPruneOutputFilesCountsDaysFromTheNewestReport(t *testing.T) {
	dir := t.TempDir()
	names := []string{"backtest/top3_sweep_20260401.html", "backtest/top3_sweep_20260410.html", "actual/top3_actual_20260418.html",
		"actual/top3_actual_20260420.html", "etf/top3_etf_asof_2025-06-30_20260420.html", "etf/sin_fecha.html", "notas.txt"}
	for _, n := range names {
		touch(t, filepath.Join(dir, n))
	}
	doomed, err := PruneOutputFiles(dir, 3) // conserva 18, 19 y 20 de abril (desde el más reciente, no desde hoy)
	if err != nil {
		t.Fatal(err)
	}
	if len(doomed) != 2 {
		t.Fatalf("borrados: %v", doomed)
	}
	for _, gone := range names[:2] {
		if exists(filepath.Join(dir, gone)) {
			t.Errorf("%s debió borrarse", gone)
		}
	}
	for _, kept := range names[2:] {
		if !exists(filepath.Join(dir, kept)) {
			t.Errorf("%s debió conservarse", kept)
		}
	}
}

func TestPruneOutputFilesAlwaysKeepsTheLatestDayAndToleratesAMissingFolder(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a_20200101.html"))
	touch(t, filepath.Join(dir, "a_20200102.html"))
	if doomed, _ := PruneOutputFiles(dir, 0); len(doomed) != 1 || !exists(filepath.Join(dir, "a_20200102.html")) {
		t.Fatalf("borrados: %v", doomed)
	}
	if doomed, err := PruneOutputFiles(filepath.Join(dir, "no_existe"), 7); err != nil || doomed != nil {
		t.Fatalf("carpeta inexistente: %v %v", doomed, err)
	}
}

func TestPruneStaleDataDeletesOnlyWhatExpired(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cfg := config.Default() // history_years 12, shares_ttl 30, fundamentals_ttl 7, no_data_ttl 30
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
	if day := repo.LatestPriceDate(d); day != "2026-10-01" {
		t.Fatalf("quedó %s", day)
	}
}

func TestRunPrunesFilesRunsAndStaleData(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.DBPath = filepath.Join(dir, "t.db")
	cfg.Output.Dir = filepath.Join(dir, "out")
	touch(t, filepath.Join(cfg.Output.Dir, "x_20200101.html"))
	touch(t, filepath.Join(cfg.Output.Dir, "x_20200201.html"))
	rep, err := Run(cfg, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 1 || len(rep.Rows) != 5 || rep.Rows[0].Name != "corridas del backtest" {
		t.Fatalf("informe: %+v", rep)
	}
}
