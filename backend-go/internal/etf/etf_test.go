package etf

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"top3/internal/model"
	"top3/internal/stats"
	"top3/internal/testutil"
)

type drift struct{ drift, vol float64 }

// prices son series con retornos diarios alternados ±vol alrededor de drift: precio, deriva y volatilidad conocidos.
func prices(days int, order []string, d map[string]drift) *model.Prices {
	all := testutil.BDays("2020-01-01", "2024-12-31")
	idx := all[len(all)-days:]
	p := model.NewPrices(idx, order)
	for j, name := range order {
		px := 100.0
		for i := range idx {
			r := d[name].drift - d[name].vol
			if i%2 == 0 {
				r = d[name].drift + d[name].vol
			}
			px *= 1 + r
			p.Px[i*len(order)+j] = px
		}
	}
	return p
}

func score(t *testing.T, p *model.Prices, horizons []int, rf string, minVol float64) (Scores, map[string]string) {
	t.Helper()
	return ScoreEtfs(p, horizons, 21, rf, minVol)
}

func tickers(s Scores) []string {
	var out []string
	for _, r := range s.Rows {
		out = append(out, r.Ticker)
	}
	return out
}

func byTicker(s Scores, t string) Score {
	for _, r := range s.Rows {
		if r.Ticker == t {
			return r
		}
	}
	return Score{Score: math.NaN()}
}

func TestScoreRewardsReturnPerUnitOfRisk(t *testing.T) {
	// misma deriva, distinta volatilidad: el de menos riesgo rankea primero
	p := prices(300, []string{"CALM", "WILD", "DOWN"}, map[string]drift{"CALM": {0.001, 0.002}, "WILD": {0.001, 0.02}, "DOWN": {-0.001, 0.002}})
	s, excluded := score(t, p, []int{3, 6, 12}, "", 0)
	if len(excluded) != 0 || !reflect.DeepEqual(tickers(s), []string{"CALM", "WILD", "DOWN"}) {
		t.Fatalf("excluded=%v ranking=%v", excluded, tickers(s))
	}
	if !(s.Rows[0].Score > s.Rows[1].Score && s.Rows[1].Score > 0 && 0 > s.Rows[2].Score) || s.Rows[2].Rank != 3 {
		t.Fatalf("scores: %+v", s.Rows)
	}
}

func TestScoreMatchesTheDefinition(t *testing.T) {
	p := prices(300, []string{"A"}, map[string]drift{"A": {0.001, 0.01}})
	s, _ := score(t, p, []int{3, 12}, "", 0)
	series := p.Series("A")
	sum := 0.0
	for _, h := range []int{3, 12} {
		w := series[len(series)-(h*21+1):]
		vol := stats.Std(stats.PctChange(w)) * math.Sqrt(252) * math.Sqrt(float64(h)/12)
		sum += (w[len(w)-1]/w[0] - 1) / vol
	}
	if got := s.Rows[0].Score; math.Abs(got-sum/2) > 1e-9 {
		t.Fatalf("score %v, definición %v", got, sum/2)
	}
	if want := series[len(series)-1]/series[len(series)-64] - 1; math.Abs(s.Rows[0].Returns[3]-want) > 1e-12 {
		t.Fatalf("ret_3 %v, %v", s.Rows[0].Returns[3], want)
	}
}

func TestRiskFreeReferenceIsSubtractedSoCashLikeEtfsDoNotWin(t *testing.T) {
	p := prices(300, []string{"CASH", "STOCK"}, map[string]drift{"CASH": {0.0002, 0.0001}, "STOCK": {0.0008, 0.01}})
	plain, _ := score(t, p, []int{3, 6, 12}, "", 0)
	if plain.Rows[0].Ticker != "CASH" { // sin referencia, lo casi sin riesgo gana
		t.Fatalf("sin referencia: %v", tickers(plain))
	}
	ranked, _ := score(t, p, []int{3, 6, 12}, "CASH", 0)
	if ranked.Rows[0].Ticker != "STOCK" || ranked.RiskFree != "CASH" || math.Abs(byTicker(ranked, "CASH").Score) > 1e-12 {
		t.Fatalf("con referencia: %+v", ranked)
	}
}

