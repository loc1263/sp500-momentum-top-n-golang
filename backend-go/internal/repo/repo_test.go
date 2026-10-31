package repo

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"top3/internal/db"
	"top3/internal/model"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func str(s string) *string { return &s }

func TestSharesAndFundamentalsCacheRespectTheCutoff(t *testing.T) {
	d := openTemp(t)
	must(t, SaveShares(d, map[string]float64{"OLD": 10, "NEW": 20}, "2026-01-01"))
	must(t, SaveShares(d, map[string]float64{"NEW": 25}, "2026-06-01")) // REPLACE: NEW queda con la fecha nueva
	got, err := FreshShares(d, "2026-03-01")
	must(t, err)
	if !reflect.DeepEqual(got, map[string]float64{"NEW": 25}) {
		t.Fatalf("FreshShares: %v", got)
	}
	n, err := DeleteSharesBefore(d, "2026-03-01")
	must(t, err)
	if n != 1 {
		t.Errorf("DeleteSharesBefore borró %d, quiero 1", n)
	}

	must(t, SaveFundamentals(d, map[string]string{"A": `{"v":1}`, "B": `{"v":2}`}, "2026-01-01"))
	must(t, SaveFundamentals(d, map[string]string{"B": `{"v":3}`}, "2026-06-01"))
	fresh, err := FreshFundamentals(d, "2026-03-01")
	must(t, err)
	if !reflect.DeepEqual(fresh, map[string]string{"B": `{"v":3}`}) {
		t.Fatalf("FreshFundamentals: %v", fresh)
	}
	n, err = DeleteFundamentalsBefore(d, "2026-03-01")
	must(t, err)
	if n != 1 {
		t.Errorf("DeleteFundamentalsBefore borró %d, quiero 1", n)
	}
}

func TestConstituentsReplaceAndRead(t *testing.T) {
	d := openTemp(t)
	must(t, ReplaceHistory(d, []HistoryRow{{"2020-02-01", "B,C"}, {"2020-01-01", "A,B"}}))
	must(t, ReplaceHistory(d, []HistoryRow{{"2021-02-01", "B,C"}, {"2021-01-01", "A,B"}})) // reemplaza, no suma
	h, err := History(d)
	must(t, err)
	if len(h) != 2 || h[0].Date != "2021-01-01" || h[1].Tickers != "B,C" { // ordenada por fecha
		t.Fatalf("History: %+v", h)
	}

	must(t, ReplaceCurrent(d, []string{"A", "B", "A"})) // un duplicado no rompe
	cur, err := Current(d)
	must(t, err)
	if len(cur) != 2 {
		t.Fatalf("Current: %v", cur)
	}

	must(t, ReplaceChanges(d, []IndexChange{
		{Date: "2026-03-01", Added: str("X"), Removed: str("Y")},
		{Date: "2026-02-01", Added: str("Z")}, // sin baja ni motivo: NULL
		{Date: "2025-12-01", Removed: str("W")},
	}))
	ch, err := ChangesAfter(d, "2026-01-01")
	must(t, err)
	want := []ChangeAfter{{"2026-02-01", "Z", ""}, {"2026-03-01", "X", "Y"}} // posteriores al día y ordenadas; NULL = ""
	if !reflect.DeepEqual(ch, want) {
		t.Fatalf("ChangesAfter: %+v", ch)
	}
}

func TestIpsaKeepsOnlyTheLatestRun(t *testing.T) {
	d := openTemp(t)
	if got, err := LatestIpsa(d); err != nil || got != nil {
		t.Fatalf("sin corridas: %v %v", got, err)
	}
	must(t, SaveLatestIpsa(d, "2026-10-01T10:00:00", "2026-10-01", json.RawMessage(`{"n":1}`)))
	must(t, SaveLatestIpsa(d, "2026-10-02T10:00:00", "2026-10-02", json.RawMessage(`{"n":2}`)))
	got, err := LatestIpsa(d)
	must(t, err)
	if string(got) != `{"n":2}` {
		t.Errorf("LatestIpsa: %s", got)
	}
	var n int
	must(t, d.QueryRow("SELECT COUNT(*) FROM ipsa_runs").Scan(&n))
	if n != 1 {
		t.Errorf("quedaron %d corridas, quiero 1", n)
	}
}

