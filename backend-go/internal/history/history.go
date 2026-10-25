// Package history guarda en un JSON versionado el historial que no se puede regenerar (Top actual, suplentes y ranking de
// ETFs), para que sobreviva aunque se pierda la base SQLite, que es solo caché (p. ej. en GitHub Actions).
package history

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"top3/internal/config"
	"top3/internal/db"
	"top3/internal/logx"
)

const version = 1

type pickRow struct {
	RunAt          string   `json:"run_at"`
	AsOf           string   `json:"as_of"`
	Months         int      `json:"months"`
	MinCapBillions *float64 `json:"min_cap_billions"`
	Rank           int      `json:"rank"`
	Ticker         string   `json:"ticker"`
	ReturnLookback float64  `json:"return_lookback"`
	Price          float64  `json:"price"`
	MarketCap      *float64 `json:"market_cap"`
}

type etfRow struct {
	RunAt   string          `json:"run_at"`
	AsOf    string          `json:"as_of"`
	Rank    int             `json:"rank"`
	Ticker  string          `json:"ticker"`
	Score   float64         `json:"score"`
	Vol     float64         `json:"vol"`
	MaxDD   float64         `json:"max_dd"`
	SMADist float64         `json:"sma_dist"`
	Returns json.RawMessage `json:"returns"`
}

type file struct {
	Version    int       `json:"version"`
	Picks      []pickRow `json:"actual_picks"`
	Alternates []pickRow `json:"actual_alternates"`
	Etf        []etfRow  `json:"etf_scores"`
}

const pickSelect = "SELECT run_at, as_of, months, min_cap_billions, rank, ticker, return_lookback, price, market_cap FROM %s " +
	"ORDER BY as_of, run_at, COALESCE(min_cap_billions, 0), months, rank"

func readPicks(d *sql.DB, table string) ([]pickRow, error) {
	rows, err := d.Query(fmt.Sprintf(pickSelect, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pickRow
	for rows.Next() {
		var r pickRow
		var capB, mc sql.NullFloat64
		if err := rows.Scan(&r.RunAt, &r.AsOf, &r.Months, &capB, &r.Rank, &r.Ticker, &r.ReturnLookback, &r.Price, &mc); err != nil {
			return nil, err
		}
		if capB.Valid {
			r.MinCapBillions = &capB.Float64
		}
		if mc.Valid {
			r.MarketCap = &mc.Float64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func readEtf(d *sql.DB) ([]etfRow, error) {
	rows, err := d.Query("SELECT run_at, as_of, rank, ticker, score, vol, max_dd, sma_dist, returns FROM etf_scores ORDER BY as_of, run_at, rank")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []etfRow
	for rows.Next() {
		var r etfRow
		var returns string
		if err := rows.Scan(&r.RunAt, &r.AsOf, &r.Rank, &r.Ticker, &r.Score, &r.Vol, &r.MaxDD, &r.SMADist, &returns); err != nil {
			return nil, err
		}
		r.Returns = json.RawMessage(returns)
		out = append(out, r)
	}
	return out, rows.Err()
}

// encode escribe una fila por línea, para que los cambios en git se lean fila a fila.
func encode(f file) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{\n\"version\": %d", f.Version)
	section := func(name string, n int, row func(i int) any) error {
		fmt.Fprintf(&buf, ",\n%q: [", name)
		for i := range n {
			b, err := json.Marshal(row(i))
			if err != nil {
				return err
			}
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString("\n")
			buf.Write(b)
		}
		if n > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("]")
		return nil
	}
	if err := section("actual_picks", len(f.Picks), func(i int) any { return f.Picks[i] }); err != nil {
		return nil, err
	}
	if err := section("actual_alternates", len(f.Alternates), func(i int) any { return f.Alternates[i] }); err != nil {
		return nil, err
	}
	if err := section("etf_scores", len(f.Etf), func(i int) any { return f.Etf[i] }); err != nil {
		return nil, err
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

// Save escribe el historial de la base en [output] history_file. No toca el archivo si no cambió.
func Save(cfg config.Config, d *sql.DB) error {
	path := cfg.Output.HistoryFile
	if path == "" {
		return nil
	}
	f := file{Version: version}
	var err error
	if f.Picks, err = readPicks(d, "actual_picks"); err != nil {
		return err
	}
	if f.Alternates, err = readPicks(d, "actual_alternates"); err != nil {
		return err
	}
	if f.Etf, err = readEtf(d); err != nil {
		return err
	}
	out, err := encode(f)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, out) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	logx.Infof("Historial guardado: %s (%d selecciones, %d suplentes, %d filas de ETFs)", path, len(f.Picks), len(f.Alternates), len(f.Etf))
	return nil
}

func isEmpty(d *sql.DB) (bool, error) {
	var n int
	err := d.QueryRow("SELECT (SELECT COUNT(*) FROM actual_picks) + (SELECT COUNT(*) FROM actual_alternates) + (SELECT COUNT(*) FROM etf_scores)").Scan(&n)
	return n == 0, err
}

// Restore carga [output] history_file en la base si esta no tiene historial (base nueva o caché perdido). Si el archivo no
// existe no hace nada.
func Restore(cfg config.Config) error {
	path := cfg.Output.HistoryFile
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("historial %s: %w", path, err)
	}
	if f.Version != version {
		return fmt.Errorf("historial %s: versión %d no soportada", path, f.Version)
	}
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	if empty, err := isEmpty(d); err != nil || !empty {
		return err
	}
	err = db.Tx(d, func(tx *sql.Tx) error {
		for _, t := range []struct {
			table string
			rows  []pickRow
		}{{"actual_picks", f.Picks}, {"actual_alternates", f.Alternates}} {
			for _, r := range t.rows {
				if _, err := tx.Exec("INSERT INTO "+t.table+" (run_at, as_of, months, min_cap_billions, rank, ticker, return_lookback, price, market_cap) "+
					"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", r.RunAt, r.AsOf, r.Months, r.MinCapBillions, r.Rank, r.Ticker, r.ReturnLookback, r.Price, r.MarketCap); err != nil {
					return err
				}
			}
		}
		for _, r := range f.Etf {
			if _, err := tx.Exec("INSERT INTO etf_scores (run_at, as_of, rank, ticker, score, vol, max_dd, sma_dist, returns) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
				r.RunAt, r.AsOf, r.Rank, r.Ticker, r.Score, r.Vol, r.MaxDD, r.SMADist, string(r.Returns)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	logx.Infof("Historial restaurado desde %s (%d selecciones, %d suplentes, %d filas de ETFs)", path, len(f.Picks), len(f.Alternates), len(f.Etf))
	return nil
}
