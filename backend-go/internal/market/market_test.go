package market

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loc1263/gohoo"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/repo"
	"top3/internal/yahoo"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "sub", "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestParseTickers(t *testing.T) {
	if got := ParseTickers("['A', 'BRK.B', ' C ']"); !reflect.DeepEqual(got, []string{"A", "BRK-B", "C"}) {
		t.Fatalf("got %v", got)
	}
	if got := ParseTickers(""); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func seed(t *testing.T, d *sql.DB, history [][2]string, current []string, changes [][3]string) {
	t.Helper()
	for _, h := range history {
		if _, err := d.Exec("INSERT INTO constituents_history VALUES (?, ?)", h[0], h[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range current {
		if _, err := d.Exec("INSERT INTO constituents_current VALUES (?)", c); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range changes {
		if _, err := d.Exec("INSERT INTO index_changes VALUES (?, ?, ?, '')", c[0], c[1], c[2]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConstituentsOnDateAndAliases(t *testing.T) {
	d := openTemp(t)
	seed(t, d, [][2]string{{"2020-01-01", "A,BK,C"}, {"2021-01-01", "A,D"}}, []string{"Z"}, nil)
	c, err := NewConstituents(d, map[string]string{"BK": "BNY"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		day  string
		want []string
	}{
		{"2020-06-01", []string{"A", "BNY", "C"}},
		{"2021-01-01", []string{"A", "D"}},
		{"2019-01-01", nil},           // antes del primer dato
		{"2030-01-01", []string{"Z"}}, // después del último: lista actual
	} {
		if got := c.OnDate(tc.day); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("OnDate(%s) = %v, quiero %v", tc.day, got, tc.want)
		}
	}
}

func TestConstituentsApplyChanges(t *testing.T) {
	d := openTemp(t)
	seed(t, d, [][2]string{{"2020-01-01", "A,B"}}, nil, [][3]string{{"2020-06-01", "C", "A"}, {"2019-01-01", "X", "B"}})
	c, _ := NewConstituents(d, nil)
	if !reflect.DeepEqual(c.dates, []string{"2020-01-01", "2020-06-01"}) { // el cambio anterior al CSV se ignora
		t.Fatalf("dates = %v", c.dates)
	}
	if got := c.OnDate("2020-07-01"); !reflect.DeepEqual(got, []string{"B", "C"}) {
		t.Fatalf("got %v", got)
	}
}

func TestConstituentsAllBetween(t *testing.T) {
	d := openTemp(t)
	seed(t, d, [][2]string{{"2020-01-01", "A,B"}, {"2021-01-01", "C"}, {"2022-01-01", "D"}}, []string{"E"}, nil)
	c, _ := NewConstituents(d, nil)
	if got := c.AllBetween("2020-06-01", "2021-06-01"); !reflect.DeepEqual(got, []string{"A", "B", "C", "E"}) {
		t.Fatalf("got %v", got)
	}
}

func TestConstituentsWithoutHistoryUseCurrent(t *testing.T) {
	d := openTemp(t)
	seed(t, d, nil, []string{"X", "Y"}, nil)
	c, _ := NewConstituents(d, nil)
	if got := c.OnDate("2024-01-01"); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got := c.AllBetween("2020-01-01", "2024-01-01"); !reflect.DeepEqual(got, []string{"X", "Y"}) {
		t.Fatalf("got %v", got)
	}
}

func TestUniverseSelect(t *testing.T) {
	u := &Universe{Shares: map[string]float64{"A": 2e9, "B": 5e8, "C": 1e12, "E": 1e12}, MinCapBillions: 1}
	if u.MinCapUSD() != 1e9 {
		t.Fatal("MinCapUSD")
	}
	got := u.Select([]string{"A", "B", "C", "D", "E"}, []float64{1, 1, 1, 1, math.NaN()})
	if !reflect.DeepEqual(got, []string{"A", "C"}) { // B muy chica; D sin acciones; E sin precio
		t.Fatalf("got %v", got)
	}
}

func TestUniverseLabels(t *testing.T) {
	for cap, suffix := range map[float64]string{0: "", 5: "_cap5B", 10: "_largecap", 200: "_megacap"} {
		if Suffix(cap) != suffix {
			t.Errorf("Suffix(%g) = %q", cap, Suffix(cap))
		}
	}
	if Label(0) != "índice completo" || Label(200) != "mega-cap" || Label(10) != "large-cap + mega-cap" ||
		!strings.HasPrefix(Label(5), "cap.") || Title(0) != "" {
		t.Fatal("etiquetas")
	}
	if cap, ok := ExplicitMinCap(config.Default(), true, nil); !ok || cap != 10 {
		t.Fatal("--large-cap")
	}
	zero := 0.0
	if cap, ok := ExplicitMinCap(config.Default(), false, &zero); !ok || cap != 0 {
		t.Fatal("--min-cap-billions 0 es explícito (índice completo)")
	}
	if _, ok := ExplicitMinCap(config.Default(), false, nil); ok {
		t.Fatal("sin flags no hay filtro")
	}
}

const changesPage = `<html><body>
<table><tr><th>Otra</th></tr><tr><td>x</td></tr></table>
<table>
<thead>
<tr><th rowspan="2">Date</th><th colspan="2">Added</th><th colspan="2">Removed</th><th rowspan="2">Reason</th></tr>
<tr><th>Ticker</th><th>Security</th><th>Ticker</th><th>Security</th></tr>
</thead>
<tbody>
<tr><td>October 6, 2026</td><td>NEW.B</td><td>Nueva</td><td>OLD</td><td>Vieja</td><td>Reemplazo<sup>[1]</sup></td></tr>
<tr><td rowspan="2">June 1, 2026</td><td>AAA</td><td>A</td><td></td><td></td><td rowspan="2">Fusión</td></tr>
<tr><td></td><td></td><td>BBB</td><td>B</td></tr>
<tr><td>fecha rota</td><td>X</td><td>x</td><td>Y</td><td>y</td><td>?</td></tr>
</tbody></table></body></html>`

func TestParseChanges(t *testing.T) {
	rows, bad, err := parseChanges(changesPage)
	if err != nil {
		t.Fatal(err)
	}
	if bad != 1 || len(rows) != 3 {
		t.Fatalf("bad=%d rows=%+v", bad, rows)
	}
	first := rows[0]
	if first.Date != "2026-10-06" || *first.Added != "NEW-B" || *first.Removed != "OLD" || *first.Reason != "Reemplazo" {
		t.Fatalf("fila 1: %+v", first)
	}
	// rowspan: la fecha y el motivo se repiten en la fila siguiente; las celdas vacías son nulas
	if rows[1].Date != "2026-06-01" || *rows[1].Added != "AAA" || rows[1].Removed != nil {
		t.Fatalf("fila 2: %+v", rows[1])
	}
	if rows[2].Date != "2026-06-01" || rows[2].Added != nil || *rows[2].Removed != "BBB" || *rows[2].Reason != "Fusión" {
		t.Fatalf("fila 3: %+v", rows[2])
	}
	if _, _, err := parseChanges("<table><tr><td>nada</td></tr></table>"); err == nil || !strings.Contains(err.Error(), "TOP-NET-004") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseCurrent(t *testing.T) {
	page := `<table><tr><th>Symbol</th><th>Security</th></tr><tr><td>BRK.B</td><td>Berkshire</td></tr><tr><td>AAPL</td><td>Apple</td></tr></table>`
	got, err := parseCurrent(page)
	if err != nil || !reflect.DeepEqual(got, []string{"BRK-B", "AAPL"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

// fakeYahoo sirve el chart diario de cada ticker desde series (fecha -> cierre ajustado).
type fakeYahoo struct {
	mu     sync.Mutex
	series map[string]map[string]float64
	calls  map[string]int
	broken map[string]bool // tickers para los que Yahoo responde con un error del servidor
}

func (f *fakeYahoo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticker := strings.TrimPrefix(r.URL.Path, "/v8/finance/chart/")
	f.mu.Lock()
	f.calls[ticker]++
	s, ok := f.series[ticker]
	broken := f.broken[ticker]
	f.mu.Unlock()
	if broken {
		w.WriteHeader(500)
		return
	}
	if !ok {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)
		return
	}
	from, to := int64(0), int64(1<<62)
	if v := r.URL.Query().Get("period1"); v != "" {
		fmt.Sscan(v, &from)
	}
	if v := r.URL.Query().Get("period2"); v != "" {
		fmt.Sscan(v, &to)
	}
	var days []string
	for day := range s {
		days = append(days, day)
	}
	sortStrings(days)
	var ts []int64
	var closes []float64
	for _, day := range days {
		at := dateutil.MustParse(day).Add(14*time.Hour + 30*time.Minute).Unix()
		if at >= from && at < to {
			ts = append(ts, at)
			closes = append(closes, s[day])
		}
	}
	body, _ := json.Marshal(map[string]any{"chart": map[string]any{"error": nil, "result": []any{map[string]any{
		"meta":       map[string]any{"symbol": ticker, "currency": "USD", "instrumentType": "EQUITY", "exchangeTimezoneName": "UTC"},
		"timestamp":  ts,
		"indicators": map[string]any{"quote": []any{map[string]any{"open": closes, "high": closes, "low": closes, "close": closes, "volume": closes}}, "adjclose": []any{map[string]any{"adjclose": closes}}},
	}}}})
	w.Write(body)
}

func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func newDownloader(t *testing.T, f *fakeYahoo) *Downloader {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	y, err := yahoo.New("", gohoo.WithBaseURL(srv.URL), gohoo.WithRetryPolicy(gohoo.RetryPolicy{MaxAttempts: 1}))
	if err != nil {
		t.Fatal(err)
	}
	return &Downloader{Y: y, Workers: 4}
}

func daily(start string, n int, base, step float64) map[string]float64 {
	out := map[string]float64{}
	d := dateutil.MustParse(start)
	for i := 0; i < n; i++ {
		out[dateutil.Day(d)] = base + step*float64(i)
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func TestFetchClassifiesGoneAndKeepsRowsInRange(t *testing.T) {
	f := &fakeYahoo{series: map[string]map[string]float64{"AAA": daily("2024-01-01", 10, 100, 1)}, calls: map[string]int{}}
	dl := newDownloader(t, f)
	res, err := dl.Fetch(context.Background(), []string{"AAA", "GONE"}, "2024-01-03", "2024-01-08", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 5 || res.Rows[0].Date != "2024-01-03" || res.Rows[4].Date != "2024-01-07" { // fin exclusivo
		t.Fatalf("rows = %+v", res.Rows)
	}
	if !reflect.DeepEqual(res.NoData, []string{"GONE"}) || !res.Gone["GONE"] {
		t.Fatalf("NoData=%v Gone=%v", res.NoData, res.Gone)
	}
}

func TestUpdatePricesRefetchesOnlyTickersWithChangedAdjustment(t *testing.T) {
	d := openTemp(t)
	s := config.Default().Data
	f := &fakeYahoo{series: map[string]map[string]float64{
		"STABLE":   daily("2024-01-01", 30, 100, 1),
		"SPLIT":    daily("2024-01-01", 30, 50, 0.5),
		"NEWCOMER": daily("2024-01-01", 30, 10, 0.1),
	}, calls: map[string]int{}}
	dl := newDownloader(t, f)

	// Lo guardado llega hasta el 20 de enero. STABLE coincide con Yahoo; SPLIT guardó el doble de lo que Yahoo dice hoy
	// (pagó dividendo o hizo split desde entonces: cambió su base de ajuste).
	for day := range daily("2024-01-01", 20, 0, 0) {
		saveRows(t, d, []repoRow{{"STABLE", day, f.series["STABLE"][day]}, {"SPLIT", day, 2 * f.series["SPLIT"][day]}})
	}
	err := dl.UpdatePrices(context.Background(), d, []string{"STABLE", "SPLIT", "NEWCOMER"}, "2024-01-01", "2024-02-01", s)
	if err != nil {
		t.Fatal(err)
	}
	last, _ := repo.LastDates(d)
	for _, tk := range []string{"STABLE", "SPLIT", "NEWCOMER"} {
		if last[tk] != "2024-01-30" {
			t.Fatalf("%s llega a %s: %v", tk, last[tk], last)
		}
	}
	closes, _ := repo.ClosesOnOrBefore(d, []string{"SPLIT", "STABLE"}, "2024-01-05")
	if closes["SPLIT"] != f.series["SPLIT"]["2024-01-05"] {
		t.Fatalf("SPLIT debió bajarse completo con los precios nuevos: %v", closes)
	}
	// STABLE: una sola descarga corta (solape); SPLIT: la corta y la completa; NEWCOMER: solo la completa
	if f.calls["STABLE"] != 1 || f.calls["SPLIT"] != 2 || f.calls["NEWCOMER"] != 1 {
		t.Fatalf("llamadas: %v", f.calls)
	}
	if v, _ := db.GetMeta(d, "prices_fetched"); v != db.Today() {
		t.Fatalf("prices_fetched = %q", v)
	}
}

type repoRow = struct {
	Ticker, Date string
	Close        float64
}

func saveRows(t *testing.T, d *sql.DB, rows []repoRow) {
	t.Helper()
	for _, r := range rows {
		if _, err := d.Exec("INSERT OR REPLACE INTO prices VALUES (?, ?, ?)", r.Ticker, r.Date, r.Close); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDownloadPricesKeepsWhatIsStoredWhenNothingCanBeDownloaded(t *testing.T) {
	d := openTemp(t)
	s := config.Default().Data
	saveRows(t, d, []repoRow{{"AAA", "2024-01-02", 10}, {"AAA", "2024-01-03", 11}})
	f := &fakeYahoo{series: map[string]map[string]float64{}, calls: map[string]int{}, broken: map[string]bool{"AAA": true}}
	dl := newDownloader(t, f)
	err := dl.DownloadPrices(context.Background(), d, []string{"AAA"}, "2024-01-01", "2024-02-01", s)
	if err == nil || !strings.Contains(err.Error(), "TOP-NET-007") {
		t.Fatalf("err = %v", err)
	}
	if !repo.HasPrices(d) {
		t.Fatal("se borraron los precios guardados")
	}
}

func TestUpdatePricesDoesNotDeleteTickersThatFailedForTransientReasons(t *testing.T) {
	d := openTemp(t)
	s := config.Default().Data
	f := &fakeYahoo{series: map[string]map[string]float64{"OK": daily("2024-01-01", 30, 100, 1)},
		calls: map[string]int{}, broken: map[string]bool{"FLAKY": true}}
	dl := newDownloader(t, f)
	for day := range daily("2024-01-01", 20, 0, 0) {
		saveRows(t, d, []repoRow{{"OK", day, f.series["OK"][day]}, {"FLAKY", day, 5}})
	}
	err := dl.UpdatePrices(context.Background(), d, []string{"OK", "FLAKY"}, "2024-01-01", "2024-02-01", s)
	if err != nil {
		t.Fatal(err)
	}
	last, _ := repo.LastDates(d)
	if last["OK"] != "2024-01-30" || last["FLAKY"] != "2024-01-20" { // FLAKY conserva lo que tenía
		t.Fatalf("last = %v", last)
	}
}

func totalCalls(f *fakeYahoo) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// Los tickers que Yahoo no informa se anotan por el día: la segunda corrida no vuelve a pedirlos, salvo con --no-cache.
func TestEnsureSharesRemembersTickersWithoutDataForTheDay(t *testing.T) {
	d := openTemp(t)
	f := &fakeYahoo{series: map[string]map[string]float64{}, calls: map[string]int{}}
	dl := newDownloader(t, f)
	s := config.Default().Data
	ctx := context.Background()

	if _, err := dl.EnsureShares(ctx, d, []string{"GONE1", "GONE2"}, true, s, nil); err != nil {
		t.Fatal(err)
	}
	first := totalCalls(f)
	if first == 0 {
		t.Fatal("la primera corrida debería pedirlos a Yahoo")
	}
	got, err := dl.EnsureShares(ctx, d, []string{"GONE1", "GONE2"}, true, s, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
	if n := totalCalls(f); n != first {
		t.Fatalf("la segunda corrida hizo %d requests más", n-first)
	}
	if _, err := dl.EnsureShares(ctx, d, []string{"GONE1"}, false, s, nil); err != nil { // --no-cache
		t.Fatal(err)
	}
	if totalCalls(f) == first {
		t.Fatal("con --no-cache sí se vuelven a pedir")
	}
}
