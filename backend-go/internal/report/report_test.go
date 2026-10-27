package report

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/model"
	"top3/internal/repo"
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

func seedRun(t *testing.T, d *sql.DB, months, offset int, cap any, years float64, skip ...int) {
	t.Helper()
	params := map[string]any{"sweep_months": months, "offset_months": offset, "min_cap_billions": cap,
		"backtest": map[string]any{"backtest_years": years}}
	final, total, cagr := 1210.0, 21.0, 30.0
	history := []model.Rebalance{
		{Date: "2024-01-01", Picks: []model.Pick{{Ticker: "AAA", Return: 0.5}, {Ticker: "BBB", Return: 0.4}}},
		{Date: "2024-05-01", Picks: []model.Pick{{Ticker: "CCC", Return: 0.3}, {Ticker: "AAA", Return: 0.2}}},
	}
	equity := []model.EquityPoint{{Date: dateutil.MustParse("2024-01-01"), Capital: 1000}, {Date: dateutil.MustParse("2024-05-01"), Capital: 1100},
		{Date: dateutil.MustParse("2024-09-01"), Capital: 1210}}
	if len(skip) > 0 {
		params["skip_months"] = skip[0]
		final, total, cagr = 1500, 50, 60
		history = history[:1]
		history[0].Picks = []model.Pick{{Ticker: "ZZZ", Return: 0.1}}
		equity = equity[:1]
		equity = append(equity, model.EquityPoint{Date: dateutil.MustParse("2024-09-01"), Capital: 1500})
	}
	_, err := repo.SaveRun(d, params, "2024-01-01", "2024-09-01", model.Result{Equity: equity, History: history, FinalCapital: final, TotalReturnPct: total, CAGRPct: cagr})
	if err != nil {
		t.Fatal(err)
	}
}

