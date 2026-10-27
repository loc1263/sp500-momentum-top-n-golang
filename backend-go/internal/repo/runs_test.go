package repo

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"top3/internal/db"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func addRun(t *testing.T, d *sql.DB, runAt, params string) int64 {
	t.Helper()
	res, err := d.Exec("INSERT INTO runs (run_at, params) VALUES (?, ?)", runAt, params)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	d.Exec("INSERT INTO selections (run_id, rebalance_date, rank, ticker, return_lookback) VALUES (?, '2024-01-01', 1, 'A', 0.1)", id)
	d.Exec("INSERT INTO equity (run_id, date, capital) VALUES (?, '2024-01-01', 1000)", id)
	return id
}

func ids(t *testing.T, d *sql.DB, table, column string) []int64 {
	t.Helper()
	rows, err := d.Query("SELECT DISTINCT " + column + " FROM " + table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		out = append(out, id)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

func same(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPruneRunsDropsOldDaysAndSameDayDuplicates(t *testing.T) {
	d := openTemp(t)
	addRun(t, d, "2024-01-01T10:00:00", "p")
	addRun(t, d, "2024-01-09T10:00:00", "p")
	last := addRun(t, d, "2024-01-09T11:00:00", "p") // reemplaza a la anterior del mismo día
	other := addRun(t, d, "2024-01-09T11:00:00", "q")
	n, err := PruneRuns(d, 3)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	want := []int64{last, other}
	for _, c := range [][2]string{{"runs", "id"}, {"selections", "run_id"}, {"equity", "run_id"}} {
		if got := ids(t, d, c[0], c[1]); !same(got, want) {
			t.Errorf("%s: %v, quiero %v", c[0], got, want)
		}
	}
}

func TestPruneRunsKeepsTheLastDaysCountedFromTheLatestRun(t *testing.T) {
	d := openTemp(t)
	addRun(t, d, "2024-01-06T10:00:00", "x") // 4.º día contando desde el último: fuera con keep_days=3
	a := addRun(t, d, "2024-01-08T10:00:00", "a")
	b := addRun(t, d, "2024-01-09T10:00:00", "b")
	if n, _ := PruneRuns(d, 3); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if got := ids(t, d, "runs", "id"); !same(got, []int64{a, b}) {
		t.Fatalf("quedaron %v", got)
	}
}

func TestPruneRunsNeverRemovesTheLatestDayAndToleratesAnEmptyDatabase(t *testing.T) {
	d := openTemp(t)
	if n, err := PruneRuns(d, 7); n != 0 || err != nil {
		t.Fatalf("vacía: %d %v", n, err)
	}
	keep := addRun(t, d, "2024-01-09T10:00:00", "k")
	if n, _ := PruneRuns(d, 0); n != 0 {
		t.Fatalf("n=%d", n)
	}
	if got := ids(t, d, "runs", "id"); !same(got, []int64{keep}) {
		t.Fatalf("quedaron %v", got)
	}
}

func sweepParams(extra map[string]any) string {
	m := map[string]any{"sweep_months": 4, "offset_months": 0, "min_cap_billions": nil, "backtest": map[string]any{"backtest_years": 5}}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestPruneRunsDedupesByReportKeyNotByExactParams(t *testing.T) {
	d := openTemp(t)
	addRun(t, d, "2024-01-09T10:00:00", sweepParams(map[string]any{"data": map[string]any{"runs_keep_days": nil}})) // config vieja
	n1 := addRun(t, d, "2024-01-09T11:00:00", sweepParams(map[string]any{"data": map[string]any{"runs_keep_days": 7}}))
	n2 := addRun(t, d, "2024-01-09T11:00:00", sweepParams(map[string]any{"sweep_months": 6}))
	n3 := addRun(t, d, "2024-01-09T11:00:00", sweepParams(map[string]any{"min_cap_billions": 200.0}))
	if n, _ := PruneRuns(d, 7); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if got := ids(t, d, "runs", "id"); !same(got, []int64{n1, n2, n3}) {
		t.Fatalf("quedaron %v", got)
	}
}

func TestReportAndPruneUseTheSameRunKey(t *testing.T) {
	parse := func(s string) (RunKey, bool) {
		var m map[string]any
		json.Unmarshal([]byte(s), &m)
		return ParseRunKey(m)
	}
	if k, ok := parse(sweepParams(nil)); !ok || k != (RunKey{0, 5, 4, 0, 0}) {
		t.Fatalf("clave: %+v", k)
	}
	if k, _ := parse(sweepParams(map[string]any{"skip_months": 1})); k.Skip != 1 {
		t.Fatalf("skip: %+v", k)
	}
	if _, ok := parse(`{"months": 4}`); ok {
		t.Fatal("una corrida que no es de un sweep no tiene clave")
	}
	a, b := RunKey{0, 5, 4, 0, 0}, RunKey{0, 5, 4, 1, 0}
	if !a.Less(b) || b.Less(a) {
		t.Fatal("orden de claves")
	}
}

func TestMaybePruneRunsPrunesEveryTimeButVacuumsOnceADay(t *testing.T) {
	d := openTemp(t)
	addRun(t, d, "2020-01-01T10:00:00", "a")
	addRun(t, d, "2024-01-09T10:00:00", "b")
	if n, _ := MaybePruneRuns(d, 1, false); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if v, _ := db.GetMeta(d, "runs_vacuumed"); v != time.Now().Format("2006-01-02") {
		t.Fatalf("runs_vacuumed = %q", v)
	}
	addRun(t, d, "2020-01-01T10:00:00", "c") // otra corrida el mismo día: se limpia igual
	if n, _ := MaybePruneRuns(d, 1, false); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if n, _ := MaybePruneRuns(d, 1, false); n != 0 {
		t.Fatalf("n=%d", n)
	}
}

func TestEtfqKeepsOnlyTheLatestRunAndUpdatesCategories(t *testing.T) {
	d := openTemp(t)
	if p, err := LatestEtfq(d); p != nil || err != nil {
		t.Fatalf("sin corridas: %s %v", p, err)
	}
	for _, run := range []struct{ at, asOf, payload string }{
		{"2024-08-29T10:00:00", "2024-08-29", `{"v":1}`}, {"2024-08-30T10:00:00", "2024-08-30", `{"v":2}`}} {
		if err := SaveLatestEtfq(d, run.at, run.asOf, json.RawMessage(run.payload)); err != nil {
			t.Fatal(err)
		}
	}
	if p, _ := LatestEtfq(d); string(p) != `{"v":2}` {
		t.Fatalf("último: %s", p)
	}
	if n := countRows(t, d, "etfq_runs"); n != 1 {
		t.Fatalf("%d corridas guardadas", n)
	}

	if err := SaveEtfqCategories(d, map[string]string{"IVV": "Large Blend", "IWM": "Small Blend"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveEtfqCategories(d, map[string]string{"IWM": "Small Value"}); err != nil {
		t.Fatal(err)
	}
	got, err := EtfqCategories(d)
	if err != nil || len(got) != 2 || got["IVV"][0] != "Large Blend" || got["IWM"][0] != "Small Value" || got["IWM"][1] != time.Now().Format("2006-01-02") {
		t.Fatalf("categorías: %v %v", got, err)
	}
}

func countRows(t *testing.T, d *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