func TestOHLCVFramesFromRowsAlignsFieldsAndSkipsNaN(t *testing.T) {
	nan := math.NaN()
	f := OHLCVFramesFromRows([]OHLCVRow{
		{Ticker: "A", Date: "2026-01-02", Open: 1, High: 3, Low: 0.5, Close: 2, Volume: 100},
		{Ticker: "A", Date: "2026-01-03", Open: nan, High: nan, Low: nan, Close: 4, Volume: nan}, // solo cierre
		{Ticker: "B", Date: "2026-01-02", Open: 5, High: 6, Low: 4, Close: 5.5, Volume: 50},
	})
	for name, p := range map[string]*model.Prices{"open": f.Open, "high": f.High, "low": f.Low, "volume": f.Volume} {
		if p.Rows() != f.Close.Rows() || p.Cols() != f.Close.Cols() {
			t.Errorf("%s no está alineada con el cierre", name)
		}
	}
	ja, _ := f.Close.Col("A")
	if !math.IsNaN(f.Open.At(1, ja)) || f.Close.At(1, ja) != 4 {
		t.Error("el día sin apertura debe quedar en NaN y con cierre")
	}
	if f.High.At(0, ja) != 3 {
		t.Error("máximo de A el primer día")
	}
}

func TestEtfScoresReplaceByDayAndPreviousTop(t *testing.T) {
	d := openTemp(t)
	row := func(rank int, tk string, score float64) EtfScoreRow {
		return EtfScoreRow{Rank: rank, Ticker: tk, Score: score, Vol: 0.1, MaxDD: -0.2, SMADist: 0.03, Returns: map[string]float64{"3": 0.05}}
	}
	must(t, ReplaceEtfScores(d, "2026-09-30", []EtfScoreRow{row(1, "AAA", 2), row(2, "BBB", 1)}))
	must(t, ReplaceEtfScores(d, "2026-10-31", []EtfScoreRow{row(1, "BBB", 3), row(2, "AAA", 2)}))
	must(t, ReplaceEtfScores(d, "2026-10-31", []EtfScoreRow{row(1, "CCC", 4)})) // repetir el día lo reemplaza

	if got, err := LatestEtfAsOf(d); err != nil || got != "2026-10-31" {
		t.Errorf("LatestEtfAsOf: %q", got)
	}
	r, err := EtfRanking(d, "2026-10-31")
	must(t, err)
	if len(r) != 1 || r[0].Ticker != "CCC" || r[0].Returns["3"] != 0.05 || r[0].RunAt == "" {
		t.Fatalf("EtfRanking: %+v", r)
	}
	day, top, err := PreviousEtfTop(d, "2026-10-31", 1)
	must(t, err)
	if day != "2026-09-30" || !reflect.DeepEqual(top, []string{"AAA"}) {
		t.Errorf("PreviousEtfTop: %q %v", day, top)
	}
	if day, top, err := PreviousEtfTop(d, "2026-09-30", 5); err != nil || day != "" || top != nil {
		t.Errorf("sin anterior: %q %v %v", day, top, err)
	}
	empty := openTemp(t)
	if got, err := LatestEtfAsOf(empty); err != nil || got != "" {
		t.Error("base vacía")
	}
}

