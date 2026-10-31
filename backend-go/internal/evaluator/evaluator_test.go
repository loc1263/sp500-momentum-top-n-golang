package evaluator

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// healthy es una empresa sana y barata, con todos los datos.
func healthy() Raw {
	r := emptyRaw()
	r.Ticker, r.Name, r.Sector, r.Industry, r.Currency, r.FinCurrency = "OK", "Ok Corp", "Tech", "Software", "USD", "USD"
	r.Price, r.MarketCap, r.Beta, r.W52Low, r.W52High = 100, 1000e9, 0.9, 80, 120
	r.PE, r.FwdPE, r.EPS, r.Dividend = 12, 10, 8, 1.5
	r.RevGrowth, r.EpsGrowth, r.ROE, r.Target = 12, 15, 20, 130
	r.Revenue, r.NetIncome, r.OCF, r.FCF, r.Debt, r.Cash = 100e9, 20e9, 30e9, 25e9, 10e9, 40e9
	r.CurrentRatio = 2
	r.QRevenue, r.QOpIncome, r.QInterest, r.QEquity = 25e9, 7e9, 0.2e9, 50e9
	r.Rating, r.AsOf = "Buy", "2026-09-30"
	r.Trend = Trend{
		Years:     []string{"FY2022", "FY2023", "FY2024", "FY2025"},
		Revenue:   []float64{70e9, 80e9, 90e9, 100e9},
		NetIncome: []float64{10e9, 13e9, 16e9, 20e9},
		FCF:       []float64{10e9, 15e9, 20e9, 25e9},
		OpMargin:  []float64{20, 22, 24, 28},
		Debt:      []float64{12e9, 11e9, 10e9, 10e9},
	}
	return r
}

// emptyRaw es una empresa sin ningún dato (todo NaN).
func emptyRaw() Raw {
	r := Raw{}
	for _, p := range []*float64{&r.Price, &r.MarketCap, &r.Beta, &r.W52Low, &r.W52High, &r.PE, &r.FwdPE, &r.EPS, &r.Dividend,
		&r.RevGrowth, &r.EpsGrowth, &r.ROE, &r.Target, &r.Revenue, &r.NetIncome, &r.OCF, &r.FCF, &r.Debt, &r.Cash,
		&r.CurrentRatio, &r.DebtToEquity, &r.QRevenue, &r.QOpIncome, &r.QInterest, &r.QEquity} {
		*p = nan
	}
	return r
}

func cat(c Company, id string) Category {
	for _, k := range c.Categories {
		if k.ID == id {
			return k
		}
	}
	return Category{}
}

func metric(c Company, id string) Metric {
	for _, k := range c.Categories {
		for _, m := range k.Metrics {
			if m.ID == id {
				return m
			}
		}
	}
	return Metric{}
}

func near(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-6 {
		t.Fatalf("obtuve %v, quiero %v", got, want)
	}
}

func TestHealthyCompanyIsGood(t *testing.T) {
	c := Analyze(healthy(), SP500)
	for _, id := range []string{"prof", "grow", "cash", "debt", "val", "trend"} {
		if v := cat(c, id).Verdict; v != Good {
			t.Errorf("%s: veredicto %s, quiero good", id, v)
		}
	}
	if c.Verdict != Good {
		t.Errorf("veredicto general %s", c.Verdict)
	}
	near(t, metric(c, "net_margin").Value, 20)
	near(t, metric(c, "op_margin").Value, 28)
	near(t, metric(c, "fcf_margin").Value, 25)
	near(t, metric(c, "capex_cover").Value, 6) // capex = 30 − 25 = 5
	near(t, metric(c, "fcf_yield").Value, 2.5)
	near(t, metric(c, "net_debt").Value, -30e9)
	if metric(c, "net_debt").Label != "Caja neta" {
		t.Error("más caja que deuda debe decir «Caja neta»")
	}
	near(t, metric(c, "interest_cover").Value, 35)
	near(t, metric(c, "debt_equity").Value, 0.2)
	near(t, metric(c, "target_upside").Value, 30)
	near(t, metric(c, "range_52w").Value, 50)
}

