package tables

import (
	"reflect"
	"strings"
	"testing"

	"top3/internal/dateutil"
	"top3/internal/model"
)

func calendar(offset int, final, cagr float64) model.Calendar {
	return model.Calendar{Offset: offset, Result: model.Result{
		Equity: []model.EquityPoint{{Date: dateutil.MustParse("2024-01-01"), Capital: 1000},
			{Date: dateutil.MustParse("2024-09-01"), Capital: final}},
		History:        []model.Rebalance{{Date: "2024-01-01", Picks: []model.Pick{{Ticker: "A", Return: 0.5}}}},
		FinalCapital:   final,
		TotalReturnPct: (final/1000 - 1) * 100,
		CAGRPct:        cagr,
	}}
}

func term(months int, cagrs ...float64) model.Term {
	t := model.Term{Months: months}
	for i, c := range cagrs {
		t.Calendars = append(t.Calendars, calendar(i, 1000*(1+c/100), c))
	}
	return t
}

func TestTermStats(t *testing.T) {
	tm := model.Term{Months: 4, Calendars: []model.Calendar{calendar(1, 1500, 40), calendar(0, 1100, 10), calendar(2, 1300, 25)}}
	if tm.Base().Offset != 0 {
		t.Fatal("base")
	}
	if m, lo, hi := tm.CagrStats(); m != 25 || lo != 10 || hi != 40 {
		t.Fatalf("%v %v %v", m, lo, hi)
	}
	if m, lo, hi := tm.CapitalStats(); m != 1300 || lo != 1100 || hi != 1500 {
		t.Fatalf("%v %v %v", m, lo, hi)
	}
	if (model.Term{Months: 4, Calendars: []model.Calendar{calendar(1, 1500, 40)}}).Base() != nil {
		t.Fatal("sin calendario base")
	}
	if model.Median([]float64{1, 2, 3, 10}) != 2.5 {
		t.Fatal("mediana par")
	}
}

func TestHistoryTable(t *testing.T) {
	history := []model.Rebalance{
		{Date: "2024-01-01", Picks: []model.Pick{{Ticker: "A", Return: 0.5}, {Ticker: "B", Return: -0.1234}}, Caps: map[string]float64{"A": 3e12}},
		{Date: "2024-05-01", Picks: []model.Pick{{Ticker: "C", Return: 0.01}}},
	}
	equity := []model.EquityPoint{
		{Date: dateutil.MustParse("2024-01-01"), Capital: 1000},
		{Date: dateutil.MustParse("2024-05-01"), Capital: 1200},
		{Date: dateutil.MustParse("2024-09-03"), Capital: 1500}}
	tb := HistoryTable(history, equity, 1000)
	wantHeaders := []string{"Fecha", "#1", "#2", "Cap. aprox. (US$ B)", "Capital (US$)", "Gan/Pérd"}
	if !reflect.DeepEqual(tb.Headers, wantHeaders) {
		t.Fatalf("headers %v", tb.Headers)
	}
	want := [][]string{
		{"2024-01-01", "A +50.0%", "B -12.3%", "3,000 / n/d", "1,000.00", "+0.0%"},
		{"2024-05-01", "C +1.0%", "", "n/d", "1,200.00", "+20.0%"},
		{"2024-09-03", "", "", "", "1,500.00", "+50.0%"},
	}
	if !reflect.DeepEqual(tb.Rows, want) {
		t.Fatalf("rows %q", tb.Rows)
	}
}

func TestSweepTableOrdersByMedianAndListsTermsWithoutData(t *testing.T) {
	terms := []model.Term{term(3, 10, 12), term(4, 30, 20), {Months: 6}}
	tb := SweepTable(terms)
	if len(tb.Rows) != 3 || tb.Rows[0][0] != "4 meses" || tb.Rows[1][0] != "3 meses" || tb.Rows[2][0] != "6 meses" {
		t.Fatalf("filas: %q", tb.Rows)
	}
	if tb.Rows[2][2] != "sin datos" {
		t.Fatalf("plazo sin datos: %q", tb.Rows[2])
	}
	best := SweepBest(terms, 1000)
	if !strings.HasPrefix(best, "Mejor plazo: 4 meses (CAGR mediano +25.0%, rango +20.0% a +30.0%; calendario base US$ 1,000 → US$ 1,300.00") {
		t.Fatalf("best = %q", best)
	}
	if SweepBest([]model.Term{{Months: 3}}, 1000) != "" {
		t.Fatal("sin resultados no hay mejor plazo")
	}
}

func TestOffsetsTable(t *testing.T) {
	terms := []model.Term{term(3, 10, 12), term(4, 30, 20)}
	tb := OffsetsTable(terms)
	if tb.Rows[0][0] != "4 meses" || tb.Rows[0][2] != "+25.0%" || tb.Rows[0][5] != "1,250" {
		t.Fatalf("filas: %q", tb.Rows)
	}
	if got := OffsetsBest(terms); got != "Mejor plazo por CAGR mediano: 4 meses (mediana +25.0%, rango +20.0% a +30.0%, 2 calendarios)" {
		t.Fatalf("best = %q", got)
	}
}

func TestModelsTable(t *testing.T) {
	terms := []model.Term{term(3, 10), term(4, 20), term(6, 5)}
	skip := []model.Term{term(3, 15), term(4, 18)}
	tb := ModelsTable(terms, skip)
	want := [][]string{
		{"4 meses", "+20.0%", "+18.0%", "-2.0 pp", "1,200", "1,180", "N-0"},
		{"3 meses", "+10.0%", "+15.0%", "+5.0 pp", "1,100", "1,150", "N-1"},
		{"6 meses", "+5.0%", "sin datos", "-", "1,050", "-", "-"},
	}
	if !reflect.DeepEqual(tb.Rows, want) {
		t.Fatalf("filas: %q", tb.Rows)
	}
}

func TestCalendarTable(t *testing.T) {
	tb := CalendarTable([]model.Calendar{calendar(0, 1100, 10)})
	if !reflect.DeepEqual(tb.Rows, [][]string{{"2024-01-01", "1", "1,100.00", "+10.0%", "+10.0%"}}) {
		t.Fatalf("filas: %q", tb.Rows)
	}
}
