package etfq

import (
	"math"
	"reflect"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMidRankPercentiles(t *testing.T) {
	got := midRankPercentiles([]float64{1, 2, 3, 4}, true)
	want := []float64{12.5, 37.5, 62.5, 87.5}
	for i := range want {
		if !near(got[i], want[i]) {
			t.Fatalf("higherBetter: %v, quería %v", got, want)
		}
	}
	got = midRankPercentiles([]float64{1, 2, 3, 4}, false) // el menor es el mejor
	for i := range want {
		if !near(got[i], want[len(want)-1-i]) {
			t.Fatalf("lowerBetter: %v", got)
		}
	}
	if got := midRankPercentiles([]float64{5, 5}, true); !near(got[0], 50) || !near(got[1], 50) {
		t.Fatalf("empate: %v", got)
	}
	if got := midRankPercentiles([]float64{7}, true); !near(got[0], 50) {
		t.Fatalf("uno solo: %v", got)
	}
}

func TestQualityPercentilesAreWithinTheCategory(t *testing.T) {
	in := []QualityInput{
		{"A1", "Acciones", 3, new(0.03), new(1e11)},
		{"A2", "Acciones", 2, new(0.10), new(1e9)},
		{"B1", "Bonos", 1, new(0.10), new(1e9)}, // el mismo TER y AUM que A2, pero es el único de su categoría
	}
	rows := ScoreQuality(in, QualityParams{Weight: 0.5, TERWeight: 1, AUMWeight: 1})
	if !near(*rows[0].TERPct, 75) || !near(*rows[1].TERPct, 25) {
		t.Fatalf("TER en Acciones: %v %v", *rows[0].TERPct, *rows[1].TERPct)
	}
	if !near(*rows[2].TERPct, 50) || !near(*rows[2].AUMPct, 50) {
		t.Fatalf("el único de su categoría queda en el medio: %+v", rows[2])
	}
}

func TestFinalScoreMixesMomentumAndQuality(t *testing.T) {
	in := []QualityInput{
		{"CARO", "X", 3, new(0.50), new(1e9)},   // mejor momentum, peor calidad
		{"BARATO", "X", 2, new(0.03), new(1e9)}, // mismo AUM, mucho más barato
	}
	// Sin peso de calidad manda el momentum
	rows := ScoreQuality(in, QualityParams{Weight: 0, TERWeight: 1, AUMWeight: 1})
	if rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Fatalf("peso 0: %+v", rows)
	}
	// Con todo el peso en la calidad manda el costo
	rows = ScoreQuality(in, QualityParams{Weight: 1, TERWeight: 1, AUMWeight: 0})
	if rows[1].Rank != 1 || rows[0].Rank != 2 {
		t.Fatalf("peso 1: %+v", rows)
	}
	// Mezcla: final = (1-w) x momentum + w x calidad
	rows = ScoreQuality(in, QualityParams{Weight: 0.25, TERWeight: 4, AUMWeight: 1})
	r := rows[0] // CARO: momentum 75, TER 25, AUM 50 (empate) -> calidad (4x25 + 1x50)/5 = 30
	if !near(*r.Momentum, 75) || !near(*r.Quality, 30) || !near(*r.Final, 0.75*75+0.25*30) {
		t.Fatalf("CARO: %+v", r)
	}
}

func TestIneligibleEtfsAreExcludedWithTheirReason(t *testing.T) {
	in := []QualityInput{
		{"OK", "X", 1, new(0.05), new(5e9)},
		{"CARO", "X", 9, new(0.45), new(5e9)},
		{"CHICO", "X", 8, new(0.05), new(2e7)},
		{"AMBOS", "X", 7, new(0.45), new(2e7)},
	}
	rows := ScoreQuality(in, QualityParams{MaxTER: 0.20, MinAUM: 1e8, Weight: 0.25, TERWeight: 1, AUMWeight: 1})
	if rows[0].Rank != 1 || rows[0].Excluded != nil {
		t.Fatalf("OK: %+v", rows[0])
	}
	for i, want := range [][]string{nil, {ExcludedTER}, {ExcludedAUM}, {ExcludedTER, ExcludedAUM}} {
		if !reflect.DeepEqual(rows[i].Excluded, want) {
			t.Fatalf("%s excluido por %v, quería %v", rows[i].Ticker, rows[i].Excluded, want)
		}
	}
	for _, r := range rows[1:] {
		if r.Rank != 0 || r.Final != nil || r.Momentum != nil {
			t.Fatalf("un excluido no puntúa: %+v", r)
		}
	}
	// Los excluidos no distorsionan la posición de los demás: el único elegible queda en el medio
	if !near(*rows[0].Momentum, 50) {
		t.Fatalf("momentum de OK: %v", *rows[0].Momentum)
	}
	// 0 desactiva el filtro
	for _, r := range ScoreQuality(in, QualityParams{Weight: 0.25, TERWeight: 1, AUMWeight: 1}) {
		if r.Excluded != nil || r.Rank == 0 {
			t.Fatalf("sin filtro: %+v", r)
		}
	}
}

func TestMissingDataNeverExcludesAndOnlyDropsThatFactor(t *testing.T) {
	in := []QualityInput{
		{"SOLOTER", "X", 3, new(0.03), nil},
		{"SOLOAUM", "X", 2, nil, new(1e10)},
		{"NADA", "X", 1, nil, nil},
		{"AMBOS", "X", 0, new(0.10), new(1e9)},
	}
	rows := ScoreQuality(in, QualityParams{MaxTER: 0.2, MinAUM: 1e8, Weight: 0.25, TERWeight: 4, AUMWeight: 1})
	if rows[0].AUMPct != nil || rows[0].Quality == nil || !near(*rows[0].Quality, *rows[0].TERPct) {
		t.Fatalf("solo TER: %+v", rows[0])
	}
	if rows[1].TERPct != nil || rows[1].Quality == nil || !near(*rows[1].Quality, *rows[1].AUMPct) {
		t.Fatalf("solo AUM: %+v", rows[1])
	}
	if rows[2].Quality != nil || rows[2].Final != nil || rows[2].Rank != 0 || rows[2].Excluded != nil || rows[2].Momentum == nil {
		t.Fatalf("sin ningún dato queda sin score final, pero no excluido: %+v", rows[2])
	}
	ranks := map[int]bool{}
	for _, r := range rows {
		if r.Rank > 0 {
			ranks[r.Rank] = true
		}
	}
	if !reflect.DeepEqual(ranks, map[int]bool{1: true, 2: true, 3: true}) {
		t.Fatalf("puestos: %v", ranks)
	}
}

func TestInvalidWeightsFallBackToSafeValues(t *testing.T) {
	in := []QualityInput{{"A", "X", 2, new(0.03), new(1e9)}, {"B", "X", 1, new(0.10), new(1e8)}}
	rows := ScoreQuality(in, QualityParams{Weight: 7, TERWeight: -1, AUMWeight: 0}) // peso > 1 y pesos inválidos
	if rows[0].Final == nil || *rows[0].Final > 100 || *rows[0].Final < 0 {
		t.Fatalf("final fuera de rango: %+v", rows[0])
	}
	if !near(*rows[0].Final, *rows[0].Quality) { // el peso se recorta a 1: solo cuenta la calidad
		t.Fatalf("final %v vs calidad %v", *rows[0].Final, *rows[0].Quality)
	}
}

func TestEmptyInput(t *testing.T) {
	if rows := ScoreQuality(nil, QualityParams{}); len(rows) != 0 {
		t.Fatalf("%+v", rows)
	}
}