func TestMissingDataIsNA(t *testing.T) {
	r := emptyRaw()
	r.Ticker, r.Name, r.Price = "X", "X", 10
	c := Analyze(r, SP500)
	if c.Verdict != NA {
		t.Errorf("sin datos el veredicto es %s, quiero na", c.Verdict)
	}
	for _, k := range c.Categories {
		for _, m := range k.Metrics {
			if m.Level == Good || m.Level == Warn || m.Level == Bad {
				t.Errorf("%s: nivel %s sin datos", m.ID, m.Level)
			}
		}
	}
	if m := metric(c, "dividend"); m.Text != "No paga" {
		t.Errorf("sin dividendo debe decir «No paga», dice %q", m.Text)
	}
}

func TestThresholds(t *testing.T) {
	r := healthy()
	r.RevGrowth, r.ROE, r.PE = 2, 70, 35
	r.OCF, r.FCF = 10e9, -5e9 // capex 15 > OCF: cobertura 0,67 y caja libre negativa
	r.Debt, r.Cash = 50e9, 10e9
	c := Analyze(r, SP500)
	if metric(c, "rev_growth").Level != Bad {
		t.Error("crecer 2 % es alerta")
	}
	if m := metric(c, "roe"); m.Label != "Muy alto" || m.Level != Warn {
		t.Errorf("ROE 70 %%: %+v", m)
	}
	if m := metric(c, "pe"); m.Level != Bad || m.Label != "Caro" {
		t.Errorf("P/E 35: %+v", m)
	}
	if metric(c, "fcf").Level != Bad || metric(c, "capex_cover").Level != Bad {
		t.Error("caja libre negativa y cobertura < 1 son alerta")
	}
	// deuda neta 40 con flujo operativo 10 → 4 años: pesada
	near(t, metric(c, "net_debt_years").Value, 4)
	if metric(c, "net_debt_years").Level != Bad {
		t.Error("más de 3 años para pagar la deuda es alerta")
	}
}

func TestEarningsQuality(t *testing.T) {
	r := healthy()
	r.PE, r.FwdPE = 10, 14
	if m := metric(Analyze(r, SP500), "earnings_quality"); m.Level != Warn || m.Note == "" {
		t.Errorf("Forward P/E bien sobre el P/E: %+v", m)
	}
	r.FwdPE = 8
	if m := metric(Analyze(r, SP500), "earnings_quality"); m.Level != Good {
		t.Errorf("Forward P/E menor: %+v", m)
	}
}

func TestPEFromPriceWhenMissing(t *testing.T) {
	r := healthy()
	r.PE = nan
	near(t, metric(Analyze(r, SP500), "pe").Value, 12.5) // 100 / 8
}

func TestFcfYieldSkippedWhenCurrenciesDiffer(t *testing.T) {
	r := healthy()
	r.Currency, r.FinCurrency = "CLP", "USD"
	if m := metric(Analyze(r, IPSA), "fcf_yield"); m.Value != nil || m.Level != NA {
		t.Errorf("moneda distinta: %+v", m)
	}
}

func TestDebtEquityFallsBackToYahooPercent(t *testing.T) {
	r := healthy()
	r.QEquity, r.DebtToEquity = nan, 150
	near(t, metric(Analyze(r, SP500), "debt_equity").Value, 1.5)
}

func TestBankSkipsCashAndDebt(t *testing.T) {
	r := healthy()
	r.Sector, r.CurrentRatio = financialSector, nan
	c := Analyze(r, SP500)
	if !c.Bank {
		t.Fatal("financiera sin liquidez corriente es banco")
	}
	for _, id := range []string{"cash", "debt"} {
		if v := cat(c, id).Verdict; v != NA {
			t.Errorf("%s en un banco: %s, quiero na", id, v)
		}
	}
	for _, id := range []string{"tr_fcf", "tr_debt"} {
		if m := metric(c, id); m.ID != "" {
			t.Errorf("%s no aplica a un banco: %+v", id, m)
		}
	}
	if metric(c, "net_margin").Level != Good || metric(c, "roe").Level != Good {
		t.Error("rentabilidad sí se evalúa en un banco")
	}
	r.CurrentRatio = 1.1 // Visa y Mastercard: informan liquidez, se miden con todo
	if Analyze(r, SP500).Bank {
		t.Error("financiera con liquidez corriente no es banco")
	}
}

func TestFinCurrencyOnlyWhenDifferent(t *testing.T) {
	r := healthy()
	if Analyze(r, SP500).FinCurrency != "" {
		t.Error("misma moneda: sin fin_currency")
	}
	r.Currency, r.FinCurrency = "CLP", "USD"
	if Analyze(r, IPSA).FinCurrency != "USD" {
		t.Error("moneda distinta: fin_currency")
	}
}

