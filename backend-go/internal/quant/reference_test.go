package quant

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"top3/internal/dateutil"
	"top3/internal/model"
)

// Los valores esperados salen de testdata/reference.json: se calcularon con herramientas estadísticas externas con las
// mismas entradas. Estos tests comprueban que esta implementación da lo mismo.

// F es un número que en el JSON puede venir como "NaN", "Inf" o "-Inf".
type F float64

func (f *F) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		switch s {
		case "NaN":
			*f = F(math.NaN())
		case "Inf":
			*f = F(math.Inf(1))
		case "-Inf":
			*f = F(math.Inf(-1))
		default:
			return json.Unmarshal(b, new(float64))
		}
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = F(v)
	return nil
}

type mkResult struct {
	Portfolios []struct {
		Key       string `json:"key"`
		ExpReturn F      `json:"exp_return"`
		Vol       F      `json:"vol"`
		Sharpe    F      `json:"sharpe"`
		Weights   []struct {
			Ticker string `json:"ticker"`
			Weight F      `json:"weight"`
		} `json:"weights"`
	} `json:"portfolios"`
	MinVol        F    `json:"min_vol"`
	SectorMax     F    `json:"sector_max"`
	SectorRelaxed bool `json:"sector_relaxed"`
}

type reference struct {
	Dates   []string `json:"dates"`
	Tickers []string `json:"tickers"`
	Prices  [][]F    `json:"prices"`
	High    [][]F    `json:"high"`
	Low     [][]F    `json:"low"`
	Close   [][]F    `json:"close"`
	Volume  [][]F    `json:"volume"`

	Mom121   []F `json:"mom_12_1"`
	Mom61    []F `json:"mom_6_1"`
	Momentum []F `json:"momentum"`

	IndColumns  []string `json:"ind_columns"`
	Ind         [][]F    `json:"ind"`
	TechColumns []string `json:"tech_columns"`
	Tech        [][]F    `json:"tech"`
	EwmInput    []F      `json:"ewm_input"`
	EwmSpan12   []F      `json:"ewm_span12"`
	EwmWilder14 []F      `json:"ewm_wilder14"`
	FundTickers []string `json:"fund_tickers"`
	FundCols    []string `json:"fund_metric_columns"`
	FundMetrics [][]F    `json:"fund_metrics"`
	HealthCols  []string `json:"health_columns"`
	Health      [][]F    `json:"health"`
	MkTickers   []string `json:"mk_tickers"`
	MkMu        []F      `json:"mk_mu"`
	MkSigma     [][]F    `json:"mk_sigma"`
	MkCases     map[string]struct {
		Mu   []F `json:"mu"`
		Args struct {
			VolTarget float64  `json:"vol_target"`
			WMax      float64  `json:"w_max"`
			RiskFree  float64  `json:"risk_free"`
			Sectors   []string `json:"sectors"`
			SectorMax float64  `json:"sector_max"`
		} `json:"args"`
		Result mkResult `json:"result"`
	} `json:"mk_cases"`
}

func loadReference(t *testing.T) (*reference, map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref reference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	var parts map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		t.Fatal(err)
	}
	return &ref, parts
}

func toFloats(f []F) []float64 {
	out := make([]float64, len(f))
	for i, v := range f {
		out[i] = float64(v)
	}
	return out
}

func column(m [][]F, j int) []float64 {
	out := make([]float64, len(m))
	for i := range m {
		out[i] = float64(m[i][j])
	}
	return out
}

func near(a, b, tol float64) bool {
	switch {
	case math.IsNaN(a) || math.IsNaN(b):
		return math.IsNaN(a) && math.IsNaN(b)
	case math.IsInf(a, 0) || math.IsInf(b, 0):
		return a == b
	}
	return math.Abs(a-b) <= tol*(1+math.Abs(b))
}

func (r *reference) prices(t *testing.T, m [][]F) *model.Prices {
	t.Helper()
	dates := make([]time.Time, len(r.Dates))
	for i, d := range r.Dates {
		dates[i] = dateutil.MustParse(d)
	}
	p := model.NewPrices(dates, r.Tickers)
	for i := range m {
		for j := range m[i] {
			p.Px[i*len(r.Tickers)+j] = float64(m[i][j])
		}
	}
	return p
}

func TestEwmMatchReferenceIncludingNaNGaps(t *testing.T) {
	ref, _ := loadReference(t)
	in := toFloats(ref.EwmInput)
	for _, c := range []struct {
		name string
		got  []float64
		want []F
	}{
		{"span 12", emaSpan(in, 12), ref.EwmSpan12},
		{"wilder 14", wilder(in, 14), ref.EwmWilder14},
	} {
		for i, w := range c.want {
			if !near(c.got[i], float64(w), 1e-12) {
				t.Fatalf("%s posición %d: %v, esperado %v", c.name, i, c.got[i], float64(w))
			}
		}
	}
}

