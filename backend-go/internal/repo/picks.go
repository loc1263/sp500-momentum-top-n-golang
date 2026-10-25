package repo

import (
	"database/sql"
	"sort"

	"top3/internal/db"
)

const (
	pickKey     = "months = ? AND COALESCE(min_cap_billions, 0) = ?"
	pickColumns = "run_at, as_of, months, min_cap_billions, rank, ticker, return_lookback, price, market_cap"
)

// Pick es una acción elegida (o suplente) del Top actual. MarketCap es nil cuando no hay dato de acciones.
type Pick struct {
	Rank           int
	Ticker         string
	ReturnLookback float64
	Price          float64
	MarketCap      *float64
}

func capArg(minCapB float64) float64 { return minCapB }

// ReplacePicks guarda el Top y los suplentes de asOf; repetir la corrida con los mismos datos reemplaza la anterior.
// minCapB = 0 es el índice completo (se guarda como NULL).
func ReplacePicks(d *sql.DB, runAt, asOf string, months int, minCapB float64, picks, alternates []Pick) error {
	var capCol any
	if minCapB != 0 {
		capCol = minCapB
	}
	return db.Tx(d, func(tx *sql.Tx) error {
		for _, t := range []struct {
			table string
			rows  []Pick
		}{{"actual_picks", picks}, {"actual_alternates", alternates}} {
			if _, err := tx.Exec("DELETE FROM "+t.table+" WHERE as_of = ? AND "+pickKey, asOf, months, capArg(minCapB)); err != nil {
				return err
			}
			for _, p := range t.rows {
				if _, err := tx.Exec("INSERT INTO "+t.table+" ("+pickColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
					runAt, asOf, months, capCol, p.Rank, p.Ticker, p.ReturnLookback, p.Price, p.MarketCap); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// PreviousPicks es (fecha de datos, tickers) de la corrida guardada inmediatamente anterior a asOf, o ("", nil).
func PreviousPicks(d *sql.DB, asOf string, months int, minCapB float64) (string, []string, error) {
	var prev sql.NullString
	if err := d.QueryRow("SELECT MAX(as_of) FROM actual_picks WHERE as_of < ? AND "+pickKey, asOf, months, capArg(minCapB)).Scan(&prev); err != nil {
		return "", nil, err
	}
	if !prev.Valid {
		return "", nil, nil
	}
	rows, err := d.Query("SELECT ticker FROM actual_picks WHERE as_of = ? AND "+pickKey+" ORDER BY rank", prev.String, months, capArg(minCapB))
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var tickers []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return "", nil, err
		}
		tickers = append(tickers, t)
	}
	return prev.String, tickers, rows.Err()
}

// AnchorDate es la fecha de datos de la primera corrida guardada: de ahí se cuenta el calendario de rebalanceo ("" si no hay).
func AnchorDate(d *sql.DB, months int, minCapB float64) (string, error) {
	var v sql.NullString
	err := d.QueryRow("SELECT MIN(as_of) FROM actual_picks WHERE "+pickKey, months, capArg(minCapB)).Scan(&v)
	return v.String, err
}

// LatestPickRunAt es el run_at de la corrida más reciente del Top actual ("" si no hay).
func LatestPickRunAt(d *sql.DB) (string, error) {
	var v sql.NullString
	err := d.QueryRow("SELECT MAX(run_at) FROM actual_picks").Scan(&v)
	return v.String, err
}

// PickRow es una fila del Top actual con su corrida. Cap = 0 es el índice completo.
type PickRow struct {
	RunAt, AsOf string
	Months      int
	Cap         float64
	Pick
}

// PicksOfDay son las filas de las corridas de un día (YYYY-MM-DD), por run_at y puesto.
func PicksOfDay(d *sql.DB, day string) ([]PickRow, error) {
	rows, err := d.Query("SELECT run_at, as_of, months, COALESCE(min_cap_billions, 0), rank, ticker, return_lookback, price, "+
		"market_cap FROM actual_picks WHERE substr(run_at, 1, 10) = ? ORDER BY run_at, rank", day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PickRow
	for rows.Next() {
		var r PickRow
		var mc sql.NullFloat64
		if err := rows.Scan(&r.RunAt, &r.AsOf, &r.Months, &r.Cap, &r.Rank, &r.Ticker, &r.ReturnLookback, &r.Price, &mc); err != nil {
			return nil, err
		}
		if mc.Valid {
			v := mc.Float64
			r.MarketCap = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Alternates son los suplentes de una corrida.
func Alternates(d *sql.DB, runAt string, months int, minCapB float64) ([]Pick, error) {
	rows, err := d.Query("SELECT rank, ticker, return_lookback, price, market_cap FROM actual_alternates WHERE run_at = ? "+
		"AND COALESCE(min_cap_billions, 0) = ? AND months = ? ORDER BY rank", runAt, minCapB, months)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pick
	for rows.Next() {
		var p Pick
		var mc sql.NullFloat64
		if err := rows.Scan(&p.Rank, &p.Ticker, &p.ReturnLookback, &p.Price, &mc); err != nil {
			return nil, err
		}
		if mc.Valid {
			v := mc.Float64
			p.MarketCap = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TickerPrice es una acción elegida con el precio al elegirla.
type TickerPrice struct {
	Ticker string
	Price  float64
}

// PicksOfDate son las acciones de la última corrida guardada de una fecha de datos.
type PicksOfDate struct {
	AsOf  string
	Picks []TickerPrice
}

// LatestPicksByDate son las acciones de la última corrida guardada de cada fecha de datos, ordenadas por fecha, para el
// seguimiento.
func LatestPicksByDate(d *sql.DB, minCapB float64, months int) ([]PicksOfDate, error) {
	rows, err := d.Query("SELECT as_of, ticker, price FROM actual_picks WHERE "+pickKey+
		" AND run_at = (SELECT MAX(run_at) FROM actual_picks p WHERE p.as_of = actual_picks.as_of "+
		"AND COALESCE(p.min_cap_billions, 0) = ? AND p.months = ?) ORDER BY as_of, rank",
		months, minCapB, minCapB, months)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := map[string]int{}
	var out []PicksOfDate
	for rows.Next() {
		var asOf, ticker string
		var price float64
		if err := rows.Scan(&asOf, &ticker, &price); err != nil {
			return nil, err
		}
		i, ok := idx[asOf]
		if !ok {
			i = len(out)
			idx[asOf] = i
			out = append(out, PicksOfDate{AsOf: asOf})
		}
		out[i].Picks = append(out[i].Picks, TickerPrice{ticker, price})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].AsOf < out[b].AsOf })
	return out, rows.Err()
}