func TestUnavailableRiskFreeFallsBackToNoReference(t *testing.T) {
	p := prices(300, []string{"A"}, map[string]drift{"A": {0.001, 0.01}})
	if s, _ := score(t, p, []int{3}, "NOPE", 0); s.RiskFree != "" {
		t.Fatalf("RiskFree = %q", s.RiskFree)
	}
}

func TestEtfsWithGapsOrShortHistoryAreExcluded(t *testing.T) {
	p := prices(300, []string{"A", "B", "C"}, map[string]drift{"A": {0.001, 0.01}, "B": {0.002, 0.01}, "C": {0.003, 0.01}})
	jB, _ := p.Col("B")
	jC, _ := p.Col("C")
	p.Px[(p.Rows()-30)*3+jB] = math.NaN() // hueco dentro de la ventana
	for i := 0; i < 100; i++ {            // empezó a cotizar hace poco (no cubre 12 meses)
		p.Px[i*3+jC] = math.NaN()
	}
	s, excluded := score(t, p, []int{3, 12}, "", 0)
	if !reflect.DeepEqual(tickers(s), []string{"A"}) || len(excluded) != 2 || excluded["B"] == "" || excluded["C"] == "" {
		t.Fatalf("ranking=%v excluded=%v", tickers(s), excluded)
	}
}

func TestNotEnoughRowsExcludesEverything(t *testing.T) {
	p := prices(100, []string{"A"}, map[string]drift{"A": {0.001, 0.01}})
	s, excluded := score(t, p, []int{3, 12}, "", 0)
	if len(s.Rows) != 0 || len(excluded) != 1 {
		t.Fatalf("%v %v", s, excluded)
	}
}

func TestFlatPricesHaveNoScore(t *testing.T) {
	p := prices(300, []string{"A", "FLAT"}, map[string]drift{"A": {0.001, 0.01}, "FLAT": {0, 0}})
	s, excluded := score(t, p, []int{3}, "", 0)
	if len(s.Rows) != 1 || excluded["FLAT"] != "volatilidad nula" {
		t.Fatalf("ranking=%v excluded=%v", tickers(s), excluded)
	}
}

func TestTrendColumns(t *testing.T) {
	p := prices(300, []string{"UP", "DOWN"}, map[string]drift{"UP": {0.002, 0.005}, "DOWN": {-0.002, 0.005}})
	s, _ := score(t, p, []int{3, 12}, "", 0)
	up, down := byTicker(s, "UP"), byTicker(s, "DOWN")
	if !(up.SMADist > 0 && 0 > down.SMADist) || !(down.MaxDD < up.MaxDD && up.MaxDD <= 0) {
		t.Fatalf("up=%+v down=%+v", up, down)
	}
}

func TestCashLikeEtfsStayOutOfTheRankingButTheReferenceStillApplies(t *testing.T) {
	p := prices(300, []string{"CASH", "BILL", "STOCK", "BOND"}, map[string]drift{
		"CASH": {0.0002, 0.0001}, "BILL": {0.00022, 0.0001}, "STOCK": {0.0008, 0.01}, "BOND": {0.0002, 0.003}})
	ranked, excluded := score(t, p, []int{3, 6, 12}, "CASH", 0.01)
	if len(excluded) != 0 || !reflect.DeepEqual(ranked.CashLike, []string{"BILL", "CASH"}) || len(ranked.Rows) != 2 || ranked.RiskFree != "CASH" {
		t.Fatalf("ranked=%v excluded=%v cash=%v", tickers(ranked), excluded, ranked.CashLike)
	}
	plain, _ := score(t, p, []int{3, 6, 12}, "CASH", 0) // sin umbral, BILL competiría con un score enorme
	if len(plain.CashLike) != 0 || byTicker(plain, "BILL").Ticker != "BILL" {
		t.Fatalf("plain: %v", tickers(plain))
	}
}

