// Package repo es el acceso a SQLite: un archivo por agregado. Todo el SQL del proyecto vive acá (y el esquema en
// internal/db). Las funciones leen todas las filas antes de seguir: la base usa una sola conexión.
//
// Fecha: 2026-11-19
package repo

import (
	"database/sql"
	"math"
	"strings"
	"time"

	"topwx/internal/db"
	"topwx/internal/model"
)

func SavePrices(d *sql.DB, rows []model.PriceRow) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		st, err := tx.Prepare("INSERT OR REPLACE INTO prices (ticker, date, close) VALUES (?, ?, ?)")
		if err != nil {
			return err
		}
		defer st.Close()
		for _, r := range rows {
			if _, err := st.Exec(r.Ticker, r.Date, r.Close); err != nil {
				return err
			}
		}
		return nil
	})
}

func ClearPrices(d *sql.DB) error {
	_, err := d.Exec("DELETE FROM prices")
	return err
}

func DeletePricesBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM prices WHERE date < ?", day))
}

func ForgetNoDataBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM no_data_tickers WHERE checked <= ?", day))
}

func DeletePriceTickers(d *sql.DB, tickers []string) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		for _, t := range tickers {
			if _, err := tx.Exec("DELETE FROM prices WHERE ticker = ?", t); err != nil {
				return err
			}
		}
		return nil
	})
}

func HasPrices(d *sql.DB) bool {
	var one int
	return d.QueryRow("SELECT 1 FROM prices LIMIT 1").Scan(&one) == nil
}

func LastDates(d *sql.DB) (map[string]string, error) {
	rows, err := d.Query("SELECT ticker, MAX(date) FROM prices GROUP BY ticker")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var t, day string
		if err := rows.Scan(&t, &day); err != nil {
			return nil, err
		}
		out[t] = day
	}
	return out, rows.Err()
}

func LatestPriceDate(d *sql.DB) (string, error) {
	var v sql.NullString
	err := d.QueryRow("SELECT MAX(date) FROM prices").Scan(&v)
	return v.String, err
}

func ClosesOnOrBefore(d *sql.DB, tickers []string, day string) (map[string]float64, error) {
	if len(tickers) == 0 {
		return map[string]float64{}, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(tickers)), ", ")
	args := make([]any, 0, len(tickers)+1)
	for _, t := range tickers {
		args = append(args, t)
	}
	args = append(args, day)
	rows, err := d.Query("SELECT p.ticker, p.close FROM prices p WHERE p.ticker IN ("+marks+") AND p.date = "+
		"(SELECT MAX(date) FROM prices WHERE ticker = p.ticker AND date <= ?)", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var t string
		var c float64
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		out[t] = c
	}
	return out, rows.Err()
}

