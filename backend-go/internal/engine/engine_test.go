package engine

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"top3/internal/dateutil"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/testutil"
)

func run(t *testing.T, cons Members, p *model.Prices, start, end string, params Params) (model.Result, error) {
	t.Helper()
	if params.StepMonths == 0 {
		params.StepMonths = 3
	}
	if params.LookbackDays == 0 {
		params.LookbackDays = 63
	}
	if params.InitialCapital == 0 {
		params.InitialCapital = 1000
	}
	if params.TopN == 0 {
		params.TopN = 3
	}
	return Run(cons, NewMatrix(p), dateutil.MustParse(start), dateutil.MustParse(end), params)
}

func tickersOf(p *model.Prices) testutil.Members { return testutil.Members{Tickers: p.Tickers} }

func TestRunPicksBestPerformers(t *testing.T) {
	prices := testutil.DefaultPrices()
	res, err := run(t, tickersOf(prices), prices, "2023-09-01", "2024-12-31", Params{})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range res.History {
		var got []string
		for _, p := range h.Picks {
			got = append(got, p.Ticker)
		}
		if !reflect.DeepEqual(got, []string{"T11", "T10", "T09"}) {
			t.Fatalf("%s: %v", h.Date, got)
		}
	}
	if res.FinalCapital <= 1000 || res.TotalReturnPct <= 0 || res.CAGRPct <= 0 {
		t.Fatalf("resultado: %+v", res)
	}
	if last := res.Equity[len(res.Equity)-1].Capital; math.Abs(last-res.FinalCapital) > 1e-9 {
		t.Fatalf("la curva termina en %v y el capital final es %v", last, res.FinalCapital)
	}
}