func TestLoadUniverse(t *testing.T) {
	f := filepath.Join(t.TempDir(), "e.csv")
	os.WriteFile(f, []byte("ticker,name,category\nvti , Total ,Amplio\nVTI,Dup,X\n,Sin ticker,X\nBND,Bonos,Bonos\n"), 0o644)
	got, err := LoadUniverse(f)
	if err != nil || len(got) != 2 || got[0] != (Fund{"VTI", "Total", "Amplio"}) || got[1].Ticker != "BND" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestLoadUniverseErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadUniverse(filepath.Join(dir, "no_existe.csv")); err == nil || !strings.Contains(err.Error(), "TOP-ETF-001") {
		t.Fatalf("err = %v", err)
	}
	bad := filepath.Join(dir, "bad.csv")
	os.WriteFile(bad, []byte("ticker,name\nVTI,Total\n"), 0o644)
	if _, err := LoadUniverse(bad); err == nil || !strings.Contains(err.Error(), "category") {
		t.Fatalf("err = %v", err)
	}
	empty := filepath.Join(dir, "empty.csv")
	os.WriteFile(empty, []byte("ticker,name,category\n"), 0o644)
	if _, err := LoadUniverse(empty); err == nil || !strings.Contains(err.Error(), "vacía") {
		t.Fatalf("err = %v", err)
	}
}

func TestTheShippedEtfListIsSane(t *testing.T) {
	list, err := LoadUniverse("../../vanguard_etfs.csv")
	if err != nil || len(list) <= 50 {
		t.Fatalf("len=%d err=%v", len(list), err)
	}
	var hasRF bool
	for _, f := range list {
		if f.Name == "" || f.Category == "" {
			t.Fatalf("fila incompleta: %+v", f)
		}
		hasRF = hasRF || f.Ticker == "VUSB"
	}
	if !hasRF {
		t.Fatal("VUSB es la referencia libre de riesgo por defecto y debe estar en la lista")
	}
}

func TestTableRows(t *testing.T) {
	p := prices(300, []string{"A", "B"}, map[string]drift{"A": {0.002, 0.005}, "B": {0.0005, 0.005}})
	s, _ := score(t, p, []int{3, 12}, "", 0)
	headers, rows := TableRows(s, []Fund{{"A", "A ETF", "Cat"}, {"B", "B ETF", "Cat"}}, []int{12, 3})
	if !reflect.DeepEqual(headers, []string{"#", "Ticker", "Nombre", "Categoría", "Score", "Ret. 3m", "Ret. 12m", "Vol. anual", "Caída máx.", "vs SMA200"}) {
		t.Fatalf("headers: %q", headers)
	}
	if rows[0][1] != "A" || rows[0][2] != "A ETF" || !strings.HasPrefix(rows[0][5], "+") || !strings.HasSuffix(rows[0][7], "%") || strings.HasPrefix(rows[0][7], "+") {
		t.Fatalf("fila: %q", rows[0])
	}
}

func TestDescribeChanges(t *testing.T) {
	if got := DescribeChanges("", changes(nil, nil), 5); !strings.Contains(got, "primera corrida") {
		t.Fatal(got)
	}
	if got := DescribeChanges("2024-01-01", changes(nil, nil), 5); got != "Cambios del top 5 vs. 2024-01-01: ninguno." {
		t.Fatal(got)
	}
	if got := DescribeChanges("2024-01-01", changes([]string{"B"}, []string{"A"}), 1); got != "Cambios del top 1 vs. 2024-01-01: entran B; salen A." {
		t.Fatal(got)
	}
}
