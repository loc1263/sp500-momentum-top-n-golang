package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func open(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "sub", "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenCreatesParentAndSchema(t *testing.T) {
	d := open(t)
	rows, err := d.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var n string
		rows.Scan(&n)
		names[n] = true
	}
	for _, want := range []string{"meta", "prices", "constituents_history", "runs", "equity", "actual_picks", "quant_runs", "ohlcv"} {
		if !names[want] {
			t.Errorf("falta la tabla %s", want)
		}
	}
}

func TestMetaAndFetchedToday(t *testing.T) {
	d := open(t)
	if _, ok := GetMeta(d, "k"); ok {
		t.Fatal("no debería existir")
	}
	SetMeta(d, "k", Today())
	if !FetchedToday(d, "k") {
		t.Fatal("se guardó hoy")
	}
	SetMeta(d, "k", time.Now().AddDate(0, 0, -1).Format("2006-01-02"))
	if FetchedToday(d, "k") {
		t.Fatal("se guardó ayer")
	}
}

func TestOpenStampsTheLatestSchemaVersion(t *testing.T) {
	d := open(t)
	if v, _ := SchemaVersion(d); v != LatestVersion() {
		t.Fatalf("versión %d", v)
	}
}

func TestALegacyDatabaseWithoutVersionIsMigratedInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacy.SetMaxOpenConns(1)
	if _, err := legacy.Exec(Migrations[0].Script); err != nil { // el esquema de antes de existir las versiones (user_version 0)
		t.Fatal(err)
	}
	legacy.Exec("INSERT INTO prices VALUES ('A', '2024-01-02', 1.5)")
	legacy.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if v, _ := SchemaVersion(d); v != LatestVersion() {
		t.Fatalf("versión %d", v)
	}
	var c float64
	d.QueryRow("SELECT close FROM prices").Scan(&c)
	if c != 1.5 { // los datos se conservan
		t.Fatalf("close = %v", c)
	}
	var n int
	d.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_actual_picks_key'").Scan(&n)
	if n != 1 {
		t.Fatal("falta el índice de la migración 2")
	}
}

func TestADatabaseFromANewerVersionIsNotTouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newer.db")
	newer, _ := sql.Open("sqlite", path)
	newer.SetMaxOpenConns(1)
	if _, err := newer.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	newer.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "TOP-DAT-007") {
		t.Fatalf("err = %v", err)
	}
}

func TestTxRollsBackOnError(t *testing.T) {
	d := open(t)
	err := Tx(d, func(tx *sql.Tx) error {
		tx.Exec("INSERT INTO meta VALUES ('a', '1')")
		return sql.ErrNoRows
	})
	if err != sql.ErrNoRows {
		t.Fatalf("err = %v", err)
	}
	if _, ok := GetMeta(d, "a"); ok {
		t.Fatal("la transacción debió deshacerse")
	}
	if err := Tx(d, func(tx *sql.Tx) error { _, err := tx.Exec("INSERT INTO meta VALUES ('b', '2')"); return err }); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetMeta(d, "b"); v != "2" {
		t.Fatal("la transacción debió confirmarse")
	}
}
