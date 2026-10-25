package repo

import (
	"database/sql"
	"encoding/json"

	"top3/internal/db"
)

// SaveLatestQuant guarda el resultado del modelo por score; solo se conserva el último.
func SaveLatestQuant(d *sql.DB, runAt, asOf string, payload json.RawMessage) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM quant_runs"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO quant_runs (run_at, as_of, payload) VALUES (?, ?, ?)", runAt, asOf, string(payload))
		return err
	})
}

// LatestQuant es el último resultado guardado del modelo por score (nil si no hay).
func LatestQuant(d *sql.DB) (json.RawMessage, error) {
	var payload string
	err := d.QueryRow("SELECT payload FROM quant_runs ORDER BY run_at DESC LIMIT 1").Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}
