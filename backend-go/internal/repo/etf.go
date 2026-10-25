package repo

import (
	"database/sql"
	"encoding/json"
	"time"

	"top3/internal/db"
)

// EtfScoreRow es una fila del ranking de ETFs: puesto, ticker, score, volatilidad, caída máxima, distancia a la media de
// 200 ruedas y retornos por plazo (en meses, como texto).
type EtfScoreRow struct {
	Rank    int
	Ticker  string
	Score   float64
	Vol     float64
	MaxDD   float64
	SMADist float64
	Returns map[string]float64
	RunAt   string
}

// ReplaceEtfScores guarda el ranking de asOf; repetir la corrida con los mismos datos lo reemplaza.
func ReplaceEtfScores(d *sql.DB, asOf string, rows []EtfScoreRow) error {
	runAt := time.Now().Format("2006-01-02T15:04:05")
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM etf_scores WHERE as_of = ?", asOf); err != nil {
			return err
		}
		for _, r := range rows {
			returns, err := json.Marshal(r.Returns)
			if err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT INTO etf_scores (run_at, as_of, rank, ticker, score, vol, max_dd, sma_dist, returns) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", runAt, asOf, r.Rank, r.Ticker, r.Score, r.Vol, r.MaxDD, r.SMADist, string(returns)); err != nil {
				return err
			}
		}
		return nil
	})
}

// PreviousEtfTop es (fecha, tickers del top) del ranking guardado inmediatamente anterior a asOf, o ("", nil).
func PreviousEtfTop(d *sql.DB, asOf string, topN int) (string, []string, error) {
	var prev sql.NullString
	if err := d.QueryRow("SELECT MAX(as_of) FROM etf_scores WHERE as_of < ?", asOf).Scan(&prev); err != nil {
		return "", nil, err
	}
	if !prev.Valid {
		return "", nil, nil
	}
	rows, err := d.Query("SELECT ticker FROM etf_scores WHERE as_of = ? AND rank <= ? ORDER BY rank", prev.String, topN)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return "", nil, err
		}
		out = append(out, t)
	}
	return prev.String, out, rows.Err()
}

// LatestEtfAsOf es la fecha del último ranking guardado ("" si no hay).
func LatestEtfAsOf(d *sql.DB) string {
	var v sql.NullString
	_ = d.QueryRow("SELECT MAX(as_of) FROM etf_scores").Scan(&v)
	return v.String
}

// EtfRanking es el ranking de un día.
func EtfRanking(d *sql.DB, asOf string) ([]EtfScoreRow, error) {
	rows, err := d.Query("SELECT rank, ticker, score, vol, max_dd, sma_dist, returns, run_at FROM etf_scores "+
		"WHERE as_of = ? ORDER BY rank", asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EtfScoreRow
	for rows.Next() {
		var r EtfScoreRow
		var returns string
		if err := rows.Scan(&r.Rank, &r.Ticker, &r.Score, &r.Vol, &r.MaxDD, &r.SMADist, &returns, &r.RunAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(returns), &r.Returns); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
