package engine

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"top3/internal/dateutil"
	"top3/internal/model"
	"top3/internal/testutil"
)

// messy son precios con huecos sueltos, tickers que empiezan tarde y otros que dejan de cotizar.
func messy(days, tickers int, seed int64) *model.Prices {
	rng := rand.New(rand.NewSource(seed))
	dates := testutil.BDaysN("2022-01-03", days)
	names := make([]string, tickers)
	for i := range names {
		names[i] = "T" + string(rune('A'+i/26)) + string(rune('A'+i%26))
	}
	p := model.NewPrices(dates, names)
	for j := 0; j < tickers; j++ {
		px := 100.0
		for i := 0; i < days; i++ {
			px *= 1 + 0.0005 + 0.02*rng.NormFloat64()
			p.Px[i*tickers+j] = px
		}
	}
	for _, j := range rng.Perm(tickers)[:8] { // huecos sueltos
		for k := 0; k < 3; k++ {
			p.Px[(10+rng.Intn(days-20))*tickers+j] = math.NaN()
		}
	}
	for _, j := range rng.Perm(tickers)[:5] { // empieza tarde
		for i := 0; i < 50+rng.Intn(250); i++ {
			p.Px[i*tickers+j] = math.NaN()
		}
	}
	for _, j := range rng.Perm(tickers)[:4] { // deja de cotizar
		for i := 100 + rng.Intn(days-120); i < days; i++ {
			p.Px[i*tickers+j] = math.NaN()
		}
	}
	return p
}

// referenceRank es la definición directa: ventana de lookback+1 ruedas hasta when, sin los tickers con huecos.
func referenceRank(p *model.Prices, tickers []string, when time.Time, lookback int) ([]Ranked, bool) {
	rows := 0
	for _, d := range p.Dates {
		if !d.After(when) {
			rows++
		}
	}
	if rows < lookback+1 {
		return nil, false
	}
	var out []Ranked
	for _, t := range tickers {
		j, ok := p.Col(t)
		if !ok {
			continue
		}
		complete := true
		for i := rows - lookback - 1; i < rows; i++ {
			if math.IsNaN(p.At(i, j)) {
				complete = false
			}
		}
		if complete {
			out = append(out, Ranked{t, p.At(rows-1, j)/p.At(rows-lookback-1, j) - 1})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Return > out[b].Return })
	return out, true
}

func sampleTickers(rng *rand.Rand, p *model.Prices) []string {
	perm := rng.Perm(p.Cols())[:5+rng.Intn(p.Cols()-5)]
	out := make([]string, len(perm))
	for i, j := range perm {
		out[i] = p.Tickers[j]
	}
	return out
}

func TestReturnsMatchTheDirectDefinition(t *testing.T) {
	p := messy(400, 40, 7)
	m := NewMatrix(p)
	rng := rand.New(rand.NewSource(1))
	checked := 0
	for range 60 {
		tickers := sampleTickers(rng, p)
		when := p.Dates[rng.Intn(p.Rows())]
		for _, lookback := range []int{21, 63, 126} {
			want, ok := referenceRank(p, tickers, when, lookback)
			got, gotOK := m.Returns(tickers, when, lookback, 0)
			if ok != gotOK {
				t.Fatalf("ok: %v vs %v", gotOK, ok)
			}
			if !ok {
				continue
			}
			if len(got) != len(want) {
				t.Fatalf("largo %d vs %d", len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s lookback %d pos %d: %v vs %v", dateutil.Day(when), lookback, i, got[i], want[i])
				}
			}
			checked++
		}
	}
	if checked < 100 {
		t.Fatalf("se compararon solo %d casos", checked)
	}
}