func TestVerdict(t *testing.T) {
	cases := []struct {
		in   []Level
		want Level
	}{
		{[]Level{Good, Good, Good}, Good},
		{[]Level{Good, Good, Bad}, Warn}, // promedio 1,33
		{[]Level{Good, Warn, Warn}, Warn},
		{[]Level{Good, Good, Good, Bad}, Warn}, // un malo impide «good»
		{[]Level{Bad, Bad, Warn}, Bad},
		{[]Level{Info, NA}, NA},
		{nil, NA},
	}
	for _, c := range cases {
		if got := Verdict(c.in); got != c.want {
			t.Errorf("Verdict(%v) = %s, quiero %s", c.in, got, c.want)
		}
	}
}

func TestTrend(t *testing.T) {
	c := cat(Analyze(healthy(), SP500), "trend")
	if len(c.Years) != 4 || c.Years[0] != "FY2022" {
		t.Fatalf("años: %v", c.Years)
	}
	var rev Metric
	for _, m := range c.Metrics {
		if m.ID == "tr_revenue" {
			rev = m
		}
	}
	near(t, rev.Value, 100e9)
	if rev.Level != Good || rev.Label != "+12,6 %/año" || len(rev.Series) != 4 { // (100/70)^(1/3) − 1
		t.Errorf("ingresos: %+v", rev)
	}

	r := healthy()
	r.Trend.FCF = []float64{10e9, 8e9, 3e9, -1e9}
	r.Trend.Debt = []float64{10e9, 20e9, 40e9, 80e9}
	c = cat(Analyze(r, SP500), "trend")
	for _, m := range c.Metrics {
		if (m.ID == "tr_fcf" || m.ID == "tr_debt") && m.Level != Bad {
			t.Errorf("%s debería ser alerta: %+v", m.ID, m)
		}
	}
}

func TestTrendWithoutData(t *testing.T) {
	r := healthy()
	r.Trend = Trend{}
	if c := cat(Analyze(r, SP500), "trend"); len(c.Metrics) != 0 || c.Verdict != NA {
		t.Errorf("sin series: %+v", c)
	}
}

func company(ticker, industry string, capB float64, pe, roe float64) Company {
	r := healthy()
	r.Ticker, r.Name, r.Industry, r.MarketCap, r.PE, r.ROE = ticker, ticker, industry, capB*1e9, pe, roe
	return Analyze(r, SP500)
}

func TestPeersPickSameIndustryByCap(t *testing.T) {
	cs := []Company{
		company("A", "Software", 100, 20, 20),
		company("B", "Software", 120, 20, 20),
		company("C", "Software", 10, 20, 20),
		company("D", "Banks", 100, 20, 20),
		company("E", "Software", 90, 20, 20),
	}
	cs[3].Sector = "Finance" // sin otra empresa de su industria ni de su sector
	AttachPeers(cs, 2)
	p := cat(cs[0], "peers").Peers
	if p == nil || len(p.Tickers) != 2 || p.Tickers[0] != "E" && p.Tickers[0] != "B" {
		t.Fatalf("competidores de A: %+v", p)
	}
	for _, tk := range p.Tickers {
		if tk == "D" || tk == "C" || tk == "A" {
			t.Errorf("no debería ser competidor: %s", tk)
		}
	}
	if q := cat(cs[3], "peers"); q.Peers != nil || q.Verdict != NA {
		t.Errorf("D no tiene competidores: %+v", q)
	}
}

func TestPeersLevelsAndBest(t *testing.T) {
	cs := []Company{
		company("ME", "Software", 100, 10, 30), // P/E bajo (mejor), ROE alto (mejor)
		company("P1", "Software", 100, 20, 15),
		company("P2", "Software", 100, 25, 10),
	}
	AttachPeers(cs, 2)
	me := cat(cs[0], "peers").Peers
	for _, row := range me.Rows {
		switch row.ID {
		case "pe":
			if row.Level != Good || row.Best != -1 {
				t.Errorf("P/E: %+v", row)
			}
			near(t, row.Median, 22.5)
		case "roe":
			if row.Level != Good || row.Best != -1 {
				t.Errorf("ROE: %+v", row)
			}
		}
	}
	worst := cat(cs[2], "peers").Peers
	for _, row := range worst.Rows {
		if row.ID == "pe" && row.Level != Bad {
			t.Errorf("P/E de P2 es el peor: %+v", row)
		}
	}
}

