// Repositorio de corridas del backtest: guardado, claves, curvas de capital, selecciones y poda.
//
// Fecha: 2026-11-19

package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"top/internal/db"
	"top/internal/logx"
	"top/internal/model"
)

type RunRow struct {
	ID             int64
	Params         string
	StartDate      string
	EndDate        string
	FinalCapital   float64
	TotalReturnPct float64
	CAGRPct        float64
}

func insertRows(tx *sql.Tx, table string, cols int, rows [][]any) error {
	const perStmt = 100
	marks := "(" + strings.TrimSuffix(strings.Repeat("?, ", cols), ", ") + ")"
	for len(rows) > 0 {
		n := min(perStmt, len(rows))
		args := make([]any, 0, n*cols)
		for _, r := range rows[:n] {
			args = append(args, r...)
		}
		q := "INSERT INTO " + table + " VALUES " + strings.TrimSuffix(strings.Repeat(marks+", ", n), ", ")
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
		rows = rows[n:]
	}
	return nil
}

func SaveRun(d *sql.DB, params map[string]any, startDate, endDate string, r model.Result) (int64, error) {
	js, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.Tx(d, func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO runs (run_at, params, start_date, end_date, final_capital, total_return, cagr) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)", time.Now().Format("2006-01-02T15:04:05"), string(js), startDate, endDate,
			r.FinalCapital, r.TotalReturnPct, r.CAGRPct)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		var sel, eq [][]any
		for _, h := range r.History {
			for rank, p := range h.Picks {
				sel = append(sel, []any{id, h.Date, rank + 1, p.Ticker, p.Return})
			}
		}
		for _, e := range r.Equity {
			eq = append(eq, []any{id, e.Date.Format("2006-01-02"), e.Capital})
		}
		if err := insertRows(tx, "selections (run_id, rebalance_date, rank, ticker, return_lookback)", 5, sel); err != nil {
			return err
		}
		if err := insertRows(tx, "equity (run_id, date, capital)", 3, eq); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	logx.Infof("Resultados guardados en SQLite (runs.id=%d)", id)
	return id, nil
}

type RunKey struct {
	Cap    float64
	Years  float64
	Months int
	Offset int
	Skip   int
}

func (a RunKey) Less(b RunKey) bool {
	switch {
	case a.Cap != b.Cap:
		return a.Cap < b.Cap
	case a.Years != b.Years:
		return a.Years < b.Years
	case a.Months != b.Months:
		return a.Months < b.Months
	case a.Offset != b.Offset:
		return a.Offset < b.Offset
	}
	return a.Skip < b.Skip
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func ParseRunKey(params map[string]any) (RunKey, bool) {
	sm, ok := params["sweep_months"]
	if !ok {
		return RunKey{}, false
	}
	years := 5.0
	if bt, ok := params["backtest"].(map[string]any); ok {
		if y, ok := bt["backtest_years"]; ok {
			years = num(y)
		}
	}
	return RunKey{Cap: num(params["min_cap_billions"]), Years: years, Months: int(num(sm)),
		Offset: int(num(params["offset_months"])), Skip: int(num(params["skip_months"]))}, true
}

func LatestRunDay(d *sql.DB) (string, error) {
	var v sql.NullString
	err := d.QueryRow("SELECT MAX(substr(run_at, 1, 10)) FROM runs").Scan(&v)
	return v.String, err
}

func RunsOfDay(d *sql.DB, day string) ([]RunRow, error) {
	rows, err := d.Query("SELECT id, params, start_date, end_date, final_capital, total_return, cagr "+
		"FROM runs WHERE substr(run_at, 1, 10) = ? ORDER BY id", day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunRow
	for rows.Next() {
		var r RunRow
		if err := rows.Scan(&r.ID, &r.Params, &r.StartDate, &r.EndDate, &r.FinalCapital, &r.TotalReturnPct, &r.CAGRPct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type EquityPointRow struct {
	Date    string
	Capital float64
}

func EquityCurve(d *sql.DB, runID int64) ([]EquityPointRow, error) {
	rows, err := d.Query("SELECT date, capital FROM equity WHERE run_id = ? ORDER BY date", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EquityPointRow
	for rows.Next() {
		var r EquityPointRow
		if err := rows.Scan(&r.Date, &r.Capital); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SelectionRow struct {
	Date   string
	Rank   int
	Ticker string
	Return float64
}

func Selections(d *sql.DB, runID int64) ([]SelectionRow, error) {
	rows, err := d.Query("SELECT rebalance_date, rank, ticker, return_lookback FROM selections WHERE run_id = ? "+
		"ORDER BY rebalance_date, rank", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SelectionRow
	for rows.Next() {
		var r SelectionRow
		if err := rows.Scan(&r.Date, &r.Rank, &r.Ticker, &r.Return); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func PruneRuns(d *sql.DB, keepDays int) (int, error) {
	lastDay, err := LatestRunDay(d)
	if err != nil || lastDay == "" {
		return 0, err
	}
	last, err := time.Parse("2006-01-02", lastDay)
	if err != nil {
		return 0, err
	}
	cutoff := last.AddDate(0, 0, -(max(keepDays, 1) - 1)).Format("2006-01-02")

	type run struct {
		id          int64
		day, params string
	}
	rows, err := d.Query("SELECT id, substr(run_at, 1, 10), params FROM runs ORDER BY id")
	if err != nil {
		return 0, err
	}
	var all []run
	for rows.Next() {
		var r run
		if err := rows.Scan(&r.id, &r.day, &r.params); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	latest := map[string]int64{}
	var doomed []int64
	for _, r := range all {
		if r.day < cutoff {
			doomed = append(doomed, r.id)
			continue
		}
		group := r.day + "|" + r.params
		var parsed map[string]any
		if json.Unmarshal([]byte(r.params), &parsed) == nil {
			if key, ok := ParseRunKey(parsed); ok {
				group = fmt.Sprintf("%s|%v", r.day, key)
			}
		}
		if prev, ok := latest[group]; ok {
			doomed = append(doomed, prev)
		}
		latest[group] = r.id
	}
	err = db.Tx(d, func(tx *sql.Tx) error {
		for _, t := range []struct{ table, column string }{{"selections", "run_id"}, {"equity", "run_id"}, {"runs", "id"}} {
			for _, id := range doomed {
				if _, err := tx.Exec("DELETE FROM "+t.table+" WHERE "+t.column+" = ?", id); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(doomed) > 0 {
		logx.Infof("Limpieza: %d corridas viejas borradas (se conservan %d días, hasta %s)", len(doomed), keepDays, lastDay)
	}
	return len(doomed), nil
}

func MaybePruneRuns(d *sql.DB, keepDays int, force bool) (int, error) {
	n, err := PruneRuns(d, keepDays)
	if err != nil {
		return n, err
	}
	if n > 0 && (force || !db.FetchedToday(d, "runs_vacuumed")) {
		if err := db.SetMeta(d, "runs_vacuumed", db.Today()); err != nil {
			return n, err
		}
		if _, err := d.Exec("VACUUM"); err != nil {
			return n, err
		}
	}
	return n, nil
}