func TestHoldReturnIsEqualWeightMean(t *testing.T) {
	dates := testutil.BDaysN("2024-01-01", 3)
	p := model.NewPrices(dates, []string{"A", "B"})
	copy(p.Px, []float64{100, 100, 110, 100, 120, 100})
	m := NewMatrix(p)
	ret, kept, last, ok := m.HoldReturn([]string{"A", "B"}, dates[0], dates[2])
	if !ok || math.Abs(ret-0.10) > 1e-12 || len(kept) != 2 || !last.Equal(dates[2]) {
		t.Fatalf("ret=%v kept=%v last=%v ok=%v", ret, kept, last, ok)
	}
	if _, _, _, ok := m.HoldReturn([]string{"A"}, dates[0], dates[0]); ok {
		t.Fatal("una sola rueda no tiene retorno")
	}
	// fechas que no son ruedas: del sábado al lunes siguiente
	sat := dateutil.MustParse("2024-01-06")
	if _, _, _, ok := m.HoldReturn([]string{"A"}, sat, sat.AddDate(0, 0, 2)); ok {
		t.Fatal("sin ruedas en el rango")
	}
}

func TestHoldReturnLeavesOutTickersWithGaps(t *testing.T) {
	p := messy(300, 20, 3)
	m := NewMatrix(p)
	start, end := p.Dates[50], p.Dates[250]
	all := p.Tickers
	_, kept, _, ok := m.HoldReturn(all, start, end)
	if !ok {
		t.Fatal("no hay retorno")
	}
	for _, tk := range kept {
		j, _ := p.Col(tk)
		for i := 50; i <= 250; i++ {
			if math.IsNaN(p.At(i, j)) {
				t.Fatalf("%s tiene un hueco en la fila %d y no debió entrar", tk, i)
			}
		}
	}
	if len(kept) == len(all) {
		t.Fatal("el ejemplo debería tener tickers con huecos")
	}
}

func TestLastPricesUseTheLastKnownValue(t *testing.T) {
	p := messy(300, 20, 5)
	m := NewMatrix(p)
	rng := rand.New(rand.NewSource(2))
	for range 40 {
		tickers := sampleTickers(rng, p)
		row := rng.Intn(p.Rows())
		got := m.LastPrices(tickers, p.Dates[row])
		for k, tk := range tickers {
			j, _ := p.Col(tk)
			want := p.LastValid(j, row)
			if !(got[k] == want || (math.IsNaN(got[k]) && math.IsNaN(want))) {
				t.Fatalf("%s fila %d: %v vs %v", tk, row, got[k], want)
			}
		}
	}
	if got := m.LastPrices([]string{p.Tickers[0], "NOPE"}, dateutil.MustParse("2000-01-01")); !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
		t.Fatalf("antes de que existan precios: %v", got)
	}
}

func TestSkipDaysMovesTheWindowBack(t *testing.T) {
	p := testutil.MakePrices("2024-01-01", "2024-10-01", 3)
	m := NewMatrix(p)
	when := p.Dates[p.Rows()-1]
	got, ok := m.Returns(p.Tickers, when, 63, 21)
	if !ok {
		t.Fatal("hay ruedas de sobra")
	}
	end := p.Rows() - 21
	for _, r := range got {
		j, _ := p.Col(r.Ticker)
		want := p.At(end-1, j)/p.At(end-64, j) - 1
		if r.Return != want {
			t.Fatalf("%s: %v vs %v", r.Ticker, r.Return, want)
		}
	}
	short := testutil.MakePrices("2024-01-01", "2024-03-29", 2) // 64 ruedas
	ms := NewMatrix(short)
	if _, ok := ms.Returns(short.Tickers, short.Dates[short.Rows()-1], 63, 0); !ok {
		t.Fatal("64 ruedas alcanzan sin skip")
	}
	if _, ok := ms.Returns(short.Tickers, short.Dates[short.Rows()-1], 63, 21); ok {
		t.Fatal("con skip hacen falta 85 ruedas")
	}
}

func TestEmptyMatrixDoesNotBreak(t *testing.T) {
	m := NewMatrix(model.NewPrices(nil, nil))
	if m.RowsUntil(dateutil.MustParse("2024-01-01")) != 0 {
		t.Fatal("RowsUntil")
	}
	if _, ok := m.Returns(nil, dateutil.MustParse("2024-01-01"), 10, 0); ok {
		t.Fatal("Returns sin datos")
	}
}