func TestPeersOnlyWithinUniverse(t *testing.T) {
	a := company("A", "Software", 100, 20, 20)
	b := company("B", "Software", 100, 20, 20)
	b.Universe = IPSA
	cs := []Company{a, b}
	AttachPeers(cs, 3)
	if cat(cs[0], "peers").Peers != nil {
		t.Error("una empresa del IPSA no compite con una del S&P 500")
	}
}

func TestBuildRawFromYahooShape(t *testing.T) {
	info := map[string]any{
		"longName": "Acme", "sector": "Tech", "industry": "Software", "currency": "USD", "financialCurrency": "USD",
		"regularMarketPrice": 50.0, "marketCap": 5e9, "trailingPE": 20.0, "revenueGrowth": 0.12, "returnOnEquity": 0.3,
		"recommendationKey": "strong_buy", "dividendYield": 2.5,
	}
	pt := func(date string, v float64) map[string]any {
		return map[string]any{"asOfDate": date, "reportedValue": map[string]any{"raw": v}}
	}
	series := func(typ string, pts ...map[string]any) map[string]any {
		arr := make([]any, len(pts))
		for i, p := range pts {
			arr[i] = p
		}
		return map[string]any{"meta": map[string]any{"type": []any{typ}}, typ: arr}
	}
	ts := map[string]any{"timeseries": map[string]any{"result": []any{
		series("annualTotalRevenue", pt("2024-12-31", 90), pt("2025-12-31", 100)),
		series("annualOperatingIncome", pt("2024-12-31", 18), pt("2025-12-31", 25)),
		series("annualNetIncome", pt("2025-12-31", 20)),
		series("quarterlyTotalRevenue", pt("2025-06-30", 24), pt("2025-09-30", 26)),
		series("quarterlyOperatingIncome", pt("2025-09-30", 6.5)),
		series("quarterlyInterestExpense", pt("2025-09-30", 1.3)),
	}}}
	r := BuildRaw("ACME", info, ts)
	near(t, &r.RevGrowth, 12)
	near(t, &r.ROE, 30)
	near(t, &r.QRevenue, 26)
	near(t, &r.QOpIncome, 6.5)
	near(t, &r.QInterest, 1.3)
	if r.AsOf != "2025-09-30" || r.Rating != "Strong Buy" || r.Name != "Acme" {
		t.Errorf("%+v", r)
	}
	if !isNaN(r.QEquity) || !isNaN(r.FCF) {
		t.Error("lo que Yahoo no informa debe quedar en NaN")
	}
	if len(r.Trend.Years) != 2 || r.Trend.Years[1] != "FY2025" {
		t.Fatalf("años: %v", r.Trend.Years)
	}
	near(t, &r.Trend.OpMargin[1], 25)
	near(t, &r.Trend.OpMargin[0], 20)
	if !isNaN(r.Trend.NetIncome[0]) {
		t.Error("año sin utilidad debe quedar en NaN")
	}
}

func TestWriteProducesValidJSON(t *testing.T) {
	cs := []Company{company("A", "Software", 100, 20, 20), company("B", "Software", 50, 25, 15)}
	p := Build(cs, 3)
	path := filepath.Join(t.TempDir(), "sub", "evaluator.json")
	if err := Write(path, p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Universes []UniverseInfo `json:"universes"`
		Metrics   map[string]MetricDef
		Companies []struct {
			Ticker     string `json:"ticker"`
			Categories []struct {
				ID string `json:"id"`
			} `json:"categories"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Universes) != 1 || back.Universes[0].Count != 2 || back.Companies[0].Ticker != "A" {
		t.Errorf("universos o empresas: %+v", back)
	}
	if n := len(back.Companies[0].Categories); n != 7 {
		t.Errorf("categorías: %d, quiero 7", n)
	}
	// toda métrica que sale en una empresa debe estar en el diccionario
	for _, c := range p.Companies {
		for _, k := range c.Categories {
			for _, m := range k.Metrics {
				if _, ok := Metrics[m.ID]; !ok {
					t.Errorf("métrica %s sin definición en el catálogo", m.ID)
				}
			}
		}
	}
}
