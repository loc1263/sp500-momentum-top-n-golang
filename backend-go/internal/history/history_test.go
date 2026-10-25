package history

import (
	"os"
	"path/filepath"
	"testing"

	"top3/internal/config"
	"top3/internal/db"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.DBPath = filepath.Join(dir, "t.db")
	cfg.Output.HistoryFile = filepath.Join(dir, "sub", "history.json")
	cfg.Output.LogDir = dir
	return cfg
}

func seed(t *testing.T, cfg config.Config) {
	t.Helper()
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	stmts := []string{
		"INSERT INTO actual_picks VALUES ('2026-01-05T10:00:00','2026-01-05',4,NULL,1,'AAA',0.5,10,1e9)",
		"INSERT INTO actual_picks VALUES ('2026-01-05T10:00:00','2026-01-05',4,200,1,'BBB',0.4,20,NULL)",
		"INSERT INTO actual_alternates VALUES ('2026-01-05T10:00:00','2026-01-05',4,NULL,5,'CCC',0.3,30,2e9)",
		`INSERT INTO etf_scores VALUES ('2026-01-05T10:00:00','2026-01-05',1,'VTI',1.5,0.1,-0.05,0.03,'{"3":0.05,"12":0.2}')`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	if err := Save(cfg, d); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRestoreRoundTrip(t *testing.T) {
	cfg := testConfig(t)
	seed(t, cfg)
	first, err := os.ReadFile(cfg.Output.HistoryFile)
	if err != nil {
		t.Fatal(err)
	}

	// Base nueva (caché perdido): se restaura y al guardar de nuevo el archivo queda idéntico.
	cfg.Data.DBPath = filepath.Join(filepath.Dir(cfg.Data.DBPath), "nueva.db")
	if err := Restore(cfg); err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var picks, alts, etfs int
	d.QueryRow("SELECT COUNT(*) FROM actual_picks").Scan(&picks)
	d.QueryRow("SELECT COUNT(*) FROM actual_alternates").Scan(&alts)
	d.QueryRow("SELECT COUNT(*) FROM etf_scores").Scan(&etfs)
	if picks != 2 || alts != 1 || etfs != 1 {
		t.Fatalf("restaurado: %d selecciones, %d suplentes, %d ETFs", picks, alts, etfs)
	}
	if err := Save(cfg, d); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(cfg.Output.HistoryFile)
	if string(first) != string(second) {
		t.Fatalf("el archivo cambió tras restaurar y guardar:\n%s\n---\n%s", first, second)
	}
}

func TestRestoreKeepsExistingHistory(t *testing.T) {
	cfg := testConfig(t)
	seed(t, cfg)
	if err := Restore(cfg); err != nil { // la base ya tiene historial: no duplica
		t.Fatal(err)
	}
	d, _ := db.Open(cfg.Data.DBPath)
	defer d.Close()
	var n int
	d.QueryRow("SELECT COUNT(*) FROM actual_picks").Scan(&n)
	if n != 2 {
		t.Fatalf("filas = %d, quiero 2", n)
	}
}

func TestRestoreWithoutFile(t *testing.T) {
	cfg := testConfig(t)
	if err := Restore(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.Data.DBPath); err == nil {
		t.Fatal("no debería crear la base si no hay archivo de historial")
	}
}
