package backtest

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/engine"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/repo"
	"top3/internal/testutil"
)

func newContext(t *testing.T, start, end string) (*Context, *sql.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	prices := testutil.DefaultPrices()
	cfg := config.Default()
	cfg.Backtest.BacktestYears = 2
	cons := market.NewConstituentsFrom([]string{"2000-01-01"}, [][]string{prices.Tickers}, prices.Tickers)
	return &Context{DB: d, Cfg: cfg, Cons: cons, Prices: prices, Matrix: engine.NewMatrix(prices),
		Start: dateutil.MustParse(start), End: dateutil.MustParse(end), UseCache: true, KnownShares: map[string]float64{}}, d
}

func TestRunCalendarsSavesOneRunPerOffset(t *testing.T) {
	c, d := newContext(t, "2023-09-01", "2024-12-31")
	term, err := c.RunCalendars(3, nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(term.Calendars) != 3 || term.Calendars[0].Offset != 0 || term.Calendars[2].Offset != 2 {
		t.Fatalf("calendarios: %+v", term.Calendars)
	}
	rows, err := d.Query("SELECT params, start_date FROM runs ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var params, starts []string
	for rows.Next() {
		var p, s string
		rows.Scan(&p, &s)
		params, starts = append(params, p), append(starts, s)
	}
	rows.Close()
	if len(params) != 3 || starts[1] != "2023-10-01" { // arranque desfasado un mes
		t.Fatalf("filas: %v %v", params, starts)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(params[1]), &got); err != nil {
		t.Fatal(err)
	}
	if got["sweep_months"] != 3.0 || got["offset_months"] != 1.0 || got["min_cap_billions"] != nil ||
		got["backtest"].(map[string]any)["backtest_years"] != 2.0 {
		t.Fatalf("params: %v", got)
	}
	key, ok := repo.ParseRunKey(got)
	if !ok || key != (repo.RunKey{Cap: 0, Years: 2, Months: 3, Offset: 1, Skip: 0}) {
		t.Fatalf("clave: %+v %v", key, ok)
	}
	var nEquity, nSel int
	d.QueryRow("SELECT COUNT(*) FROM equity").Scan(&nEquity)
	d.QueryRow("SELECT COUNT(*) FROM selections").Scan(&nSel)
	if nEquity == 0 || nSel == 0 {
		t.Fatal("faltan la curva de capital y las selecciones")
	}
}

func TestRunCalendarsSkipsFailedOffsets(t *testing.T) {
	c, _ := newContext(t, "2024-12-20", "2024-12-31") // sin ruedas de ranking suficientes
	term, err := c.RunCalendars(3, nil, nil, 0)
	if err != nil || len(term.Calendars) != 0 {
		t.Fatalf("calendarios %d, err %v", len(term.Calendars), err)
	}
}

func TestRunTermsCoversConfiguredMonths(t *testing.T) {
	c, _ := newContext(t, "2023-09-01", "2024-12-31")
	c.Cfg.Simulation.Months = []int{2, 3}
	terms, err := c.RunTerms(nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 || terms[0].Months != 2 || len(terms[0].Calendars) != 2 || len(terms[1].Calendars) != 3 {
		t.Fatalf("plazos: %+v", terms)
	}
}

func TestSkipMonthRunsAreStoredWithTheirOwnKey(t *testing.T) {
	c, d := newContext(t, "2023-09-01", "2024-12-31")
	if _, err := c.RunCalendars(3, nil, nil, SkipMonths); err != nil {
		t.Fatal(err)
	}
	day, _ := repo.LatestRunDay(d)
	runs, _ := repo.RunsOfDay(d, day)
	var params map[string]any
	json.Unmarshal([]byte(runs[0].Params), &params)
	if key, _ := repo.ParseRunKey(params); key.Skip != 1 {
		t.Fatalf("clave: %+v", key)
	}
}

func TestSkippedRebalancesAreReportedInASingleWarning(t *testing.T) {
	c, _ := newContext(t, "2023-06-01", "2024-12-31")
	var buf strings.Builder
	logx.SetConsole(&buf, logx.Info)
	defer logx.SetConsole(nil, logx.Info)
	// el índice tiene solo 5 tickers hasta fin de 2023 (menos de los 10 mínimos)
	c.Cons = market.NewConstituentsFrom([]string{"2000-01-01", "2024-01-01"},
		[][]string{c.Prices.Tickers[:5], c.Prices.Tickers}, c.Prices.Tickers)
	if _, err := c.RunTerms(nil, []int{3, 4}, 0); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Count(out, "TOP-BKT-003") != 1 { // y no uno por fecha, plazo y desfase
		t.Fatalf("avisos:\n%s", out)
	}
	if !strings.Contains(out, "fechas de rebalanceo (de 2023-06-01 a 2023-12-01)") ||
		!strings.Contains(out, "índice completo, backtest 2023-06-01 a 2024-12-31") {
		t.Fatalf("aviso:\n%s", out)
	}
}

func TestAttachMarketCapsReusesSharesAlreadyKnown(t *testing.T) {
	c, _ := newContext(t, "2023-09-01", "2024-12-31")
	term, _ := c.RunCalendars(3, nil, nil, 0)
	for _, p := range term.Base().History[0].Picks {
		c.KnownShares[p.Ticker] = 10
	}
	// todo lo elegido ya se conoce: no hace falta ir a Yahoo (el Downloader es nil y fallaría)
	if err := c.AttachMarketCaps(t.Context(), []model.Term{term}, nil); err != nil {
		t.Fatal(err)
	}
	caps := term.Base().History[0].Caps
	if len(caps) != 4 {
		t.Fatalf("caps: %v", caps)
	}
}

func TestResolvePeriods(t *testing.T) {
	end := dateutil.MustParse("2026-10-24")
	bt := config.Default().Backtest
	bt.PeriodsYears = []float64{10, 2, 4}
	got, err := ResolvePeriods(bt, end, 0)
	if err != nil || len(got) != 3 || got[0].Years != 2 || dateutil.Day(got[0].Start) != "2024-10-24" || got[2].Years != 10 {
		t.Fatalf("períodos: %+v %v", got, err)
	}
	if got, _ := ResolvePeriods(bt, end, 6); len(got) != 1 || got[0].Years != 6 {
		t.Fatalf("--years: %+v", got)
	}
	bt.StartDate = "2020-10-24"
	got, _ = ResolvePeriods(bt, end, 0)
	if len(got) != 1 || got[0].Years != 6 {
		t.Fatalf("start_date explícito: %+v", got)
	}
}

func TestResolveLevels(t *testing.T) {
	cfg := config.Default()
	if got := ResolveLevels(Job{}, cfg); len(got) != 1 || got[0] != 0 {
		t.Fatalf("sin filtro: %v", got)
	}
	if got := ResolveLevels(Job{Sweep: true}, cfg); len(got) != 3 || got[2] != 200 {
		t.Fatalf("sweep: %v", got)
	}
	if got := ResolveLevels(Job{Sweep: true, LargeCap: true}, cfg); len(got) != 1 || got[0] != 10 {
		t.Fatalf("large-cap: %v", got)
	}
	cap := 50.0
	if got := ResolveLevels(Job{Sweep: true, MinCapBillions: &cap}, cfg); len(got) != 1 || got[0] != 50 {
		t.Fatalf("min-cap: %v", got)
	}
}