func TestPicksAreKeyedByMonthsAndCap(t *testing.T) {
	d := openTemp(t)
	mc := 5e9
	pk := func(rank int, tk string) Pick {
		return Pick{Rank: rank, Ticker: tk, ReturnLookback: 0.1, Price: 10, MarketCap: &mc}
	}
	// índice completo (cap 0 = NULL) y universo de cap >= 10 conviven con el mismo plazo
	must(t, ReplacePicks(d, "2026-09-01T10:00:00", "2026-09-01", 4, 0, []Pick{pk(1, "A"), pk(2, "B")}, []Pick{pk(3, "C")}))
	must(t, ReplacePicks(d, "2026-09-01T10:00:00", "2026-09-01", 4, 10, []Pick{pk(1, "X")}, nil))
	must(t, ReplacePicks(d, "2026-10-01T10:00:00", "2026-10-01", 4, 0, []Pick{{Rank: 1, Ticker: "D", Price: 7}}, []Pick{pk(2, "E")}))
	must(t, ReplacePicks(d, "2026-10-01T11:00:00", "2026-10-01", 4, 0, []Pick{{Rank: 1, Ticker: "F", Price: 8}}, nil)) // misma fecha: reemplaza

	day, prev, err := PreviousPicks(d, "2026-10-01", 4, 0)
	must(t, err)
	if day != "2026-09-01" || !reflect.DeepEqual(prev, []string{"A", "B"}) {
		t.Errorf("PreviousPicks cap 0: %q %v", day, prev)
	}
	if _, prev, _ := PreviousPicks(d, "2026-10-01", 4, 10); !reflect.DeepEqual(prev, []string{"X"}) {
		t.Errorf("PreviousPicks cap 10: %v", prev)
	}
	if day, prev, _ := PreviousPicks(d, "2026-09-01", 4, 0); day != "" || prev != nil {
		t.Errorf("sin anterior: %q %v", day, prev)
	}
	if a, err := AnchorDate(d, 4, 0); err != nil || a != "2026-09-01" {
		t.Errorf("AnchorDate: %q %v", a, err)
	}
	if a, _ := AnchorDate(d, 6, 0); a != "" {
		t.Errorf("AnchorDate de otro plazo: %q", a)
	}
	if at, err := LatestPickRunAt(d); err != nil || at != "2026-10-01T11:00:00" {
		t.Errorf("LatestPickRunAt: %q %v", at, err)
	}

	rows, err := PicksOfDay(d, "2026-09-01")
	must(t, err)
	if len(rows) != 3 || rows[0].Ticker != "X" && rows[0].Ticker != "A" || rows[0].MarketCap == nil {
		t.Fatalf("PicksOfDay: %+v", rows)
	}
	capOf := map[string]float64{}
	for _, r := range rows {
		capOf[r.Ticker] = r.Cap
	}
	if capOf["A"] != 0 || capOf["X"] != 10 {
		t.Errorf("Cap: %v", capOf)
	}
	alts, err := Alternates(d, "2026-09-01T10:00:00", 4, 0)
	must(t, err)
	if len(alts) != 1 || alts[0].Ticker != "C" || alts[0].MarketCap == nil {
		t.Errorf("Alternates: %+v", alts)
	}

	by, err := LatestPicksByDate(d, 0, 4)
	must(t, err)
	if len(by) != 2 || by[0].AsOf != "2026-09-01" || len(by[0].Picks) != 2 ||
		by[1].AsOf != "2026-10-01" || len(by[1].Picks) != 1 || by[1].Picks[0].Ticker != "F" { // de la misma fecha, la última corrida
		t.Errorf("LatestPicksByDate: %+v", by)
	}
}

