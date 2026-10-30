package quant

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"top3/internal/config"
	"top3/internal/model"
	"top3/internal/repo"
	"top3/internal/testutil"
)

// syntheticPrices son n tickers T00.. que suben a ritmos crecientes con ruido (el último es el que más sube).
func syntheticPrices(n, days int, seed int64) *model.Prices {
	rng := rand.New(rand.NewSource(seed))
	dates := testutil.BDaysN("2025-01-01", days)
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("T%02d", i)
	}
	p := model.NewPrices(dates, names)
	for j := 0; j < n; j++ {
		drift := 0.0001 + (0.0012-0.0001)*float64(j)/float64(n-1)
		px := 100.0
		for i := 0; i < days; i++ {
			px *= 1 + drift + 0.01*rng.NormFloat64()
			p.Px[i*n+j] = px
		}
	}
	return p
}

func ohlcvOf(p *model.Prices, seed int64) repo.OHLCVFrames {
	rng := rand.New(rand.NewSource(seed))
	hi, lo, vol := model.NewPrices(p.Dates, p.Tickers), model.NewPrices(p.Dates, p.Tickers), model.NewPrices(p.Dates, p.Tickers)
	for i := range p.Px {
		w := 0.002 + 0.018*rng.Float64()
		hi.Px[i], lo.Px[i], vol.Px[i] = p.Px[i]*(1+w), p.Px[i]*(1-w), 1e6+4e6*rng.Float64()
	}
	return repo.OHLCVFrames{Open: p, High: hi, Low: lo, Close: p, Volume: vol}
}

func info(sector string, roe, d2e, margin float64) Data {
	f := func(v float64) *float64 { return &v }
	return Data{ShortName: "X", Sector: sector, FcfHistory: []*float64{f(1e9), f(1.1e9), f(0.9e9), f(1e9)}, Num: map[string]*float64{
		"netIncomeToCommon": f(2e9), "marketCap": f(1e11), "returnOnEquity": f(roe), "returnOnAssets": f(0.1), "profitMargins": f(0.15),
		"operatingMargins": f(margin), "debtToEquity": f(d2e), "currentRatio": f(1.5), "quickRatio": f(1), "freeCashflow": f(1e9),
		"operatingCashflow": f(2e9), "totalRevenue": f(1e10), "totalDebt": f(5e9), "totalCash": f(3e9), "ebitda": f(3e9)}}
}