func StoredSince(d *sql.DB, since string) (map[[2]string]float64, error) {
	rows, err := d.Query("SELECT ticker, date, close FROM prices WHERE date >= ?", since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[[2]string]float64{}
	for rows.Next() {
		var t, day string
		var c float64
		if err := rows.Scan(&t, &day, &c); err != nil {
			return nil, err
		}
		out[[2]string{t, day}] = c
	}
	return out, rows.Err()
}

func readMatrix(rows *sql.Rows) (*model.Prices, error) {
	defer rows.Close()
	var long []model.PriceRow
	for rows.Next() {
		var r model.PriceRow
		if err := rows.Scan(&r.Date, &r.Ticker, &r.Close); err != nil {
			return nil, err
		}
		long = append(long, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return model.FromRows(long), nil
}

func PriceMatrixSince(d *sql.DB, start string) (*model.Prices, error) {
	rows, err := d.Query("SELECT date, ticker, close FROM prices WHERE date >= ?", start)
	if err != nil {
		return nil, err
	}
	return readMatrix(rows)
}

func PriceMatrix(d *sql.DB) (*model.Prices, error) {
	rows, err := d.Query("SELECT date, ticker, close FROM prices")
	if err != nil {
		return nil, err
	}
	return readMatrix(rows)
}

func KnownNoData(d *sql.DB, ttlDays int) (map[string]bool, error) {
	out := map[string]bool{}
	if ttlDays <= 0 {
		return out, nil
	}
	cutoff := time.Now().AddDate(0, 0, -ttlDays).Format("2006-01-02")
	rows, err := d.Query("SELECT ticker FROM no_data_tickers WHERE checked > ?", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out[t] = true
	}
	return out, rows.Err()
}

func RememberNoData(d *sql.DB, gone []string, withData []string) error {
	today := db.Today()
	return db.Tx(d, func(tx *sql.Tx) error {
		for _, t := range gone {
			if _, err := tx.Exec("INSERT OR REPLACE INTO no_data_tickers (ticker, checked) VALUES (?, ?)", t, today); err != nil {
				return err
			}
		}
		for _, t := range withData {
			if _, err := tx.Exec("DELETE FROM no_data_tickers WHERE ticker = ?", t); err != nil {
				return err
			}
		}
		return nil
	})
}

type OHLCVRow struct {
	Ticker, Date                   string
	Open, High, Low, Close, Volume float64
}

func ReplaceOHLCV(d *sql.DB, rows []OHLCVRow) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM ohlcv"); err != nil {
			return err
		}
		st, err := tx.Prepare("INSERT OR REPLACE INTO ohlcv VALUES (?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer st.Close()
		for _, r := range rows {
			if _, err := st.Exec(r.Ticker, r.Date, nullable(r.Open), nullable(r.High), nullable(r.Low), r.Close, nullable(r.Volume)); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveOHLCV reemplaza el OHLCV de los tickers que vienen en rows (los demás quedan como estaban) y borra lo anterior a oldest.
func SaveOHLCV(d *sql.DB, rows []OHLCVRow, oldest string) error {
	return db.Tx(d, func(tx *sql.Tx) error {
		del, err := tx.Prepare("DELETE FROM ohlcv WHERE ticker = ?")
		if err != nil {
			return err
		}
		defer del.Close()
		ins, err := tx.Prepare("INSERT OR REPLACE INTO ohlcv VALUES (?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer ins.Close()
		cleared := map[string]bool{}
		for _, r := range rows {
			if !cleared[r.Ticker] {
				cleared[r.Ticker] = true
				if _, err := del.Exec(r.Ticker); err != nil {
					return err
				}
			}
			if _, err := ins.Exec(r.Ticker, r.Date, nullable(r.Open), nullable(r.High), nullable(r.Low), r.Close, nullable(r.Volume)); err != nil {
				return err
			}
		}
		_, err = tx.Exec("DELETE FROM ohlcv WHERE date < ?", oldest)
		return err
	})
}

// DayClose es el cierre de un ticker en una fecha.
type DayClose struct {
	Date  string
	Close float64
}

// OHLCVCloses devuelve, por ticker, los cierres guardados desde since (inclusive) en orden de fecha.
func OHLCVCloses(d *sql.DB, since string) (map[string][]DayClose, error) {
	rows, err := d.Query("SELECT ticker, date, close FROM ohlcv WHERE date >= ? ORDER BY ticker, date", since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]DayClose{}
	for rows.Next() {
		var t string
		var c DayClose
		if err := rows.Scan(&t, &c.Date, &c.Close); err != nil {
			return nil, err
		}
		out[t] = append(out[t], c)
	}
	return out, rows.Err()
}

type OHLCVFrames struct {
	Open, High, Low, Close, Volume *model.Prices
}

func LoadOHLCV(d *sql.DB) (OHLCVFrames, error) {
	rows, err := d.Query("SELECT ticker, date, open, high, low, close, volume FROM ohlcv")
	if err != nil {
		return OHLCVFrames{}, err
	}
	defer rows.Close()
	var open, high, low, closeRows, vol []model.PriceRow
	for rows.Next() {
		var t, day string
		var o, h, l, c, v sql.NullFloat64
		if err := rows.Scan(&t, &day, &o, &h, &l, &c, &v); err != nil {
			return OHLCVFrames{}, err
		}
		add := func(dst *[]model.PriceRow, x sql.NullFloat64) {
			if x.Valid {
				*dst = append(*dst, model.PriceRow{Ticker: t, Date: day, Close: x.Float64})
			}
		}
		add(&open, o)
		add(&high, h)
		add(&low, l)
		add(&closeRows, c)
		add(&vol, v)
	}
	if err := rows.Err(); err != nil {
		return OHLCVFrames{}, err
	}
	f := OHLCVFrames{Close: model.FromRows(closeRows)}
	align := func(r []model.PriceRow) *model.Prices {
		p := model.FromRows(r)
		return alignTo(p, f.Close)
	}
	f.Open, f.High, f.Low, f.Volume = align(open), align(high), align(low), align(vol)
	return f, nil
}

func alignTo(p, ref *model.Prices) *model.Prices {
	out := model.NewPrices(ref.Dates, ref.Tickers)
	rowOf := make(map[int64]int, p.Rows())
	for i, d := range p.Dates {
		rowOf[d.Unix()] = i
	}
	for j, t := range ref.Tickers {
		pj, ok := p.Col(t)
		if !ok {
			continue
		}
		for i, d := range ref.Dates {
			if pi, ok := rowOf[d.Unix()]; ok {
				out.Px[i*ref.Cols()+j] = p.At(pi, pj)
			}
		}
	}
	return out
}

func nullable(v float64) any {
	if math.IsNaN(v) {
		return nil
	}
	return v
}

func rowsAffected(res sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
