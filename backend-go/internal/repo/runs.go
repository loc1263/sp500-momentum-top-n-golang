package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"top3/internal/db"
	"top3/internal/logx"
	"top3/internal/model"
)

// RunRow es el resumen de una corrida del backtest.
type RunRow struct {
	ID             int64
	Params         string
	StartDate      string
	EndDate        string
	FinalCapital   float64
	TotalReturnPct float64
	CAGRPct        float64
}

// insertRows inserta las filas de a muchas por sentencia (INSERT ... VALUES (...), (...)): una corrida del barrido son
// cientos de filas y una sentencia por fila era lo que más tardaba. cols es la cantidad de columnas de table.
func insertRows(tx *sql.Tx, table string, cols int, rows [][]any) error {
	const perStmt = 100 // 100 * 5 columnas = 500 parámetros, bajo el límite de SQLite
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

// SaveRun guarda una corrida (resumen, selecciones y curva de capital) y devuelve su id.
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

// RunKey es qué corrida es para el informe: universo, años del período, plazo, desfase y meses saltados del modelo.
type RunKey struct {
	Cap    float64
	Years  float64
	Months int
	Offset int
	Skip   int
}

// Less ordena las claves por sus campos, en ese orden.
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

// ParseRunKey extrae la clave de los parámetros de una corrida; ok es false si no es de un sweep. Las corridas anteriores
// al modelo skip-month cuentan como skip 0.
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

// LatestRunDay es la fecha (YYYY-MM-DD) de la corrida más reciente ("" si no hay ninguna).
func LatestRunDay(d *sql.DB) (string, error) {
	var v sql.NullString
	err := d.QueryRow("SELECT MAX(substr(run_at, 1, 10)) FROM runs").Scan(&v)
	return v.String, err
}

// RunsOfDay son las corridas de un día, de la más vieja a la más nueva.
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

// EquityPointRow es (fecha, capital).
type EquityPointRow struct {
	Date    string
	Capital float64
}

// EquityCurve es la curva de capital de una corrida.
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

// SelectionRow es (fecha de rebalanceo, puesto, ticker, retorno del ranking).
type SelectionRow struct {
	Date   string
	Rank   int
	Ticker string
	Return float64
}

// Selections son las selecciones de una corrida.
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

// PruneRuns borra las corridas viejas del backtest y devuelve cuántas.
//
// Se conservan los últimos keepDays días con corridas (contados desde el día más reciente, mínimo 1) y, dentro de ellos,
// solo la última corrida de cada clave por día: es la única que usa el informe. Las corridas que no son de un sweep se
// agrupan por sus parámetros exactos. actual_picks no se toca (historial del Top actual).
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
			doomed = append(doomed, prev) // la anterior del mismo día queda reemplazada
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

// MaybePruneRuns borra las corridas viejas en cada llamada (es rápido). Compactar el archivo (VACUUM) es lo caro: se hace
// como mucho una vez por día, salvo force.
func MaybePruneRuns(d *sql.DB, keepDays int, force bool) (int, error) {
	n, err := PruneRuns(d, keepDays)
	if err != nil {
		return n, err
	}
	if n > 0 && (force || !db.FetchedToday(d, "runs_vacuumed")) {
		if err := db.SetMeta(d, "runs_vacuumed", db.Today()); err != nil {
			return n, err
		}
		if _, err := d.Exec("VACUUM"); err != nil { // devuelve el espacio al disco
			return n, err
		}
	}
	return n, nil
}