func TestMomentumMatchReference(t *testing.T) {
	ref, _ := loadReference(t)
	p := ref.prices(t, ref.Prices)
	m12, m6 := MomentumReturns(p)
	score := MomentumScore(m12, m6)
	for j, tk := range ref.Tickers {
		if !near(m12[j], float64(ref.Mom121[j]), 1e-12) || !near(m6[j], float64(ref.Mom61[j]), 1e-12) {
			t.Errorf("%s: momentum %v %v, esperado %v %v", tk, m12[j], m6[j], float64(ref.Mom121[j]), float64(ref.Mom61[j]))
		}
		if !near(score[j], float64(ref.Momentum[j]), 1e-12) {
			t.Errorf("%s: score %v, esperado %v", tk, score[j], float64(ref.Momentum[j]))
		}
	}
}

func indOf(cols []string, row []F) TechInd {
	g := func(name string) float64 {
		for i, c := range cols {
			if c == name {
				return float64(row[i])
			}
		}
		panic(name)
	}
	return TechInd{Close: g("close"), Ema20: g("ema20"), Ema50: g("ema50"), Ema200: g("ema200"), Macd: g("macd"),
		MacdSignal: g("macd_signal"), MacdHist: g("macd_hist"), MacdHist3d: g("macd_hist_3d"), RSI: g("rsi"), Stoch: g("stoch"),
		PctB: g("pct_b"), ADX: g("adx"), PlusDI: g("plus_di"), MinusDI: g("minus_di"), OBV: g("obv"), ObvEma: g("obv_ema"),
		Obv20d: g("obv_20d"), MFI: g("mfi"), High52: g("high52"), Low52: g("low52")}
}

func TestTechnicalIndicatorsAndScoreMatchReference(t *testing.T) {
	ref, _ := loadReference(t)
	scores := make([]TechnicalScore, len(ref.Tickers))
	for j, tk := range ref.Tickers {
		got := ComputeIndicators(Bars{High: column(ref.High, j), Low: column(ref.Low, j), Close: column(ref.Close, j), Volume: column(ref.Volume, j)})
		want := indOf(ref.IndColumns, ref.Ind[j])
		pairs := []struct {
			name      string
			got, want float64
		}{
			{"close", got.Close, want.Close}, {"ema20", got.Ema20, want.Ema20}, {"ema50", got.Ema50, want.Ema50},
			{"ema200", got.Ema200, want.Ema200}, {"macd", got.Macd, want.Macd}, {"macd_signal", got.MacdSignal, want.MacdSignal},
			{"macd_hist", got.MacdHist, want.MacdHist}, {"macd_hist_3d", got.MacdHist3d, want.MacdHist3d}, {"rsi", got.RSI, want.RSI},
			{"stoch", got.Stoch, want.Stoch}, {"pct_b", got.PctB, want.PctB}, {"adx", got.ADX, want.ADX},
			{"plus_di", got.PlusDI, want.PlusDI}, {"minus_di", got.MinusDI, want.MinusDI}, {"obv", got.OBV, want.OBV},
			{"obv_ema", got.ObvEma, want.ObvEma}, {"obv_20d", got.Obv20d, want.Obv20d}, {"mfi", got.MFI, want.MFI},
			{"high52", got.High52, want.High52}, {"low52", got.Low52, want.Low52},
		}
		for _, p := range pairs {
			if !near(p.got, p.want, 1e-9) {
				t.Errorf("%s %s: %v, esperado %v", tk, p.name, p.got, p.want)
			}
		}
		scores[j] = ScoreOne(got)
	}
	TechnicalRank(scores)
	col := func(name string) int {
		for i, c := range ref.TechColumns {
			if c == name {
				return i
			}
		}
		panic(name)
	}
	for j, tk := range ref.Tickers {
		row := ref.Tech[j]
		for g, name := range TechnicalGroups {
			if !near(scores[j].Groups[g], float64(row[col(name)]), 1e-9) {
				t.Errorf("%s grupo %s: %v, esperado %v", tk, name, scores[j].Groups[g], float64(row[col(name)]))
			}
		}
		for name, got := range map[string]float64{"technical": scores[j].Technical, "rsi": scores[j].RSI, "macd_hist": scores[j].MacdHist,
			"vs_ema50": scores[j].VsEma50, "vs_ema200": scores[j].VsEma200} {
			if !near(got, float64(row[col(name)]), 1e-9) {
				t.Errorf("%s %s: %v, esperado %v", tk, name, got, float64(row[col(name)]))
			}
		}
	}
}

