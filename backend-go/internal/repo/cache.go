// Repositorio de caché: acciones en circulación y fundamentales con su frescura y borrado por
// vencimiento.
//
// Fecha: 2026-11-19

package repo

import (
	"database/sql"

	"topwx/internal/db"
)

func FreshShares(d *sql.DB, cutoff string) (map[string]float64, error) {
	rows, err := d.Query("SELECT ticker, shares FROM shares WHERE fetched >= ?", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var t string
		var s sql.NullFloat64
		if err := rows.Scan(&t, &s); err != nil {
			return nil, err
		}
		if s.Valid {
			out[t] = s.Float64
		}
	}
	return out, rows.Err()
}

func SaveShares(d *sql.DB, shares map[string]float64, fetched string) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		for t, s := range shares {
			if _, err := tx.Exec("INSERT OR REPLACE INTO shares (ticker, shares, fetched) VALUES (?, ?, ?)", t, s, fetched); err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteSharesBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM shares WHERE fetched < ?", day))
}

func DeleteFundamentalsBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM fundamentals WHERE fetched < ?", day))
}

func FreshFundamentals(d *sql.DB, cutoff string) (map[string]string, error) {
	rows, err := d.Query("SELECT ticker, data FROM fundamentals WHERE fetched >= ?", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var t, data string
		if err := rows.Scan(&t, &data); err != nil {
			return nil, err
		}
		out[t] = data
	}
	return out, rows.Err()
}

func SaveFundamentals(d *sql.DB, data map[string]string, fetched string) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		for t, js := range data {
			if _, err := tx.Exec("INSERT OR REPLACE INTO fundamentals (ticker, fetched, data) VALUES (?, ?, ?)", t, fetched, js); err != nil {
				return err
			}
		}
		return nil
	})
}