func build(t *testing.T, d *sql.DB, cfg config.Config) *Report {
	t.Helper()
	r, err := Build(d, cfg, dateutil.MustParse("2024-09-05"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestBuildReportGroupsLatestRuns(t *testing.T) {
	d, cfg := openTemp(t), config.Default() // periods_years = [3, 5, 10]
	seedRun(t, d, 4, 0, nil, 5)
	seedRun(t, d, 4, 1, nil, 5)
	seedRun(t, d, 4, 0, nil, 5) // repetida: gana la última
	seedRun(t, d, 6, 0, 10.0, 5)
	seedRun(t, d, 4, 0, nil, 10)
	seedRun(t, d, 4, 0, nil, 7) // período fuera de [backtest] periods_years: se ignora
	r := build(t, d, cfg)

	if len(r.Backtest) != 2 || r.Backtest[0].MinCapBillions != nil || *r.Backtest[1].MinCapBillions != 10 {
		t.Fatalf("universos: %+v", r.Backtest)
	}
	full := r.Backtest[0]
	if full.Label != "índice completo" || len(full.Periods) != 2 || full.Periods[0].Years != 5 || full.Periods[1].Years != 10 {
		t.Fatalf("períodos: %+v", full.Periods)
	}
	if full.Periods[0].StartDate != "2024-01-01" || full.Periods[0].EndDate != "2024-09-01" || !reflect.DeepEqual(r.Settings.PeriodsYears, []float64{5, 10}) {
		t.Fatalf("fechas o años: %+v %v", full.Periods[0], r.Settings.PeriodsYears)
	}
	term := full.Periods[0].Terms[0]
	if term.Months != 4 || len(term.Calendars) != 2 || term.Calendars[0].Offset != 0 || term.Calendars[1].Offset != 1 || term.Summary.Calendars != 2 {
		t.Fatalf("plazo: %+v", term)
	}
	cal := term.Calendars[0]
	if cal.TotalReturn != 0.21 || cal.CAGR != 0.30 || cal.History[0].Picks[0].Ticker != "AAA" || cal.History[0].Picks[1].Ticker != "BBB" {
		t.Fatalf("calendario: %+v", cal)
	}
	if *cal.History[1].Capital != 1100 || math.Abs(*cal.History[1].Gain-0.1) > 1e-9 || *cal.FirstRebalance != "2024-01-01" {
		t.Fatalf("historial: %+v", cal.History[1])
	}
	if len(r.Actual) != 0 {
		t.Fatalf("Top actual: %+v", r.Actual)
	}
}

func TestBuildReportWithEmptyDatabase(t *testing.T) {
	r := build(t, openTemp(t), config.Default())
	if r.RunDate != nil || len(r.Backtest) != 0 || len(r.Actual) != 0 || r.Etf != nil {
		t.Fatalf("reporte: %+v", r)
	}
	js, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	json.Unmarshal(js, &back)
	if back["backtest"].([]any) == nil || back["actual"].([]any) == nil || back["quant"] != nil || back["etfq"] != nil || back["etf"] != nil {
		t.Fatalf("JSON: %s", js)
	}
}

func TestBuildReportSeparatesTheSkipMonthModel(t *testing.T) {
	d := openTemp(t)
	seedRun(t, d, 4, 0, nil, 5)
	seedRun(t, d, 4, 0, nil, 5, 1)
	r := build(t, d, config.Default())
	base := r.Backtest[0].Periods[0].Terms[0].Calendars[0]
	skip := r.BacktestSkip[0].Periods[0].Terms[0].Calendars[0]
	if base.FinalCapital != 1210 || skip.FinalCapital != 1500 || skip.History[0].Picks[0].Ticker != "ZZZ" {
		t.Fatalf("base=%+v skip=%+v", base, skip)
	}
}

func insertPicks(t *testing.T, d *sql.DB, rows ...[]any) {
	t.Helper()
	for _, r := range rows {
		if _, err := d.Exec("INSERT INTO actual_picks VALUES (?,?,?,?,?,?,?,?,?)", r...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestActualPicksTakeTheLatestRunOfEachUniverse(t *testing.T) {
	d := openTemp(t)
	insertPicks(t, d,
		[]any{"2024-09-01T10:00:00", "2024-08-30", 4, nil, 1, "OLD", 0.1, 1.0, 1e9},
		[]any{"2024-09-01T11:00:00", "2024-08-30", 4, nil, 1, "AAA", 0.9, 10.0, 5e9},
		[]any{"2024-09-01T11:00:00", "2024-08-30", 4, nil, 2, "BBB", 0.8, 20.0, nil},
		[]any{"2024-09-01T11:00:00", "2024-08-30", 4, 200.0, 1, "MEGA", 0.5, 30.0, 3e11})
	got, err := buildActual(d, dateutil.MustParse("2024-09-05"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Label != "índice completo" || got[1].Label != "mega-cap" ||
		got[0].Picks[0].Ticker != "AAA" || got[0].Picks[1].Ticker != "BBB" || got[0].AsOf != "2024-08-30" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Picks[1].MarketCap != nil || *got[0].Picks[0].MarketCap != 5e9 {
		t.Fatal("market_cap nulo")
	}
}

func TestActualIncludesRotation(t *testing.T) {
	d := openTemp(t)
	insertPicks(t, d,
		[]any{"2024-08-29T10:00:00", "2024-08-29", 4, nil, 1, "AAA", 0.9, 10.0, nil},
		[]any{"2024-08-29T10:00:00", "2024-08-29", 4, nil, 2, "OLD", 0.8, 10.0, nil},
		[]any{"2024-08-30T10:00:00", "2024-08-30", 4, nil, 1, "AAA", 0.9, 10.0, nil},
		[]any{"2024-08-30T10:00:00", "2024-08-30", 4, nil, 2, "NEW", 0.8, 10.0, nil})
	got, _ := buildActual(d, dateutil.MustParse("2024-09-05"))
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	a := got[0]
	if *a.PreviousAsOf != "2024-08-29" || !reflect.DeepEqual(*a.Changes, Changes{Entered: []string{"NEW"}, Exited: []string{"OLD"}, Kept: []string{"AAA"}}) ||
		*a.Anchor != "2024-08-29" || a.NextRebalance != "2024-12-01" {
		t.Fatalf("rotación: %+v", a)
	}
}

func TestActualWithoutPreviousRunHasNoChanges(t *testing.T) {
	d := openTemp(t)
	insertPicks(t, d, []any{"2024-08-30T10:00:00", "2024-08-30", 4, nil, 1, "AAA", 0.9, 10.0, nil})
	got, _ := buildActual(d, dateutil.MustParse("2024-09-05"))
	if got[0].PreviousAsOf != nil || got[0].Changes != nil {
		t.Fatalf("got %+v", got[0])
	}
}

func addEtf(t *testing.T, d *sql.DB, asOf string, ranking ...string) {
	t.Helper()
	for i, tk := range ranking {
		rank := i + 1
		if _, err := d.Exec("INSERT INTO etf_scores VALUES (?,?,?,?,?,?,?,?,?)", asOf+"T10:00:00", asOf, rank, tk, 2.0-float64(rank)/10, 0.1, -0.05, 0.03, `{"3": 0.05, "12": 0.2}`); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEtfRankingInTheReport(t *testing.T) {
	d, cfg := openTemp(t), config.Default()
	csvPath := filepath.Join(t.TempDir(), "e.csv")
	os.WriteFile(csvPath, []byte("ticker,name,category\nVTI,Total Stock Market,Amplio\nVUSB,Ultra-Short Bond,Bonos\n"), 0o644)
	cfg.Etf.CSV, cfg.Etf.TopN, cfg.Etf.RiskFreeTicker = csvPath, 1, "VUSB"
	if e, _ := buildEtf(d, cfg); e != nil {
		t.Fatal("sin ranking guardado no hay sección de ETFs")
	}
	addEtf(t, d, "2024-08-29", "VTI", "VUSB")
	addEtf(t, d, "2024-08-30", "VUSB", "VTI")
	e, err := buildEtf(d, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if e.AsOf != "2024-08-30" || !reflect.DeepEqual(e.Horizons, []int{3, 12}) || *e.RiskFree != "VUSB" || e.Rows[0].Ticker != "VUSB" {
		t.Fatalf("etf: %+v", e)
	}
	want := EtfRow{Rank: 2, Ticker: "VTI", Name: "Total Stock Market", Category: "Amplio", Score: 1.8, Vol: 0.1, MaxDD: -0.05, SMADist: 0.03,
		Returns: map[string]float64{"3": 0.05, "12": 0.2}}
	if got := e.Rows[1]; math.Abs(got.Score-1.8) > 1e-12 || got.Name != want.Name || got.Category != want.Category || !reflect.DeepEqual(got.Returns, want.Returns) {
		t.Fatalf("fila: %+v", got)
	}
	if *e.PreviousAsOf != "2024-08-29" || !reflect.DeepEqual(*e.Changes, Changes{Entered: []string{"VUSB"}, Exited: []string{"VTI"}, Kept: []string{}}) { // top_n = 1
		t.Fatalf("cambios: %+v", e.Changes)
	}
}

func TestEtfReportSurvivesAMissingCSV(t *testing.T) {
	d, cfg := openTemp(t), config.Default()
	cfg.Etf.CSV, cfg.Etf.RiskFreeTicker = filepath.Join(t.TempDir(), "no_existe.csv"), ""
	addEtf(t, d, "2024-08-30", "VTI")
	e, err := buildEtf(d, cfg)
	if err != nil || e.Rows[0].Name != "" || e.RiskFree != nil || e.Changes != nil {
		t.Fatalf("etf: %+v err %v", e, err)
	}
}

func TestHalvesSplitThePeriodAtItsMidpoint(t *testing.T) {
	// 4 años: capital x2 en la primera mitad y x1,5 en la segunda
	cal := Calendar{Offset: 0, StartDate: "2020-01-01", EndDate: "2024-01-01", Equity: []EquityPoint{
		{"2020-01-01", 1000}, {"2022-01-01", 2000}, {"2024-01-01", 3000}}}
	h, err := halves([]Calendar{cal})
	if err != nil {
		t.Fatal(err)
	}
	if h.Split != "2021-12-31" { // mitad exacta del período
		t.Fatalf("split %s", h.Split)
	}
	if math.Abs(*h.First-(math.Sqrt(2)-1)) > 0.01 || math.Abs(*h.Second-(math.Sqrt(1.5)-1)) > 0.01 {
		t.Fatalf("mitades: %v %v", *h.First, *h.Second)
	}
}

func TestHalvesLeaveATooShortHalfEmpty(t *testing.T) {
	cal := Calendar{Offset: 0, StartDate: "2024-01-01", EndDate: "2024-03-01", Equity: []EquityPoint{{"2024-01-01", 1000}, {"2024-03-01", 1100}}}
	h, _ := halves([]Calendar{cal})
	if h.First != nil {
		t.Fatalf("first = %v", *h.First)
	}
}

func seedClosePrices(t *testing.T, d *sql.DB, rows ...[]any) {
	t.Helper()
	for _, r := range rows {
		if _, err := d.Exec("INSERT INTO prices VALUES (?,?,?)", r...); err != nil {
			t.Fatal(err)
		}
	}
}

func seedTrack(t *testing.T, d *sql.DB, asOf string, picks ...[2]any) {
	t.Helper()
	for i, p := range picks {
		insertPicks(t, d, []any{asOf + "T10:00:00", asOf, 4, nil, i + 1, p[0], 0.5, p[1], 1e9})
	}
}

func TestTrackingMeasuresOpenAndClosedSelections(t *testing.T) {
	d := openTemp(t)
	seedClosePrices(t, d,
		[]any{"AAA", "2024-01-02", 10.0}, []any{"AAA", "2024-05-02", 12.0}, []any{"AAA", "2024-07-01", 15.0},
		[]any{"BBB", "2024-01-02", 20.0}, []any{"BBB", "2024-05-02", 18.0}, []any{"BBB", "2024-07-01", 18.0})
	seedTrack(t, d, "2024-01-02", [2]any{"AAA", 10.0}, [2]any{"BBB", 20.0})
	seedTrack(t, d, "2024-06-03", [2]any{"AAA", 14.0}, [2]any{"BBB", 20.0})
	got, err := tracking(d, 0, 4)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v err %v", got, err)
	}
	opened, closed := got[0], got[1] // de la más nueva a la más vieja
	if opened.AsOf != "2024-06-03" || opened.Closed || math.Abs(opened.Return-((15.0/14-1)+(18.0/20-1))/2) > 1e-12 {
		t.Fatalf("abierta: %+v", opened)
	}
	if closed.AsOf != "2024-01-02" || !closed.Closed || closed.Until != "2024-05-02" || math.Abs(closed.Return-(0.2-0.1)/2) > 1e-12 { // solo hasta que terminó la tenencia de 4 meses
		t.Fatalf("cerrada: %+v", closed)
	}
}

func TestTrackingSkipsTheSelectionOfTheLastDayAndEmptyData(t *testing.T) {
	d := openTemp(t)
	if got, _ := tracking(d, 0, 4); len(got) != 0 { // sin precios
		t.Fatalf("got %+v", got)
	}
	seedClosePrices(t, d, []any{"AAA", "2024-01-02", 10.0})
	seedTrack(t, d, "2024-01-02", [2]any{"AAA", 10.0})
	if got, _ := tracking(d, 0, 4); len(got) != 0 { // todavía no pasó ni un día
		t.Fatalf("got %+v", got)
	}
}

func TestTermEndClampsToDay28(t *testing.T) {
	for _, c := range []struct{ from, want string }{{"2024-01-31", "2024-05-31"}, {"2024-10-31", "2025-02-28"}, {"2024-01-15", "2024-05-15"}} {
		if got := dateutil.Day(termEnd(dateutil.MustParse(c.from), 4)); got != c.want {
			t.Errorf("%s + 4 meses = %s, quiero %s", c.from, got, c.want)
		}
	}
}

func TestExportWritesJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.DBPath = filepath.Join(dir, "t.db")
	cfg.Output.FrontendData = filepath.Join(dir, "x", "report.json")
	cfg.Output.HistoryFile = filepath.Join(dir, "history.json")
	cfg.Output.LogDir = dir
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	seedRun(t, d, 4, 0, nil, 5)
	d.Close()
	path, err := Export(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var back struct {
		Backtest []struct {
			Periods []struct {
				Terms []struct {
					Months int `json:"months"`
				} `json:"terms"`
			} `json:"periods"`
		} `json:"backtest"`
		GeneratedAt string `json:"generated_at"`
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Backtest[0].Periods[0].Terms[0].Months != 4 {
		t.Fatalf("JSON: %s", raw)
	}
	if _, err := time.Parse("2006-01-02T15:04:05", back.GeneratedAt); err != nil {
		t.Fatalf("generated_at = %q", back.GeneratedAt)
	}
}

func TestEtfqPayloadPassesThroughToTheReport(t *testing.T) {
	d, cfg := openTemp(t), config.Default()
	r, err := Build(d, cfg, time.Now())
	if err != nil || string(r.Etfq) != "null" {
		t.Fatalf("sin corrida guardada: %s %v", r.Etfq, err)
	}
	if err := repo.SaveLatestEtfq(d, "2024-08-30T10:00:00", "2024-08-30", json.RawMessage(`{"as_of":"2024-08-30","rows":[]}`)); err != nil {
		t.Fatal(err)
	}
	if r, err = Build(d, cfg, time.Now()); err != nil || string(r.Etfq) != `{"as_of":"2024-08-30","rows":[]}` {
		t.Fatalf("con corrida guardada: %s %v", r.Etfq, err)
	}
}