func TestHealthMatchReference(t *testing.T) {
	ref, parts := loadReference(t)
	var raw map[string]map[string]any
	if err := json.Unmarshal(parts["fund_data"], &raw); err != nil {
		t.Fatal(err)
	}
	data := map[string]Data{}
	for tk, m := range raw {
		b, _ := json.Marshal(m)
		d, ok := parseData(string(b))
		if !ok {
			t.Fatalf("%s: datos no válidos", tk)
		}
		data[tk] = d
	}
	rows := HealthIndicators(ref.FundTickers, data)
	if len(rows) != len(ref.FundTickers) {
		t.Fatalf("filas: %d", len(rows))
	}
	for i, tk := range ref.FundTickers {
		for k, name := range ref.FundCols {
			if name == "market_cap" {
				if !near(rows[i].MarketCap, float64(ref.FundMetrics[i][k]), 1e-12) {
					t.Errorf("%s market_cap: %v", tk, rows[i].MarketCap)
				}
				continue
			}
			if !near(rows[i].V[name], float64(ref.FundMetrics[i][k]), 1e-12) {
				t.Errorf("%s %s: %v, esperado %v", tk, name, rows[i].V[name], float64(ref.FundMetrics[i][k]))
			}
		}
	}
	health := HealthScore(rows, map[string]float64{"rentabilidad": 1, "solvencia": 1, "liquidez": 1, "caja": 2})
	col := func(name string) int {
		for i, c := range ref.HealthCols {
			if c == name {
				return i
			}
		}
		panic(name)
	}
	for i, tk := range ref.FundTickers {
		row := ref.Health[i]
		for g, name := range HealthGroups {
			if !near(health[i].Groups[g], float64(row[col(name)]), 1e-9) {
				t.Errorf("%s grupo %s: %v, esperado %v", tk, name, health[i].Groups[g], float64(row[col(name)]))
			}
		}
		if float64(health[i].Measure) != float64(row[col("groups")]) || !near(health[i].Health, float64(row[col("health")]), 1e-9) {
			t.Errorf("%s: grupos %d salud %v, esperado %v %v", tk, health[i].Measure, health[i].Health, float64(row[col("groups")]), float64(row[col("health")]))
		}
	}
}

func TestMarkowitzInputsMatchReference(t *testing.T) {
	ref, _ := loadReference(t)
	p := ref.prices(t, ref.Prices).Select(ref.MkTickers).Tail(260)
	mu, sigma := Inputs(p, 0.25, 0.09)
	for i := range mu {
		if !near(mu[i], float64(ref.MkMu[i]), 1e-10) {
			t.Errorf("mu[%d] = %v, esperado %v", i, mu[i], float64(ref.MkMu[i]))
		}
		for j := range sigma[i] {
			if !near(sigma[i][j], float64(ref.MkSigma[i][j]), 1e-9) {
				t.Errorf("sigma[%d][%d] = %v, LedoitWolf %v", i, j, sigma[i][j], float64(ref.MkSigma[i][j]))
			}
		}
	}
}

func TestMarkowitzMatchesReferenceFrontier(t *testing.T) {
	ref, _ := loadReference(t)
	sigma := make([][]float64, len(ref.MkSigma))
	for i := range sigma {
		sigma[i] = toFloats(ref.MkSigma[i])
	}
	for name, c := range ref.MkCases {
		t.Run(name, func(t *testing.T) {
			got, err := Optimize(ref.MkTickers, toFloats(c.Mu), sigma, c.Args.VolTarget, c.Args.WMax, c.Args.RiskFree, c.Args.Sectors, orOne(c.Args.SectorMax))
			if err != nil {
				t.Fatal(err)
			}
			want := c.Result
			if len(got.Portfolios) != len(want.Portfolios) {
				t.Fatalf("carteras: %d, esperado %d", len(got.Portfolios), len(want.Portfolios))
			}
			if !near(got.MinVol, float64(want.MinVol), 1e-5) || !near(got.SectorMax, float64(want.SectorMax), 1e-9) || got.SectorRelaxed != want.SectorRelaxed {
				t.Errorf("min_vol %v/%v sector_max %v/%v relaxed %v/%v", got.MinVol, float64(want.MinVol), got.SectorMax, float64(want.SectorMax), got.SectorRelaxed, want.SectorRelaxed)
			}
			for k, wp := range want.Portfolios {
				gp := got.Portfolios[k]
				if gp.Key != wp.Key {
					t.Fatalf("cartera %d: %s, esperado %s", k, gp.Key, wp.Key)
				}
				// el optimizador de referencia termina con tolerancia ~1e-8; los pesos coinciden a unas milésimas y las métricas mucho mejor
				if !near(gp.ExpReturn, float64(wp.ExpReturn), 2e-4) || !near(gp.Vol, float64(wp.Vol), 2e-4) || !near(gp.Sharpe, float64(wp.Sharpe), 5e-4) {
					t.Errorf("%s: retorno %v/%v vol %v/%v sharpe %v/%v", gp.Key, gp.ExpReturn, float64(wp.ExpReturn), gp.Vol, float64(wp.Vol), gp.Sharpe, float64(wp.Sharpe))
				}
				byTicker := map[string]float64{}
				for _, w := range gp.Weights {
					byTicker[w.Ticker] = w.Weight
				}
				for _, w := range wp.Weights {
					if math.Abs(byTicker[w.Ticker]-float64(w.Weight)) > 5e-3 {
						t.Errorf("%s peso de %s: %v, esperado %v", gp.Key, w.Ticker, byTicker[w.Ticker], float64(w.Weight))
					}
				}
			}
		})
	}
}

func orOne(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}
