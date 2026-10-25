// Package db abre SQLite (un único archivo), aplica las migraciones y guarda los
// metadatos de caché.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // driver sin cgo

	"top3/internal/errs"
	"top3/internal/logx"
)

// Migration es un script SQL con su versión. La versión aplicada queda en PRAGMA user_version; para cambiar el esquema se
// agrega una migración al final (nunca se edita una ya publicada) y Open la aplica sola.
type Migration struct {
	Version int
	Script  string
}

// Migrations son las migraciones del esquema. Se aplican en orden y cada una sube PRAGMA user_version.
var Migrations = []Migration{
	{1, `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE IF NOT EXISTS constituents_history (date TEXT PRIMARY KEY, tickers TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS constituents_current (ticker TEXT PRIMARY KEY);
CREATE TABLE IF NOT EXISTS index_changes (
    date TEXT NOT NULL, added TEXT, removed TEXT, reason TEXT
);
CREATE TABLE IF NOT EXISTS prices (
    ticker TEXT NOT NULL, date TEXT NOT NULL, close REAL NOT NULL,
    PRIMARY KEY (ticker, date)
);
CREATE TABLE IF NOT EXISTS shares (ticker TEXT PRIMARY KEY, shares REAL, fetched TEXT);
CREATE TABLE IF NOT EXISTS no_data_tickers (ticker TEXT PRIMARY KEY, checked TEXT);
CREATE TABLE IF NOT EXISTS runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, run_at TEXT, params TEXT,
    start_date TEXT, end_date TEXT,
    final_capital REAL, total_return REAL, cagr REAL
);
CREATE TABLE IF NOT EXISTS selections (
    run_id INTEGER, rebalance_date TEXT, rank INTEGER, ticker TEXT, return_lookback REAL
);
CREATE TABLE IF NOT EXISTS equity (run_id INTEGER, date TEXT, capital REAL);
CREATE TABLE IF NOT EXISTS actual_picks (
    run_at TEXT, as_of TEXT, months INTEGER, min_cap_billions REAL,
    rank INTEGER, ticker TEXT, return_lookback REAL, price REAL, market_cap REAL
);
CREATE TABLE IF NOT EXISTS actual_alternates (
    run_at TEXT, as_of TEXT, months INTEGER, min_cap_billions REAL,
    rank INTEGER, ticker TEXT, return_lookback REAL, price REAL, market_cap REAL
);
CREATE TABLE IF NOT EXISTS etf_scores (
    run_at TEXT, as_of TEXT, rank INTEGER, ticker TEXT, score REAL, vol REAL, max_dd REAL, sma_dist REAL,
    returns TEXT
);
CREATE TABLE IF NOT EXISTS fundamentals (ticker TEXT PRIMARY KEY, fetched TEXT, data TEXT);
CREATE TABLE IF NOT EXISTS ohlcv (
    ticker TEXT NOT NULL, date TEXT NOT NULL, open REAL, high REAL, low REAL, close REAL NOT NULL, volume REAL,
    PRIMARY KEY (ticker, date)
);
CREATE TABLE IF NOT EXISTS quant_runs (run_at TEXT PRIMARY KEY, as_of TEXT, payload TEXT);
CREATE INDEX IF NOT EXISTS idx_selections_run ON selections (run_id);
CREATE INDEX IF NOT EXISTS idx_equity_run ON equity (run_id);
CREATE INDEX IF NOT EXISTS idx_runs_run_at ON runs (run_at);
`},
	{2, `
CREATE INDEX IF NOT EXISTS idx_actual_picks_key ON actual_picks (months, min_cap_billions, as_of);
CREATE INDEX IF NOT EXISTS idx_actual_alternates_run ON actual_alternates (run_at);
CREATE INDEX IF NOT EXISTS idx_etf_scores_as_of ON etf_scores (as_of);
CREATE INDEX IF NOT EXISTS idx_prices_date ON prices (date);
`},
}

// LatestVersion es la versión del esquema que conoce este código.
func LatestVersion() int { return Migrations[len(Migrations)-1].Version }

// SchemaVersion es la versión del esquema de la base (PRAGMA user_version).
func SchemaVersion(db *sql.DB) (int, error) {
	var v int
	err := db.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}

// Migrate lleva el esquema a la última versión aplicando, en orden, las migraciones que faltan.
func Migrate(db *sql.DB) error {
	current, err := SchemaVersion(db)
	if err != nil {
		return err
	}
	if current > LatestVersion() {
		return errs.DatDBNewer.Errorf("La base tiene el esquema v%d y este código llega a v%d.", current, LatestVersion())
	}
	for _, m := range Migrations {
		if m.Version <= current {
			continue
		}
		if _, err := db.Exec(m.Script); err != nil {
			return fmt.Errorf("migración v%d: %w", m.Version, err)
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil { // PRAGMA no admite parámetros
			return err
		}
		logx.Debugf("Esquema SQLite migrado a v%d", m.Version)
	}
	return nil
}

// pragmas son los ajustes de cada conexión. WAL con synchronous=NORMAL evita un fsync por transacción (un barrido del
// backtest guarda más de mil corridas): la base sigue consistente ante cualquier corte y solo un corte de luz puede
// perder las últimas transacciones. El resto es memoria: caché de 64 MB, temporales en RAM y archivo mapeado.
const pragmas = "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)" +
	"&_pragma=temp_store(MEMORY)&_pragma=cache_size(-65536)&_pragma=mmap_size(1073741824)&_pragma=wal_autocheckpoint(20000)"

// Open abre la base (creando carpeta y tablas si faltan) con el esquema al día.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+pragmas)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // una sola conexión: las escrituras no se pisan y las transacciones son explícitas
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	abs, _ := filepath.Abs(path)
	logx.Debugf("SQLite abierta: %s", abs)
	return db, nil
}

// Tx corre fn dentro de una transacción: la confirma si devuelve nil y la deshace si no.
func Tx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GetMeta lee un valor de la tabla meta ("" y false si no está).
func GetMeta(db *sql.DB, key string) (string, bool) {
	var v sql.NullString
	if err := db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v); err != nil || !v.Valid {
		return "", false
	}
	return v.String, true
}

// SetMeta guarda un valor en la tabla meta.
func SetMeta(db *sql.DB, key string, value any) error {
	_, err := db.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", key, fmt.Sprint(value))
	return err
}

// FetchedToday dice si el dato `key` se descargó hoy.
func FetchedToday(db *sql.DB, key string) bool {
	v, ok := GetMeta(db, key)
	return ok && v == Today()
}

// Today es hoy como YYYY-MM-DD.
func Today() string { return time.Now().Format("2006-01-02") }
