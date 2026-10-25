package repo

import (
	"database/sql"

	"top3/internal/db"
)

// HistoryRow es una fila del CSV histórico: fecha y tickers separados por coma.
type HistoryRow struct{ Date, Tickers string }

// IndexChange es un alta/baja del índice según Wikipedia. Added, Removed y Reason son nulos si la celda venía vacía.
type IndexChange struct {
	Date                   string
	Added, Removed, Reason *string
}

// ReplaceHistory reemplaza la composición histórica.
func ReplaceHistory(d *sql.DB, rows []HistoryRow) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM constituents_history"); err != nil {
			return err
		}
		for _, r := range rows {
			if _, err := tx.Exec("INSERT OR REPLACE INTO constituents_history (date, tickers) VALUES (?, ?)", r.Date, r.Tickers); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceCurrent reemplaza la lista actual del índice.
func ReplaceCurrent(d *sql.DB, tickers []string) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM constituents_current"); err != nil {
			return err
		}
		for _, t := range tickers {
			if _, err := tx.Exec("INSERT OR IGNORE INTO constituents_current (ticker) VALUES (?)", t); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceChanges reemplaza las altas/bajas del índice.
func ReplaceChanges(d *sql.DB, rows []IndexChange) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM index_changes"); err != nil {
			return err
		}
		for _, r := range rows {
			if _, err := tx.Exec("INSERT INTO index_changes (date, added, removed, reason) VALUES (?, ?, ?, ?)",
				r.Date, r.Added, r.Removed, r.Reason); err != nil {
				return err
			}
		}
		return nil
	})
}

// History es la composición histórica ordenada por fecha.
func History(d *sql.DB) ([]HistoryRow, error) {
	rows, err := d.Query("SELECT date, tickers FROM constituents_history ORDER BY date")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryRow
	for rows.Next() {
		var r HistoryRow
		if err := rows.Scan(&r.Date, &r.Tickers); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Current es la lista actual del índice.
func Current(d *sql.DB) ([]string, error) {
	rows, err := d.Query("SELECT ticker FROM constituents_current")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ChangeAfter es un alta/baja posterior a un día (las bajas y altas vacías son "").
type ChangeAfter struct{ Date, Added, Removed string }

// ChangesAfter son las altas/bajas posteriores a day, por fecha.
func ChangesAfter(d *sql.DB, day string) ([]ChangeAfter, error) {
	rows, err := d.Query("SELECT date, added, removed FROM index_changes WHERE date > ? ORDER BY date", day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChangeAfter
	for rows.Next() {
		var date string
		var added, removed sql.NullString
		if err := rows.Scan(&date, &added, &removed); err != nil {
			return nil, err
		}
		out = append(out, ChangeAfter{date, added.String, removed.String})
	}
	return out, rows.Err()
}