func TestPricesRoundTrip(t *testing.T) {
	d := openTemp(t)
	if last, err := LatestPriceDate(d); HasPrices(d) || err != nil || last != "" {
		t.Fatal("base vacía")
	}
	must(t, SavePrices(d, []model.PriceRow{
		{Ticker: "A", Date: "2026-01-02", Close: 10}, {Ticker: "A", Date: "2026-01-05", Close: 11},
		{Ticker: "B", Date: "2026-01-02", Close: 20}, {Ticker: "C", Date: "2025-12-01", Close: 30},
	}))
	must(t, SavePrices(d, []model.PriceRow{{Ticker: "A", Date: "2026-01-05", Close: 12}})) // REPLACE
	if last, err := LatestPriceDate(d); !HasPrices(d) || err != nil || last != "2026-01-05" {
		t.Error("HasPrices o LatestPriceDate")
	}
	last, err := LastDates(d)
	must(t, err)
	if last["A"] != "2026-01-05" || last["B"] != "2026-01-02" || last["C"] != "2025-12-01" {
		t.Errorf("LastDates: %v", last)
	}

	// el cierre en el día o el último anterior; un ticker sin precios no figura
	cl, err := ClosesOnOrBefore(d, []string{"A", "B", "ZZZ"}, "2026-01-04")
	must(t, err)
	if !reflect.DeepEqual(cl, map[string]float64{"A": 10, "B": 20}) {
		t.Errorf("ClosesOnOrBefore: %v", cl)
	}
	if cl, err := ClosesOnOrBefore(d, nil, "2026-01-04"); err != nil || len(cl) != 0 {
		t.Errorf("sin tickers: %v %v", cl, err)
	}

	since, err := StoredSince(d, "2026-01-01")
	must(t, err)
	if len(since) != 3 || since[[2]string{"A", "2026-01-05"}] != 12 {
		t.Errorf("StoredSince: %v", since)
	}
	all, err := PriceMatrix(d)
	must(t, err)
	from, err := PriceMatrixSince(d, "2026-01-01")
	must(t, err)
	if all.Rows() != 3 || all.Cols() != 3 || from.Rows() != 2 || from.Cols() != 2 {
		t.Errorf("matrices: %dx%d y %dx%d", all.Rows(), all.Cols(), from.Rows(), from.Cols())
	}

	n, err := DeletePricesBefore(d, "2026-01-01")
	must(t, err)
	if n != 1 {
		t.Errorf("DeletePricesBefore borró %d, quiero 1", n)
	}
	must(t, DeletePriceTickers(d, []string{"B"}))
	if got, _ := LastDates(d); len(got) != 1 {
		t.Errorf("tras borrar B: %v", got)
	}
	must(t, ClearPrices(d))
	if HasPrices(d) {
		t.Error("ClearPrices")
	}
}

func TestNoDataTickersExpire(t *testing.T) {
	d := openTemp(t)
	must(t, RememberNoData(d, []string{"GONE", "BACK"}, nil))
	must(t, RememberNoData(d, nil, []string{"BACK"})) // BACK ahora sí tiene datos: se olvida
	known, err := KnownNoData(d, 30)
	must(t, err)
	if !reflect.DeepEqual(known, map[string]bool{"GONE": true}) {
		t.Errorf("KnownNoData: %v", known)
	}
	if k, _ := KnownNoData(d, 0); len(k) != 0 { // sin TTL no se recuerda nada
		t.Errorf("TTL 0: %v", k)
	}
	n, err := ForgetNoDataBefore(d, db.Today())
	must(t, err)
	if n != 1 {
		t.Errorf("ForgetNoDataBefore borró %d, quiero 1", n)
	}
}

func TestOHLCVStoresNaNAsNull(t *testing.T) {
	d := openTemp(t)
	nan := math.NaN()
	must(t, ReplaceOHLCV(d, []OHLCVRow{
		{Ticker: "A", Date: "2026-01-02", Open: 1, High: 3, Low: 0.5, Close: 2, Volume: 100},
		{Ticker: "A", Date: "2026-01-03", Open: nan, High: nan, Low: nan, Close: 4, Volume: nan},
	}))
	must(t, ReplaceOHLCV(d, []OHLCVRow{ // reemplaza todo lo anterior
		{Ticker: "A", Date: "2026-01-02", Open: 1, High: 3, Low: 0.5, Close: 2, Volume: 100},
		{Ticker: "A", Date: "2026-01-03", Open: nan, High: nan, Low: nan, Close: 4, Volume: nan},
	}))
	f, err := LoadOHLCV(d)
	must(t, err)
	ja, _ := f.Close.Col("A")
	if f.Close.Rows() != 2 || f.Volume.At(0, ja) != 100 || !math.IsNaN(f.Volume.At(1, ja)) || !math.IsNaN(f.Open.At(1, ja)) {
		t.Errorf("LoadOHLCV: cierre %d filas, volumen %v / %v", f.Close.Rows(), f.Volume.At(0, ja), f.Volume.At(1, ja))
	}
	var n int
	must(t, d.QueryRow("SELECT COUNT(*) FROM ohlcv WHERE volume IS NULL").Scan(&n))
	if n != 1 {
		t.Errorf("filas con volumen NULL: %d, quiero 1", n)
	}
}
