package actual

import (
	"database/sql"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/repo"
)

// makePrices son n tickers que crecen a ritmos distintos durante `days` días hábiles que terminan el 2024-12-31.
func makePrices(days, n int) *model.Prices {
	var dates []time.Time
	for d := dateutil.MustParse("2024-12-31"); len(dates) < days; d = d.AddDate(0, 0, -1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			dates = append([]time.Time{d}, dates...)
		}
	}
	names := make([]string, n)
	for i := range names {
		names[i] = "T" + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	p := model.NewPrices(dates, names)
	for i := 0; i < n; i++ {
		for k := 0; k < days; k++ {
			p.Px[k*n+i] = 100 * math.Pow(1+0.001*float64(i+1), float64(k))
		}
	}
	return p
}

func names(r []string) string { return strings.Join(r, ",") }

func topNames(t *testing.T, p *model.Prices, months, topN int, u *market.Universe) ([]string, *model.Prices) {
	t.Helper()
	ranked, window, err := ComputeTop(p, months, topN, 21, u, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range ranked {
		out = append(out, r.Ticker)
	}
	return out, window
}

func TestComputeTopOrdersByReturn(t *testing.T) {
	top, window := topNames(t, makePrices(140, 12), 4, 3, nil)
	if names(top) != "T11,T10,T09" || window.Rows() != 4*21+1 {
		t.Fatalf("top=%v ventana=%d", top, window.Rows())
	}
}

func TestComputeTopDropsIncompleteTickers(t *testing.T) {
	p := makePrices(140, 12)
	j, _ := p.Col("T11")
	for i := p.Rows() - 50; i < p.Rows(); i++ {
		p.Px[i*p.Cols()+j] = math.NaN()
	}
	var buf strings.Builder
	logx.SetConsole(&buf, logx.Info)
	defer logx.SetConsole(nil, logx.Info)
	top, window := topNames(t, p, 4, 2, nil)
	if names(top) != "T10,T09" || window.Has("T11") {
		t.Fatalf("top=%v", top)
	}
	if !strings.Contains(buf.String(), "TOP-ACT-002") || !strings.Contains(buf.String(), "T11") {
		t.Fatalf("falta el aviso: %q", buf.String())
	}
}

func TestComputeTopNotEnoughData(t *testing.T) {
	_, _, err := ComputeTop(makePrices(30, 12), 4, 3, 21, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "TOP-ACT-001") {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := ComputeTop(model.NewPrices(nil, nil), 4, 3, 21, nil, 0); err == nil {
		t.Fatal("sin precios tampoco se puede")
	}
}

func TestComputeTopMarketCapFilter(t *testing.T) {
	p := makePrices(140, 12)
	shares := map[string]float64{}
	for _, tk := range p.Tickers {
		shares[tk] = 1
	}
	shares["T05"] = 1e12
	top, _ := topNames(t, p, 4, 3, &market.Universe{Shares: shares, MinCapBillions: 1})
	if names(top) != "T05" {
		t.Fatalf("top = %v", top)
	}
}

func TestComputeTopReturnsAlternatesAfterThePicks(t *testing.T) {
	ranked, _, err := ComputeTop(makePrices(140, 12), 4, 3, 21, nil, 4)
	if err != nil || len(ranked) != 7 || ranked[3].Ticker != "T08" {
		t.Fatalf("ranked=%v err=%v", ranked, err)
	}
}

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCurrentTickersAppliesAliasesAndDedupes(t *testing.T) {
	d := openTemp(t)
	for _, tk := range []string{"BK", "AAPL", "BNY"} {
		d.Exec("INSERT INTO constituents_current VALUES (?)", tk)
	}
	db.SetMeta(d, "constituents_current_fetched", db.Today()) // descargada hoy: no se vuelve a bajar
	got, err := CurrentTickers(d, map[string]string{"BK": "BNY"}, true, config.Default().Data)
	if err != nil || !reflect.DeepEqual(got, []string{"BNY", "AAPL"}) {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestDiffPicks(t *testing.T) {
	got := DiffPicks([]string{"A", "B", "C"}, []string{"C", "D", "A"})
	if !reflect.DeepEqual(got, Changes{Entered: []string{"D"}, Exited: []string{"B"}, Kept: []string{"C", "A"}}) {
		t.Fatalf("got %+v", got)
	}
}

func TestNextRebalanceStepsFromTheAnchorMonth(t *testing.T) {
	day := dateutil.MustParse
	for _, tc := range []struct {
		today, want string
	}{{"2024-02-01", "2024-05-01"}, {"2024-05-02", "2024-09-01"}, {"2024-05-01", "2024-05-01"}} {
		got, err := NextRebalance("2024-01-10", 4, day(tc.today))
		if err != nil || dateutil.Day(got) != tc.want {
			t.Errorf("hoy %s: %v %v, quiero %s", tc.today, got, err, tc.want)
		}
	}
}

func TestDescribe(t *testing.T) {
	today := dateutil.MustParse("2024-02-01")
	first := Describe("", DiffPicks(nil, []string{"A"}), "2024-01-10", 4, today)
	if !strings.Contains(first[0], "primera corrida") || !strings.Contains(first[1], "2024-05-01") {
		t.Fatalf("primera: %q", first)
	}
	changed := Describe("2024-01-01", DiffPicks([]string{"A", "B"}, []string{"B", "C"}), "", 4, today)
	if !reflect.DeepEqual(changed, []string{"Cambios vs. 2024-01-01: entran C; salen A; se mantienen B."}) {
		t.Fatalf("cambios: %q", changed)
	}
	same := Describe("2024-01-01", DiffPicks([]string{"A"}, []string{"A"}), "", 4, today)
	if !strings.Contains(same[0], "ninguno") {
		t.Fatalf("igual: %q", same)
	}
}

func addPicks(t *testing.T, d *sql.DB, asOf string, tickers []string, months int, cap any) {
	t.Helper()
	for i, tk := range tickers {
		if _, err := d.Exec("INSERT INTO actual_picks VALUES (?,?,?,?,?,?,?,?,?)", asOf+"T10:00:00", asOf, months, cap, i+1, tk, 0.1, 1.0, 1e9); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPreviousPicksIsTheLatestBeforeAsOf(t *testing.T) {
	d := openTemp(t)
	addPicks(t, d, "2024-01-01", []string{"A", "B"}, 4, nil)
	addPicks(t, d, "2024-02-01", []string{"B", "C"}, 4, nil)
	addPicks(t, d, "2024-03-01", []string{"C", "D"}, 4, nil)
	prev, tickers, err := repo.PreviousPicks(d, "2024-03-01", 4, 0)
	if err != nil || prev != "2024-02-01" || !reflect.DeepEqual(tickers, []string{"B", "C"}) {
		t.Fatalf("%q %v %v", prev, tickers, err)
	}
}

func TestPreviousPicksIgnoresOtherUniverseAndTerm(t *testing.T) {
	d := openTemp(t)
	addPicks(t, d, "2024-01-01", []string{"A"}, 4, 200.0)
	addPicks(t, d, "2024-01-01", []string{"B"}, 6, nil)
	if prev, _, _ := repo.PreviousPicks(d, "2024-02-01", 4, 0); prev != "" {
		t.Fatalf("no debía haber previa: %q", prev)
	}
	if prev, tickers, _ := repo.PreviousPicks(d, "2024-02-01", 4, 200); prev != "2024-01-01" || tickers[0] != "A" {
		t.Fatalf("%q %v", prev, tickers)
	}
}

func TestAnchorDateIsTheFirstRun(t *testing.T) {
	d := openTemp(t)
	addPicks(t, d, "2024-02-15", []string{"A"}, 4, nil)
	addPicks(t, d, "2024-01-10", []string{"A"}, 4, nil)
	if got, _ := repo.AnchorDate(d, 4, 0); got != "2024-01-10" {
		t.Fatalf("got %q", got)
	}
	if got, _ := repo.AnchorDate(d, 6, 0); got != "" {
		t.Fatalf("got %q", got)
	}
}

func fakeResult(prices *model.Prices, upTo int) *Result {
	window := prices.Slice(0, upTo).Tail(4*21 + 1)
	cap := 5e11
	return &Result{Months: 4, TopN: 2, AsOf: window.Dates[window.Rows()-1], Window: window,
		Picks:      []repo.Pick{{Rank: 1, Ticker: "T11", ReturnLookback: 0.3, Price: 110, MarketCap: &cap}, {Rank: 2, Ticker: "T10", ReturnLookback: 0.2, Price: 90}},
		Alternates: []repo.Pick{{Rank: 3, Ticker: "T09", ReturnLookback: 0.1, Price: 50}}}
}

func TestSaveTwiceDoesNotDuplicateRowsAndReportsChanges(t *testing.T) {
	d := openTemp(t)
	prices := makePrices(140, 12)
	today := dateutil.MustParse("2025-01-02")
	r := fakeResult(prices, prices.Rows())
	for range 2 {
		if _, err := Save(d, r, today); err != nil {
			t.Fatal(err)
		}
	}
	var n, alts int
	d.QueryRow("SELECT COUNT(*) FROM actual_picks").Scan(&n)
	d.QueryRow("SELECT COUNT(*) FROM actual_alternates").Scan(&alts)
	if n != 2 || alts != 1 {
		t.Fatalf("picks=%d alternates=%d", n, alts)
	}
	// el día siguiente cambia un puesto: la nota lo cuenta contra la corrida anterior
	next := fakeResult(prices, prices.Rows())
	next.AsOf = next.AsOf.AddDate(0, 0, 1)
	next.Picks[1].Ticker = "T08"
	notes, err := Save(d, next, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || !strings.Contains(notes[0], "entran T08; salen T10; se mantienen T11") || !strings.Contains(notes[1], "Próximo rebalanceo") {
		t.Fatalf("notas: %q", notes)
	}
}
