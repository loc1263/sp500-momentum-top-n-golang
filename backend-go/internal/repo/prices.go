// Package repo es el acceso a SQLite: un archivo por agregado. Todo el SQL del proyecto vive acá (y el esquema en
// internal/db). Las funciones leen todas las filas antes de seguir: la base usa una sola conexión.
package repo

import (
	"database/sql"
	"math"
	"strings"
	"time"

	"top3/internal/db"
	"top3/internal/model"
)

// SavePrices guarda precios en formato largo (INSERT OR REPLACE).
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

// ClearPrices borra todos los precios.
func ClearPrices(d *sql.DB) error {
	_, err := d.Exec("DELETE FROM prices")
	return err
}

// DeletePricesBefore borra los precios anteriores a day (YYYY-MM-DD) y devuelve cuántas filas.
func DeletePricesBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM prices WHERE date < ?", day))
}

// ForgetNoDataBefore olvida los tickers sin datos consultados hasta day inclusive (ya vencieron: se volverían a pedir igual).
func ForgetNoDataBefore(d *sql.DB, day string) (int64, error) {
	return rowsAffected(d.Exec("DELETE FROM no_data_tickers WHERE checked <= ?", day))
}

// DeletePriceTickers borra todos los precios de los tickers.
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

// HasPrices dice si hay algún precio guardado.
func HasPrices(d *sql.DB) bool {
	var one int
	return d.QueryRow("SELECT 1 FROM prices LIMIT 1").Scan(&one) == nil
}

// LastDates es {ticker: último día con precio}.
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

// LatestPriceDate es el último día con algún precio ("" si no hay).
func LatestPriceDate(d *sql.DB) string {
	var v sql.NullString
	_ = d.QueryRow("SELECT MAX(date) FROM prices").Scan(&v)
	return v.String
}

// ClosesOnOrBefore es {ticker: último cierre en day o antes} en una sola consulta; los tickers sin precios no figuran.
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

// StoredSince son los cierres guardados desde since, por ticker y fecha.
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

// PriceMatrixSince son los cierres desde start como matriz (fechas x tickers).
func PriceMatrixSince(d *sql.DB, start string) (*model.Prices, error) {
	rows, err := d.Query("SELECT date, ticker, close FROM prices WHERE date >= ?", start)
	if err != nil {
		return nil, err
	}
	return readMatrix(rows)
}

// PriceMatrix son todos los cierres como matriz (fechas x tickers).
func PriceMatrix(d *sql.DB) (*model.Prices, error) {
	rows, err := d.Query("SELECT date, ticker, close FROM prices")
	if err != nil {
		return nil, err
	}
	return readMatrix(rows)
}

// KnownNoData son los tickers que Yahoo dijo hace menos de ttlDays días que no existen o están dados de baja.
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

// RememberNoData anota los tickers sin datos de esta descarga y olvida los que ahora sí tienen.
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

// OHLCVRow es una fila de la tabla ohlcv.
type OHLCVRow struct {
	Ticker, Date                   string
	Open, High, Low, Close, Volume float64
}

// ReplaceOHLCV reemplaza todo el OHLCV guardado.
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

// OHLCVFrames son los campos del OHLCV como matrices fecha x ticker.
type OHLCVFrames struct {
	Open, High, Low, Close, Volume *model.Prices
}

// LoadOHLCV lee el OHLCV guardado como una matriz por campo.
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
	// Las cinco matrices comparten fechas y tickers para poder recorrerlas juntas.
	f := OHLCVFrames{Close: model.FromRows(closeRows)}
	align := func(r []model.PriceRow) *model.Prices {
		p := model.FromRows(r)
		return alignTo(p, f.Close)
	}
	f.Open, f.High, f.Low, f.Volume = align(open), align(high), align(low), align(vol)
	return f, nil
}

// alignTo devuelve p con las mismas filas y columnas que ref (lo que falte, NaN).
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

// nullable guarda un NaN como NULL (SQLite no tiene NaN).
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
