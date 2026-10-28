package repo

import (
	"database/sql"
	"encoding/json"
	"math"

	"top3/internal/db"
	"top3/internal/model"
)

// SaveLatestIpsa guarda el resultado del modelo por score del IPSA; solo se conserva el último.
func SaveLatestIpsa(d *sql.DB, runAt, asOf string, payload json.RawMessage) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM ipsa_runs"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO ipsa_runs (run_at, as_of, payload) VALUES (?, ?, ?)", runAt, asOf, string(payload))
		return err
	})
}

// LatestIpsa es el último resultado guardado del modelo por score del IPSA (nil si no hay).
func LatestIpsa(d *sql.DB) (json.RawMessage, error) {
	var payload string
	err := d.QueryRow("SELECT payload FROM ipsa_runs ORDER BY run_at DESC LIMIT 1").Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}

// OHLCVFramesFromRows arma las matrices fecha x ticker del OHLCV a partir de filas en memoria (sin pasar por SQLite).
func OHLCVFramesFromRows(rows []OHLCVRow) OHLCVFrames {
	var open, high, low, closeRows, vol []model.PriceRow
	for _, r := range rows {
		add := func(dst *[]model.PriceRow, x float64) {
			if !math.IsNaN(x) {
				*dst = append(*dst, model.PriceRow{Ticker: r.Ticker, Date: r.Date, Close: x})
			}
		}
		add(&open, r.Open)
		add(&high, r.High)
		add(&low, r.Low)
		add(&closeRows, r.Close)
		add(&vol, r.Volume)
	}
	f := OHLCVFrames{Close: model.FromRows(closeRows)}
	align := func(r []model.PriceRow) *model.Prices { return alignTo(model.FromRows(r), f.Close) }
	f.Open, f.High, f.Low, f.Volume = align(open), align(high), align(low), align(vol)
	return f
}