func TestComputeBuildsTheFinalRankingAndMarkowitz(t *testing.T) {
	prices := syntheticPrices(14, 400, 0)
	tickers := append(append([]string(nil), prices.Tickers...), "NODATA")
	sort.Strings(tickers)
	rng := rand.New(rand.NewSource(3))
	sectors := []string{"Technology", "Energy", "Health"}
	data := map[string]Data{}
	for i, tk := range prices.Tickers {
		data[tk] = info(sectors[i%3], -0.1+0.6*rng.Float64(), 5+295*rng.Float64(), 0.4*rng.Float64())
	}
	cfg := config.Default()
	cfg.Quant.TopN, cfg.Quant.Weights = 6, []float64{1, 1, 2}
	out, err := Compute(ComputeInput{Prices: prices, Tickers: tickers, Data: data, OHLCV: ohlcvOf(prices, 1)}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cov := out.Coverage
	if cov.Universe != 15 || cov.Ranked != 14 || cov.Technical != 14 || !contains(cov.Excluded.NoTechnical, "NODATA") || !contains(cov.Excluded.NoMomentum, "NODATA") {
		t.Fatalf("cobertura: %+v", cov)
	}
	if len(out.Ranking) != 14 {
		t.Fatalf("ranking de %d", len(out.Ranking))
	}
	weightSum := 0.0
	for i, r := range out.Ranking {
		if r.Rank != i+1 {
			t.Fatalf("puesto %d en la posición %d", r.Rank, i)
		}
		if i > 0 && float64(r.Score) > float64(out.Ranking[i-1].Score) {
			t.Fatalf("el ranking no está ordenado por score en %d", i)
		}
		want := (float64(r.Momentum) + float64(r.Health) + 2*float64(r.Technical)) / 4 // promedio ponderado de los tres scores
		if math.Abs(float64(r.Score)-want) > 1e-9 {
			t.Fatalf("%s: score %v, esperado %v", r.Ticker, float64(r.Score), want)
		}
		if len(r.TechnicalGroups) != 4 || math.IsNaN(float64(r.RSI)) {
			t.Fatalf("%s: grupos técnicos %v rsi %v", r.Ticker, r.TechnicalGroups, r.RSI)
		}
		if i >= 6 && r.Weight != 0 {
			t.Fatalf("%s está fuera del top y pesa %v", r.Ticker, r.Weight)
		}
		weightSum += float64(r.Weight)
	}
	if math.Abs(weightSum-1) > 1e-9 {
		t.Fatalf("los pesos suman %v", weightSum)
	}
	top := make([]string, 6)
	for i := range top {
		top[i] = out.Ranking[i].Ticker
	}
	for i, mt := range out.Markowitz.Tickers {
		if mt.Ticker != top[i] {
			t.Fatalf("Markowitz sobre %v, el top es %v", out.Markowitz.Tickers, top)
		}
	}
	var main *Portfolio
	for i := range out.Markowitz.Portfolios {
		if out.Markowitz.Portfolios[i].Key == out.Markowitz.Main {
			main = &out.Markowitz.Portfolios[i]
		}
	}
	if main == nil {
		t.Fatalf("no está la cartera principal %q", out.Markowitz.Main)
	}
	for _, w := range main.Weights {
		if !contains(top, w.Ticker) {
			t.Fatalf("%s pesa pero no está en el top", w.Ticker)
		}
	}
	js, err := json.Marshal(out) // sin NaN ni infinitos: es JSON válido
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"as_of"`, `"coverage"`, `"ranking"`, `"markowitz"`, `"portfolios"`, `"min_vol"`, `"sin_datos_tecnicos"`, `"health_groups"`} {
		if !strings.Contains(string(js), key) {
			t.Errorf("falta %s en el JSON", key)
		}
	}
}

func TestComputeNeedsEnoughStocksWithAllThreeScores(t *testing.T) {
	prices := syntheticPrices(5, 400, 0)
	cfg := config.Default() // top_n = 10 > 5 acciones
	data := map[string]Data{}
	for _, tk := range prices.Tickers {
		data[tk] = info("Technology", 0.2, 50, 0.2)
	}
	_, err := Compute(ComputeInput{Prices: prices, Tickers: prices.Tickers, Data: data, OHLCV: ohlcvOf(prices, 1)}, cfg)
	if err == nil || !strings.Contains(err.Error(), "TOP-QNT-001") {
		t.Fatalf("err = %v", err)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestNumMarshalsNaNAsNull(t *testing.T) {
	js, _ := json.Marshal(map[string]Num{"a": Num(math.NaN()), "b": Num(math.Inf(1)), "c": 1.5})
	if string(js) != `{"a":null,"b":null,"c":1.5}` {
		t.Fatalf("got %s", js)
	}
	var back map[string]Num
	if err := json.Unmarshal([]byte(`{"a":null,"c":2}`), &back); err != nil || !math.IsNaN(float64(back["a"])) || back["c"] != 2 {
		t.Fatalf("back = %v err %v", back, err)
	}
}

func TestIndicatorBoundsAndSpecialCases(t *testing.T) {
	prices := syntheticPrices(4, 400, 0)
	o := ohlcvOf(prices, 1)
	for _, tk := range prices.Tickers {
		c, h, l, v := prices.Series(tk), o.High.Series(tk), o.Low.Series(tk), o.Volume.Series(tk)
		in01 := func(name string, x []float64, from int) {
			for i := from; i < len(x); i++ {
				if x[i] < 0 || x[i] > 100 {
					t.Fatalf("%s %s[%d] = %v", tk, name, i, x[i])
				}
			}
		}
		in01("rsi", rsi(c, 14), 30)
		in01("stoch", stochastic(h, l, c, 14, 3), 30)
		in01("mfi", mfi(h, l, c, v, 14), 30)
		a, _, _ := adx(h, l, c, 14)
		in01("adx", a, 60)
	}
	rising := make([]float64, 59)
	for i := range rising {
		rising[i] = float64(i + 1)
	}
	if got := last(rsi(rising, 14)); got != 100 { // sin pérdidas: RSI = 100
		t.Fatalf("rsi de una serie siempre al alza: %v", got)
	}
	got := obv([]float64{1, 2, 1.5, 1.5}, []float64{10, 10, 20, 30})
	want := []float64{0, 10, -10, -10}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("obv = %v", got)
		}
	}
}

func TestMomentumLeavesOutTickersWithGapsOrShortHistory(t *testing.T) {
	p := syntheticPrices(6, 400, 0)
	j1, _ := p.Col("T01")
	j2, _ := p.Col("T02")
	p.Px[(400-100)*6+j1] = math.NaN()
	for i := 0; i < 300; i++ {
		p.Px[i*6+j2] = math.NaN()
	}
	m12, m6 := MomentumReturns(p)
	if !math.IsNaN(m12[j1]) || !math.IsNaN(m6[j2]) {
		t.Fatalf("T01 y T02 deben quedar fuera: %v %v", m12[j1], m6[j2])
	}
	score := MomentumScore(m12, m6)
	for i, s := range score {
		if i != j1 && i != j2 && (s < 0 || s > 100 || math.IsNaN(s)) {
			t.Fatalf("score %d = %v", i, s)
		}
	}
	short := p.Tail(100)
	a, _ := MomentumReturns(short)
	for _, v := range a {
		if !math.IsNaN(v) {
			t.Fatal("con menos de un año no hay momentum")
		}
	}
}

func TestMarkowitzRespectsTheLimitsAndOrdersRisk(t *testing.T) {
	prices := syntheticPrices(10, 300, 0)
	mu, sigma := Inputs(prices, 0.25, 0.09)
	for _, m := range mu {
		if m < -1 || m > 1 {
			t.Fatalf("mu fuera de rango: %v", mu)
		}
	}
	out, err := Optimize(prices.Tickers, mu, sigma, 0.30, 0.25, 0.04, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Portfolio{}
	for _, p := range out.Portfolios {
		by[p.Key] = p
		sum, maxW := 0.0, 0.0
		for _, w := range p.Weights {
			sum += w.Weight
			maxW = math.Max(maxW, w.Weight)
		}
		if math.Abs(sum-1) > 1e-3 || maxW > 0.25+1e-6 { // los pesos < 1e-4 no se listan
			t.Fatalf("%s: suma %v máximo %v", p.Key, sum, maxW)
		}
	}
	if len(by) != 3 || by["min_var"].Vol > by["max_sharpe"].Vol+1e-6 || by["max_return"].Vol > 0.30+1e-4 ||
		by["max_return"].ExpReturn < by["min_var"].ExpReturn-1e-6 || by["max_sharpe"].Sharpe < by["min_var"].Sharpe-1e-6 ||
		math.Abs(out.MinVol-by["min_var"].Vol) > 1e-9 {
		t.Fatalf("carteras: %+v", by)
	}
}

func TestMarkowitzCapsTheWeightOfEachSector(t *testing.T) {
	prices := syntheticPrices(10, 300, 0)
	mu, sigma := Inputs(prices, 0.25, 0.09)
	for i := 0; i < 5; i++ {
		mu[i] += 0.5 // las cinco primeras (mismo sector) son las más atractivas
	}
	sectors := []string{"A", "A", "A", "A", "A", "B", "B", "B", "C", "C"}
	free, err := Optimize(prices.Tickers, mu, sigma, 0.40, 0.25, 0.04, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	capped, err := Optimize(prices.Tickers, mu, sigma, 0.40, 0.25, 0.04, sectors, 0.40)
	if err != nil {
		t.Fatal(err)
	}
	if capped.SectorRelaxed {
		t.Fatal("el tope se puede cumplir sin relajarlo")
	}
	for _, p := range capped.Portfolios {
		total := 0.0
		for _, s := range p.Sectors {
			total += s.Weight
			if s.Weight > 0.40+1e-6 {
				t.Fatalf("%s: el sector %s pesa %v", p.Key, s.Sector, s.Weight)
			}
		}
		if math.Abs(total-1) > 1e-3 {
			t.Fatalf("%s: los sectores suman %v", p.Key, total)
		}
	}
	if free.Portfolios[1].Sectors[0].Weight <= 0.40+1e-3 { // sin tope el sector A pesa más
		t.Fatalf("sin tope: %+v", free.Portfolios[1].Sectors)
	}
	if capped.Portfolios[0].ExpReturn > free.Portfolios[0].ExpReturn+1e-6 {
		t.Fatal("el tope no puede mejorar el retorno")
	}
}

func TestFeasibleSectorCap(t *testing.T) {
	two := append(repeat("A", 6), repeat("B", 4)...)
	if got := FeasibleSectorCap(two, 0.25, 0.40); math.Abs(got-0.50) > 1e-9 { // con 40% solo se coloca 80%: sube a 50%
		t.Fatalf("got %v", got)
	}
	if got := FeasibleSectorCap(repeat("A", 10), 0.25, 0.40); math.Abs(got-1) > 1e-9 {
		t.Fatalf("got %v", got)
	}
	five := []string{"A", "B", "C", "D", "E", "A", "B", "C", "D", "E"}
	if got := FeasibleSectorCap(five, 0.25, 0.40); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("got %v", got)
	}
	prices := syntheticPrices(4, 300, 0)
	mu, sigma := Inputs(prices, 0.25, 0.09)
	out, err := Optimize(prices.Tickers, mu, sigma, 0.5, 0.5, 0.04, repeat("A", 4), 0.40)
	if err != nil || !out.SectorRelaxed || math.Abs(out.SectorMax-1) > 1e-9 {
		t.Fatalf("relajado: %+v err %v", out, err)
	}
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func TestMarkowitzDropsMaxReturnWhenTheRiskTargetIsUnreachable(t *testing.T) {
	prices := syntheticPrices(10, 300, 0)
	mu, sigma := Inputs(prices, 0.25, 0.09)
	out, err := Optimize(prices.Tickers, mu, sigma, 0.001, 0.25, 0.04, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, p := range out.Portfolios {
		keys = append(keys, p.Key)
	}
	if strings.Join(keys, ",") != "max_sharpe,min_var" {
		t.Fatalf("carteras: %v", keys)
	}
}

func TestOptimizeMinLeavesNoSmallPositions(t *testing.T) {
	prices := syntheticPrices(10, 300, 0)
	mu, sigma := Inputs(prices, 0.25, 0.09)
	for _, wMin := range []float64{0, 0.10} {
		out, err := OptimizeMin(prices.Tickers, mu, sigma, 0.30, 0.25, 0.04, nil, 1, wMin, 0.01)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range out.Portfolios {
			sum := 0.0
			for _, w := range p.Weights {
				sum += w.Weight
				if wMin > 0 && w.Weight < wMin-1e-4 {
					t.Errorf("w_min=%g %s: %s pesa %.4f", wMin, p.Key, w.Ticker, w.Weight)
				}
				if w.Weight > 0.25+1e-4 {
					t.Errorf("%s: %s pesa %.4f, sobre el tope", p.Key, w.Ticker, w.Weight)
				}
			}
			if math.Abs(sum-1) > 1e-6 {
				t.Errorf("w_min=%g %s: pesos suman %.6f", wMin, p.Key, sum)
			}
		}
	}
}
