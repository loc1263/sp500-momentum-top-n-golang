package etfq

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/loc1263/gohoo"

	"top3/internal/config"
	"top3/internal/etf"
)

func fund(ticker, issuer string, ter, aum *float64) Fund {
	return Fund{Ticker: ticker, Name: ticker + " ETF", Issuer: issuer, TER: ter, AUM: aum}
}

func TestFundOfReadsTerAumAndNames(t *testing.T) {
	q := gohoo.ScreenQuote{Symbol: " ivv ", LongName: "iShares Core S&P 500 ETF", ShortName: "ISHARES CORE",
		Raw: map[string]any{"netExpenseRatio": 0.03, "netAssets": 8.8e11}}
	f, ok := fundOf(q, "iShares")
	if !ok || f.Ticker != "IVV" || f.Name != "iShares Core S&P 500 ETF" || f.Issuer != "iShares" || *f.TER != 0.03 || *f.AUM != 8.8e11 {
		t.Fatalf("%+v", f)
	}
	// Sin nombre largo se usa el corto; TER y AUM ausentes, negativos o en cero no cuentan
	f, ok = fundOf(gohoo.ScreenQuote{Symbol: "XYZ", ShortName: "XYZ FUND", Raw: map[string]any{"netExpenseRatio": -1.0, "netAssets": 0.0}}, "SPDR")
	if !ok || f.Name != "XYZ FUND" || f.TER != nil || f.AUM != nil {
		t.Fatalf("%+v", f)
	}
	f, _ = fundOf(gohoo.ScreenQuote{Symbol: "ZERO", Raw: map[string]any{"netExpenseRatio": 0.0}}, "SPDR")
	if f.TER == nil || *f.TER != 0 { // un TER de 0% es un dato válido
		t.Fatalf("TER 0: %+v", f)
	}
	if _, ok := fundOf(gohoo.ScreenQuote{Symbol: "  "}, "x"); ok {
		t.Fatal("sin símbolo no hay ETF")
	}
}

func TestAssembleJoinsMomentumQualityAndCategory(t *testing.T) {
	n := func(v float64) *float64 { return &v }
	funds := []Fund{
		fund("AAA", "iShares", n(0.03), n(5e9)),
		fund("BBB", "Vanguard", n(0.45), n(5e9)), // TER por encima del máximo
		fund("CCC", "Schwab", n(0.05), n(2e7)),   // AUM por debajo del mínimo
		fund("DDD", "SPDR", nil, nil),            // sin datos: no se excluye, tampoco puntúa
		fund("EEE", "Invesco", n(0.10), n(1e9)),
	}
	scores := etf.Scores{RiskFree: "VUSB", Rows: []etf.Score{
		{Ticker: "BBB", Rank: 1, Score: 2.0, Returns: map[int]float64{3: 0.05, 12: 0.2}},
		{Ticker: "AAA", Rank: 2, Score: 1.5, Returns: map[int]float64{3: 0.04, 12: 0.1}},
		{Ticker: "CCC", Rank: 3, Score: 1.0, Returns: map[int]float64{3: 0.03, 12: 0.1}},
		{Ticker: "DDD", Rank: 4, Score: 0.5, Returns: map[int]float64{3: 0.02, 12: 0.1}},
		{Ticker: "EEE", Rank: 5, Score: 0.2, Returns: map[int]float64{3: 0.01, 12: 0.1}},
	}}
	cfg := config.Default().Etfq
	cfg.MaxTER, cfg.MinAumMillions, cfg.QualityWeight, cfg.QualityWeights = 0.20, 100, 0.5, []float64{1, 1}
	p := Assemble(funds, scores, map[string]string{"AAA": "Large Blend", "BBB": "Large Blend", "CCC": "Large Blend", "EEE": "Large Blend"},
		map[string]int{"iShares": 40, "Vanguard": 30, "SPDR": 20, "Invesco": 10, "Schwab": 5}, Params{Config: cfg, Horizons: []int{3, 12}, NotRanked: 7},
		"2026-10-27", "2026-10-27T10:00:00")

	if p.AsOf != "2026-10-27" || p.NotRanked != 7 || p.RiskFree == nil || *p.RiskFree != "VUSB" || !reflect.DeepEqual(p.Horizons, []int{3, 12}) {
		t.Fatalf("payload: %+v", p)
	}
	if p.Settings.MaxTER != 0.20 || p.Settings.MinAumMillions != 100 || p.Settings.QualityWeight != 0.5 {
		t.Fatalf("settings: %+v", p.Settings)
	}
	var order []string
	by := map[string]Row{}
	for _, r := range p.Rows {
		order = append(order, r.Ticker)
		by[r.Ticker] = r
	}
	// Con score final primero (AAA y EEE); el resto por puesto de momentum (BBB 1, CCC 3, DDD 4)
	if !reflect.DeepEqual(order, []string{"AAA", "EEE", "BBB", "CCC", "DDD"}) {
		t.Fatalf("orden: %v", order)
	}
	if r := by["AAA"]; r.Rank == nil || *r.Rank != 1 || r.Category != "Large Blend" || r.GroupSize != 2 || r.Returns["12"] != 0.1 || r.MomentumRank != 2 {
		t.Fatalf("AAA: %+v", r)
	}
	if r := by["BBB"]; r.Rank != nil || !reflect.DeepEqual(r.Excluded, []string{ExcludedTER}) {
		t.Fatalf("BBB: %+v", r)
	}
	if r := by["CCC"]; r.Rank != nil || !reflect.DeepEqual(r.Excluded, []string{ExcludedAUM}) {
		t.Fatalf("CCC: %+v", r)
	}
	if r := by["DDD"]; r.Rank != nil || r.Category != NoCategory || r.Final != nil || len(r.Excluded) != 0 || r.GroupSize != 1 {
		t.Fatalf("DDD: %+v", r)
	}
	// Gestoras en el orden de [etfq] families, con ETFs listados, rankeados y con score final
	want := []Issuer{{"iShares", 40, 1, 1}, {"Vanguard", 30, 1, 0}, {"SPDR", 20, 1, 0}, {"Invesco", 10, 1, 1}, {"Schwab", 5, 1, 0}}
	if !reflect.DeepEqual(p.Issuers, want) {
		t.Fatalf("gestoras: %+v", p.Issuers)
	}
	raw, err := p.JSON()
	if err != nil || !strings.Contains(string(raw), `"excluded":[]`) || !json.Valid(raw) {
		t.Fatalf("json: %s %v", raw, err)
	}
}

func TestAssembleOmitsIssuersThatReturnedNothing(t *testing.T) {
	cfg := config.Default().Etfq
	p := Assemble(nil, etf.Scores{}, nil, map[string]int{"iShares": 3}, Params{Config: cfg}, "d", "g")
	if len(p.Issuers) != 1 || p.Issuers[0].Name != "iShares" || p.RiskFree != nil || len(p.Rows) != 0 {
		t.Fatalf("%+v", p)
	}
}