func TestRunMarketCapFilterLeavesTooFewTickers(t *testing.T) {
	prices := testutil.DefaultPrices()
	shares := map[string]float64{}
	for _, tk := range prices.Tickers {
		shares[tk] = 1
		if tk == "T00" || tk == "T01" || tk == "T02" {
			shares[tk] = 1e12
		}
	}
	// solo 3 tickers superan el umbral -> menos de 10 candidatos -> no hay rebalanceos
	_, err := run(t, tickersOf(prices), prices, "2023-09-01", "2024-12-31",
		Params{Universe: &market.Universe{Shares: shares, MinCapBillions: 1}})
	if err == nil || !strings.Contains(err.Error(), "TOP-BKT-001") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWithoutConstituentsFails(t *testing.T) {
	prices := testutil.DefaultPrices()
	if _, err := run(t, testutil.Members{}, prices, "2023-09-01", "2024-12-31", Params{}); err == nil {
		t.Fatal("se esperaba un error")
	}
}

func TestEligibleTickersRequiresPricesAndCap(t *testing.T) {
	prices := testutil.MakePrices("2023-01-02", "2024-12-31", 3)
	m := NewMatrix(prices)
	when := dateutil.MustParse("2024-01-02")
	if got := EligibleTickers([]string{"T00", "NOPE", "T02"}, m, when, nil); !reflect.DeepEqual(got, []string{"T00", "T02"}) {
		t.Fatalf("got %v", got)
	}
	u := &market.Universe{Shares: map[string]float64{"T00": 1e12, "T02": 1}, MinCapBillions: 1}
	if got := EligibleTickers([]string{"T00", "NOPE", "T02"}, m, when, u); !reflect.DeepEqual(got, []string{"T00"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRankReturnsOrdersAndRequiresHistory(t *testing.T) {
	prices := testutil.MakePrices("2023-01-02", "2024-12-31", 4)
	m := NewMatrix(prices)
	got, ok := RankReturns(m, prices.Tickers, dateutil.MustParse("2024-06-03"), 21, nil, 0)
	if !ok || got[0].Ticker != "T03" || got[3].Ticker != "T00" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := RankReturns(m, prices.Tickers, dateutil.MustParse("2023-01-10"), 21, nil, 0); ok {
		t.Fatal("con pocas ruedas no se puede rankear")
	}
}

func TestRankReturnsSkipDaysIgnoresTheMostRecentRows(t *testing.T) {
	dates := testutil.BDaysN("2024-01-01", 30)
	p := model.NewPrices(dates, []string{"UP_LATE", "UP_EARLY"})
	for k := 0; k < 30; k++ {
		late, early := 100.0, 195.0
		if k >= 20 {
			late = 100 + 10*float64(k-19)
		}
		if k < 20 {
			early = 100 + 5*float64(k)
		}
		p.Px[k*2], p.Px[k*2+1] = late, early
	}
	m := NewMatrix(p)
	last := dates[len(dates)-1]
	order := func(skip int) []string {
		r, ok := RankReturns(m, p.Tickers, last, 9, nil, skip)
		if !ok {
			return nil
		}
		var out []string
		for _, x := range r {
			out = append(out, x.Ticker)
		}
		return out
	}
	if got := order(0); !reflect.DeepEqual(got, []string{"UP_LATE", "UP_EARLY"}) {
		t.Fatalf("sin skip: %v", got)
	}
	if got := order(10); !reflect.DeepEqual(got, []string{"UP_EARLY", "UP_LATE"}) {
		t.Fatalf("con skip: %v", got)
	}
	if _, ok := RankReturns(m, p.Tickers, last, 25, nil, 10); ok { // 26 + 10 ruedas > 30
		t.Fatal("falta historia con skip")
	}
}

type fewEarly struct{ tickers []string }

func (f fewEarly) OnDate(day string) []string {
	if day < "2024-01-01" {
		return f.tickers[:5]
	}
	return f.tickers
}

func TestRunSkippedCollectsDatesInsteadOfWarning(t *testing.T) {
	prices := testutil.DefaultPrices()
	skipped := Skipped{}
	if _, err := run(t, fewEarly{prices.Tickers}, prices, "2023-09-01", "2024-12-31", Params{Skipped: skipped}); err != nil {
		t.Fatal(err)
	}
	got := skipped[errs.BktFewTickers]
	if len(skipped) != 1 || len(got) != 2 || !got["2023-09-01"] || !got["2023-12-01"] {
		t.Fatalf("skipped = %v", skipped)
	}
}

func TestCommissionChargesEntryAndOnlyPositionsThatChange(t *testing.T) {
	prices := testutil.DefaultPrices()
	free, _ := run(t, tickersOf(prices), prices, "2023-09-01", "2024-12-31", Params{})
	paid, _ := run(t, tickersOf(prices), prices, "2023-09-01", "2024-12-31", Params{Commission: 0.001})
	// la selección no cambia nunca: solo se paga la compra inicial
	if want := free.FinalCapital * (1 - 0.001); math.Abs(paid.FinalCapital-want) > 1e-9 {
		t.Fatalf("pagado %v, esperado %v", paid.FinalCapital, want)
	}
}

func TestCountNew(t *testing.T) {
	if countNew([]string{"A", "B", "C"}, []string{"A", "X", "Y"}) != 2 || countNew([]string{"A"}, []string{"A"}) != 0 {
		t.Fatal("countNew")
	}
	// rotación de 1 de 3 con comisión 1%: 2% / 3, como en el backtest original
	changed := float64(countNew([]string{"T08", "T07", "T05"}, []string{"T08", "T07", "T06"})) / 3
	if math.Abs(2*0.01*changed-0.02/3) > 1e-12 {
		t.Fatal("fracción rotada")
	}
}

func TestReportSkippedWarnsOncePerCode(t *testing.T) {
	var buf strings.Builder
	logx.SetConsole(&buf, logx.Info)
	skipped := Skipped{errs.BktFewTickers: {"2023-12-01": true, "2023-06-01": true}}
	ReportSkipped(skipped, "índice completo, backtest 2023-06-01 a 2024-12-31")
	out := buf.String()
	if strings.Count(out, "TOP-BKT-003") != 1 || !strings.Contains(out, "en 2 fechas de rebalanceo (de 2023-06-01 a 2023-12-01)") {
		t.Fatalf("salida: %q", out)
	}
}

// Con memoria de rankings el resultado es el mismo, incluidos los rebalanceos omitidos, y las corridas con otro desfase o
// otro período reutilizan lo que ya se calculó.
func TestRunWithMemoGivesTheSameResultAndSkips(t *testing.T) {
	prices := testutil.DefaultPrices()
	cons := tickersOf(prices)
	memo := NewMemo()
	for _, tc := range []struct{ start, end string }{
		{"2022-12-01", "2024-12-31"}, // las primeras fechas no tienen ruedas suficientes: se omiten
		{"2023-06-01", "2024-12-31"}, // comparte fechas con la anterior
		{"2023-07-01", "2024-12-31"}, // otro desfase
	} {
		plainSkips, memoSkips := Skipped{}, Skipped{}
		want, err1 := run(t, cons, prices, tc.start, tc.end, Params{Skipped: plainSkips})
		got, err2 := run(t, cons, prices, tc.start, tc.end, Params{Skipped: memoSkips, Memo: memo})
		again, err3 := run(t, cons, prices, tc.start, tc.end, Params{Memo: memo, Skipped: Skipped{}}) // todo desde la memoria
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatal(err1, err2, err3)
		}
		if !reflect.DeepEqual(want, got) || !reflect.DeepEqual(want, again) {
			t.Fatalf("%s: resultado distinto con memoria", tc.start)
		}
		if !reflect.DeepEqual(plainSkips, memoSkips) {
			t.Fatalf("%s: omitidos %v vs %v", tc.start, plainSkips, memoSkips)
		}
	}
	if len(memo.m) == 0 {
		t.Fatal("la memoria quedó vacía")
	}
}
