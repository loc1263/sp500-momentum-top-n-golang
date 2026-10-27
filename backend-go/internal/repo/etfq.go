package repo

import (
	"database/sql"
	"encoding/json"
	"time"

	"top3/internal/db"
)

// SaveLatestEtfq guarda el resultado de los ETFs por gestora; solo se conserva el último.
func SaveLatestEtfq(d *sql.DB, runAt, asOf string, payload json.RawMessage) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM etfq_runs"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO etfq_runs (run_at, as_of, payload) VALUES (?, ?, ?)", runAt, asOf, string(payload))
		return err
	})
}

// LatestEtfq es el último resultado guardado de los ETFs por gestora (nil si no hay).
func LatestEtfq(d *sql.DB) (json.RawMessage, error) {
	var payload string
	err := d.QueryRow("SELECT payload FROM etfq_runs ORDER BY run_at DESC LIMIT 1").Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}

// EtfqCategories son las categorías guardadas de cada ETF y el día (YYYY-MM-DD) en que se bajaron.
func EtfqCategories(d *sql.DB) (map[string][2]string, error) {
	rows, err := d.Query("SELECT ticker, category, fetched FROM etfq_category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]string{}
	for rows.Next() {
		var ticker, category, fetched string
		if err := rows.Scan(&ticker, &category, &fetched); err != nil {
			return nil, err
		}
		out[ticker] = [2]string{category, fetched}
	}
	return out, rows.Err()
}

// SaveEtfqCategories guarda (o actualiza) la categoría de cada ETF con la fecha de hoy.
func SaveEtfqCategories(d *sql.DB, categories map[string]string) error {
	today := time.Now().Format("2006-01-02")
	return db.Tx(d, func(tx *sql.Tx) error {
		for ticker, category := range categories {
			if _, err := tx.Exec("INSERT INTO etfq_category (ticker, category, fetched) VALUES (?, ?, ?) "+
				"ON CONFLICT(ticker) DO UPDATE SET category = excluded.category, fetched = excluded.fetched", ticker, category, today); err != nil {
				return err
			}
		}
		return nil
	})
}
