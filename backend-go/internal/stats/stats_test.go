package stats

import (
	"math"
	"reflect"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-12 || (math.IsNaN(a) && math.IsNaN(b)) }

func TestRankPctAveragesTiesAndKeepsNaN(t *testing.T) {
	nan := math.NaN()
	got := RankPct([]float64{10, 30, nan, 20, 20}, true)
	// válidos: 10 (1), 20 y 20 (2 y 3 -> 2.5), 30 (4); se divide por 4
	want := []float64{0.25, 1, nan, 0.625, 0.625}
	for i := range want {
		if !almost(got[i], want[i]) {
			t.Fatalf("posición %d: %v, quiero %v (%v)", i, got[i], want[i], got)
		}
	}
	desc := RankPct([]float64{10, 30, 20}, false)
	if !reflect.DeepEqual(desc, []float64{1, 1.0 / 3, 2.0 / 3}) {
		t.Fatalf("descendente: %v", desc)
	}
}

func TestRankPctHandlesInfinities(t *testing.T) {
	inf := math.Inf(1)
	got := RankPct([]float64{1, inf, 5, inf}, true)
	if !almost(got[0], 0.25) || !almost(got[2], 0.5) || !almost(got[1], 0.875) || !almost(got[3], 0.875) {
		t.Fatalf("got %v", got)
	}
	asc := RankPct([]float64{1, inf}, false) // ascending=False: el infinito es el mejor puesto
	if !almost(asc[1], 0.5) || !almost(asc[0], 1) {
		t.Fatalf("got %v", asc)
	}
}

func TestStdMeanAndPctChange(t *testing.T) {
	xs := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	if !almost(Std(xs), math.Sqrt(32.0/7)) || !almost(Mean(xs), 5) {
		t.Fatalf("std=%v mean=%v", Std(xs), Mean(xs))
	}
	if !math.IsNaN(Std([]float64{1})) || !math.IsNaN(Mean(nil)) {
		t.Fatal("pocos datos")
	}
	if !math.IsNaN(Mean([]float64{1, math.NaN()})) || MeanSkipNaN([]float64{1, math.NaN(), 3}) != 2 {
		t.Fatal("NaN")
	}
	if got := PctChange([]float64{100, 110, 99}); !almost(got[0], 0.1) || !almost(got[1], -0.1) {
		t.Fatalf("got %v", got)
	}
}
