// Pruebas del evaluador: empresa sana, datos faltantes, umbrales, calidad de ganancias, P/E, FCF,
// deuda/capital y bancos.
//
// Fecha: 2026-11-19

package evaluator

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/config"
)

func healthy() Raw {
	r := emptyRaw()
	r.Ticker, r.Name, r.Sector, r.Industry, r.Currency, r.FinCurrency = "OK", "Ok Corp", "Tech", "Software", "USD", "USD"
	r.Price, r.MarketCap, r.Beta, r.W52Low, r.W52High = 100, 300e9, 0.9, 80, 110
	r.PE, r.FwdPE, r.EPS, r.Dividend = 12, 10, 8, 1.5
	r.RevGrowth, r.EpsGrowth, r.ROE, r.Target = 12, 15, 20, 130
	r.Revenue, r.NetIncome, r.OCF, r.FCF, r.Capex, r.Debt, r.Cash = 100e9, 20e9, 30e9, 25e9, 5e9, 10e9, 40e9
	r.CurrentRatio = 2
	r.SMA200, r.Ret3M, r.Ret6M, r.IdxRet3M, r.IdxRet6M = 90, 8, 12, 5, 6
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

func emptyRaw() Raw {
	r := Raw{}
	for _, p := range []*float64{&r.Price, &r.MarketCap, &r.Beta, &r.W52Low, &r.W52High, &r.PE, &r.FwdPE, &r.EPS, &r.Dividend,
		&r.RevGrowth, &r.EpsGrowth, &r.ROE, &r.Target, &r.Revenue, &r.NetIncome, &r.OCF, &r.FCF, &r.Capex, &r.Debt, &r.Cash,
		&r.CurrentRatio, &r.DebtToEquity, &r.EBITDA, &r.EBIT, &r.TaxRate, &r.InvestedCapital, &r.GrossMargin, &r.PB, &r.PEG, &r.Payout, &r.SMA200, &r.Ret3M, &r.Ret6M, &r.IdxRet3M, &r.IdxRet6M, &r.QRevenue, &r.QOpIncome, &r.QInterest, &r.QEquity} {
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
	near(t, metric(c, "capex_cover").Value, 6)
	near(t, metric(c, "fcf_yield").Value, 25.0/300*100)
	near(t, metric(c, "net_debt").Value, -30e9)
	if metric(c, "net_debt").Label != "Caja neta" {
		t.Error("más caja que deuda debe decir «Caja neta»")
	}
	near(t, metric(c, "interest_cover").Value, 35)
	near(t, metric(c, "debt_equity").Value, 0.2)
	near(t, metric(c, "target_upside").Value, 30)
	near(t, metric(c, "range_52w").Value, 20.0/30*100)
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
	r.OCF, r.FCF, r.Capex = 10e9, -5e9, 15e9
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
	near(t, metric(Analyze(r, SP500), "pe").Value, 12.5)
}

func TestFcfYieldSkippedWhenCurrenciesDiffer(t *testing.T) {
	r := healthy()
	r.Currency, r.FinCurrency = "EUR", "USD"
	if m := metric(Analyze(r, SP500), "fcf_yield"); m.Value != nil || m.Level != NA {
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
	r.CurrentRatio = 1.1
	if Analyze(r, SP500).Bank {
		t.Error("financiera con liquidez corriente no es banco")
	}
}

func TestFinCurrencyOnlyWhenDifferent(t *testing.T) {
	r := healthy()
	if Analyze(r, SP500).FinCurrency != "" {
		t.Error("misma moneda: sin fin_currency")
	}
	r.Currency, r.FinCurrency = "EUR", "USD"
	if Analyze(r, SP500).FinCurrency != "USD" {
		t.Error("moneda distinta: fin_currency")
	}
}

func TestVerdict(t *testing.T) {
	cases := []struct {
		in   []Level
		want Level
	}{
		{[]Level{Good, Good, Good}, Good},
		{[]Level{Good, Good, Bad}, Warn},
		{[]Level{Good, Warn, Warn}, Warn},
		{[]Level{Good, Good, Good, Bad}, Warn},
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
	if rev.Level != Good || rev.Label != "+12,6 %/año" || len(rev.Series) != 4 {
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

func TestNewIndicators(t *testing.T) {
	r := healthy()
	r.EBITDA, r.EBIT, r.TaxRate, r.InvestedCapital = 30e9, 25e9, 0.2, 100e9
	r.GrossMargin, r.PB, r.PEG, r.Payout = 60, 3, 1.5, 50
	c := Analyze(r, SP500)

	// EV = 300 + 10 − 40 = 270 mil millones.
	for _, tc := range []struct {
		id    string
		value float64
		level Level
		label string
	}{
		{"ev_ebitda", 9, Good, "Barato"},
		{"ev_fcf", 10.8, Good, "Barato"},
		{"peg", 1.5, Warn, "Medio"},
		{"ps", 3, Info, ""},
		{"pb", 3, Info, ""},
		{"roic", 20, Good, ""},
		{"gross_margin", 60, Info, ""},
		{"net_debt_ebitda", 0, Good, ""},
		{"payout", 50, Good, ""},
	} {
		m := metric(c, tc.id)
		near(t, m.Value, tc.value)
		if m.Level != tc.level || m.Label != tc.label {
			t.Errorf("%s: %+v", tc.id, m)
		}
	}

	r.EBITDA, r.FCF = -5e9, -5e9
	r.Debt, r.Cash = 50e9, 10e9
	r.Payout, r.PEG = 90, -1
	c = Analyze(r, SP500)
	if m := metric(c, "ev_ebitda"); m.Level != Bad || m.Label != "Sin utilidad" {
		t.Errorf("EBITDA negativo: %+v", m)
	}
	if m := metric(c, "ev_fcf"); m.Level != Bad || m.Label != "Sin caja libre" {
		t.Errorf("FCF negativo: %+v", m)
	}
	if m := metric(c, "peg"); m.Level != Bad || m.Label != "Sin crecimiento" {
		t.Errorf("PEG negativo: %+v", m)
	}
	if m := metric(c, "payout"); m.Level != Bad {
		t.Errorf("payout 90 %%: %+v", m)
	}
	if m := metric(c, "net_debt_ebitda"); m.Value != nil {
		t.Errorf("sin EBITDA positivo no hay cociente: %+v", m)
	}

	r.EBITDA, r.Dividend = 10e9, 0
	c = Analyze(r, SP500)
	near(t, metric(c, "net_debt_ebitda").Value, 4)
	if m := metric(c, "net_debt_ebitda"); m.Level != Bad {
		t.Errorf("4× es alerta: %+v", m)
	}
	if m := metric(c, "payout"); m.Value != nil || m.Level != NA {
		t.Errorf("sin dividendo no hay payout: %+v", m)
	}
}

func TestNewIndicatorsNeedComparableData(t *testing.T) {
	r := healthy()
	r.EBITDA, r.EBIT, r.TaxRate, r.InvestedCapital = 30e9, 25e9, 0.2, 100e9
	r.FinCurrency = "EUR"
	c := Analyze(r, SP500)
	for _, id := range []string{"ev_ebitda", "ev_fcf", "ps"} {
		if m := metric(c, id); m.Value != nil {
			t.Errorf("%s con monedas distintas debe quedar sin dato: %+v", id, m)
		}
	}

	b := healthy()
	b.Sector, b.CurrentRatio = financialSector, nan
	b.EBITDA, b.EBIT, b.TaxRate, b.InvestedCapital = 30e9, 25e9, 0.2, 100e9
	c = Analyze(b, SP500)
	for _, id := range []string{"ev_ebitda", "ev_fcf", "roic", "net_debt_ebitda"} {
		if m := metric(c, id); m.Value != nil {
			t.Errorf("%s no aplica a un banco: %+v", id, m)
		}
	}
}

func TestNewTrendSeries(t *testing.T) {
	r := healthy()
	r.Trend.EPS = []float64{2, 2.5, 3, 3.5}
	r.Trend.GrossMargin = []float64{50, 52, 55, 60}
	c := cat(Analyze(r, SP500), "trend")
	var eps, gross Metric
	for _, m := range c.Metrics {
		switch m.ID {
		case "tr_eps":
			eps = m
		case "tr_gross_margin":
			gross = m
		}
	}
	near(t, eps.Value, 3.5)
	if eps.Level != Good || eps.Label != "+20,5 %/año" {
		t.Errorf("EPS: %+v", eps)
	}
	near(t, gross.Value, 60)
	if gross.Level != Good || gross.Label != "+10,0 pp" {
		t.Errorf("margen bruto: %+v", gross)
	}
}

func TestBuildRawZeroGrossMarginIsMissing(t *testing.T) {
	r := BuildRaw("BANK", map[string]any{"grossMargins": 0.0, "regularMarketPrice": 10.0}, nil)
	if !isNaN(r.GrossMargin) {
		t.Errorf("margen bruto 0 de Yahoo debe quedar sin dato: %v", r.GrossMargin)
	}
}

func TestScreenSolidAndCheap(t *testing.T) {
	healthy := func() Raw {
		r := healthy()
		r.EBITDA = 30e9
		return r
	}
	solid, cheap := Screen(Analyze(healthy(), SP500))
	if !solid || !cheap {
		t.Errorf("empresa sana y barata: sólida=%v barata=%v", solid, cheap)
	}

	r := healthy()
	r.PE = 35
	if solid, cheap := Screen(Analyze(r, SP500)); !solid || cheap {
		t.Errorf("con un P/E caro no es barata: sólida=%v barata=%v", solid, cheap)
	}

	r = healthy()
	r.Debt, r.Cash = nan, nan
	if _, cheap := Screen(Analyze(r, SP500)); cheap {
		t.Error("con un solo múltiplo disponible no se puede decir que es barata")
	}

	r = healthy()
	r.Debt, r.Cash, r.OCF = 200e9, 10e9, 30e9 // deuda neta 6,3 veces el EBITDA
	if solid, _ := Screen(Analyze(r, SP500)); solid {
		t.Error("con deuda pesada no es sólida")
	}

	r = healthy()
	r.QInterest = 5e9 // el resultado operativo (7 mil millones) cubre los intereses solo 1,4 veces
	if solid, _ := Screen(Analyze(r, SP500)); solid {
		t.Error("con poca cobertura de intereses no es sólida")
	}

	// Como Visa o Mastercard: liquidez corriente bajo 1 y deuda/patrimonio alto, pero deuda neta y cobertura holgadas.
	r = healthy()
	r.CurrentRatio, r.DebtToEquity, r.QEquity = 0.9, 350, 10e9
	if c := Analyze(r, SP500); cat(c, "debt").Verdict == Good {
		t.Fatalf("el caso debe tener Deuda y liquidez fuera de «Bien»: %+v", cat(c, "debt"))
	}
	if solid, _ := Screen(Analyze(r, SP500)); !solid {
		t.Error("liquidez corriente baja y deuda/patrimonio alto no impiden ser sólida si la deuda neta y la cobertura son holgadas")
	}

	r = healthy()
	r.EBITDA = nan
	if solid, _ := Screen(Analyze(r, SP500)); solid {
		t.Error("sin EBITDA no se puede medir la solvencia")
	}

	b := healthy()
	b.Sector, b.CurrentRatio = financialSector, nan
	if solid, _ := Screen(Analyze(b, SP500)); solid {
		t.Error("un banco no tiene sección de deuda y no califica como sólido")
	}
}

func TestPriceTrendMetrics(t *testing.T) {
	c := Analyze(healthy(), SP500)
	// Precio 100 contra SMA 200 de 90; máximo de 52 semanas 110.
	near(t, metric(c, "sma200").Value, (100.0/90-1)*100)
	if m := metric(c, "sma200"); m.Level != Good || m.Label != "Sobre la SMA 200" {
		t.Errorf("sma200: %+v", m)
	}
	for _, id := range []string{"ret_3m", "ret_6m"} {
		if m := metric(c, id); m.Level != Good || m.Label != "Positivo" || m.Note == "" {
			t.Errorf("%s: %+v", id, m)
		}
	}
	near(t, metric(c, "off_high").Value, 10.0/110*100)
	if m := metric(c, "off_high"); m.Level != Good || m.Label != "Cerca del máximo" {
		t.Errorf("off_high: %+v", m)
	}

	r := healthy()
	r.SMA200 = 120
	r.W52High = 125
	c = Analyze(r, SP500)
	if m := metric(c, "sma200"); m.Level != Bad || m.Label != "Bajo la SMA 200" {
		t.Errorf("bajo la media: %+v", m)
	}
	if m := metric(c, "off_high"); m.Level != Bad || m.Label != "Lejos del máximo" {
		t.Errorf("20 %% bajo el máximo: %+v", m)
	}
	r.W52High = 115 // 13 % bajo el máximo
	if m := metric(Analyze(r, SP500), "off_high"); m.Level != Warn {
		t.Errorf("13 %% bajo el máximo es atención: %+v", m)
	}
}

func TestReturnLevel(t *testing.T) {
	for _, tc := range []struct {
		ret, idx float64
		level    Level
		label    string
	}{
		{8, 5, Good, "Positivo"},
		{3, 5, Warn, "Bajo el índice"},
		{-2, -5, Warn, "Mejor que el índice"},
		{-8, -5, Bad, "Negativo"},
		{4, nan, Good, "Positivo"},
		{-4, nan, Bad, "Negativo"},
		{nan, 5, NA, ""},
	} {
		if l, lb := returnLevel(tc.ret, tc.idx); l != tc.level || lb != tc.label {
			t.Errorf("rendimiento %v contra índice %v: %s %q, quiero %s %q", tc.ret, tc.idx, l, lb, tc.level, tc.label)
		}
	}
}

func TestBarsIndicators(t *testing.T) {
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	var bars []gohoo.Bar
	for i := 0; i < 260; i++ {
		bars = append(bars, gohoo.Bar{Time: day.AddDate(0, 0, i), Close: 100 + float64(i)})
	}
	last := bars[len(bars)-1] // cierre 359
	// SMA de los últimos 200 cierres: de 160 a 359 → 259,5.
	near(t, &[]float64{sma(bars, 200)}[0], 259.5)
	if !isNaN(sma(bars[:150], 200)) {
		t.Error("con menos de 200 ruedas no hay SMA 200")
	}
	// Hace 3 meses desde 2026-09-18 es 2026-06-18, el día 167 (cierre 267).
	want := (last.Close/267 - 1) * 100
	near(t, &[]float64{returnSince(bars, 3)}[0], want)
	if !isNaN(returnSince(bars[200:], 6)) {
		t.Error("si el historial no llega a 6 meses atrás, no hay rendimiento")
	}

	r := emptyRaw()
	r.applyBars(bars, Benchmark{Ret3M: 5, Ret6M: nan})
	near(t, &r.SMA200, 259.5)
	near(t, &r.IdxRet3M, 5)
	if !isNaN(r.IdxRet6M) {
		t.Error("sin índice a 6 meses debe quedar en NaN")
	}
}

func TestTrendWithoutData(t *testing.T) {
	r := healthy()
	r.Trend = Trend{}
	if c := cat(Analyze(r, SP500), "trend"); len(c.Metrics) != 0 || c.Verdict != NA {
		t.Errorf("sin series: %+v", c)
	}
}

func mkCompany(ticker, industry string, capB float64, pe, roe float64) Company {
	r := healthy()
	r.Ticker, r.Name, r.Industry, r.MarketCap, r.PE, r.ROE = ticker, ticker, industry, capB*1e9, pe, roe
	return Analyze(r, SP500)
}

func TestPeersPickSameIndustryByCap(t *testing.T) {
	cs := []Company{
		mkCompany("A", "Software", 100, 20, 20),
		mkCompany("B", "Software", 120, 20, 20),
		mkCompany("C", "Software", 10, 20, 20),
		mkCompany("D", "Banks", 100, 20, 20),
		mkCompany("E", "Software", 90, 20, 20),
	}
	cs[3].Sector = "Finance"
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
		mkCompany("ME", "Software", 100, 10, 30),
		mkCompany("P1", "Software", 100, 20, 15),
		mkCompany("P2", "Software", 100, 25, 10),
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
		series("trailingOperatingCashFlow", pt("2025-09-30", 16)),
		series("trailingFreeCashFlow", pt("2025-09-30", 14)),
		series("trailingCapitalExpenditure", pt("2025-09-30", -2)),
		series("trailingEBITDA", pt("2025-09-30", 30)),
		series("trailingEBIT", pt("2025-09-30", 25)),
		series("quarterlyInvestedCapital", pt("2025-09-30", 100)),
		series("quarterlyTaxRateForCalcs", pt("2025-09-30", 0.2)),
		series("annualGrossProfit", pt("2024-12-31", 45), pt("2025-12-31", 60)),
		series("annualDilutedEPS", pt("2024-12-31", 2), pt("2025-12-31", 2.4)),
	}}}
	info["freeCashflow"], info["operatingCashflow"] = 3.0, 16.0
	info["ebitda"], info["grossMargins"], info["priceToBook"], info["pegRatio"], info["payoutRatio"] = 1.0, 0.6, 4.0, 1.8, 0.45
	r := BuildRaw("ACME", info, ts)
	near(t, &r.EBITDA, 30)
	near(t, &r.EBIT, 25)
	near(t, &r.InvestedCapital, 100)
	near(t, &r.TaxRate, 0.2)
	near(t, &r.GrossMargin, 60)
	near(t, &r.PB, 4)
	near(t, &r.PEG, 1.8)
	near(t, &r.Payout, 45)
	near(t, &r.Trend.GrossMargin[0], 50)
	near(t, &r.Trend.GrossMargin[1], 60)
	near(t, &r.Trend.EPS[1], 2.4)
	near(t, &r.OCF, 16)
	near(t, &r.FCF, 14)
	near(t, &r.Capex, 2)
	near(t, &r.RevGrowth, 12)
	near(t, &r.ROE, 30)
	near(t, &r.QRevenue, 26)
	near(t, &r.QOpIncome, 6.5)
	near(t, &r.QInterest, 1.3)
	if r.AsOf != "2025-09-30" || r.Rating != "Strong Buy" || r.Name != "Acme" {
		t.Errorf("%+v", r)
	}
	if !isNaN(r.QEquity) {
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

func TestEmptyUniverses(t *testing.T) {
	cfg := config.Default()
	if got := emptyUniverses([]target{{"AAPL", SP500}}, cfg); len(got) != 0 {
		t.Errorf("con la lista: %v", got)
	}
	if got := emptyUniverses(nil, cfg); len(got) != 1 || got[0] != "S&P 500" {
		t.Errorf("sin nada: %v", got)
	}
	cfg.Evaluator.Sp500 = false
	if got := emptyUniverses(nil, cfg); len(got) != 0 {
		t.Errorf("S&P 500 desactivado: %v", got)
	}
}

func TestWriteReplacesAtomicallyAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evaluator.json")
	if err := os.WriteFile(path, []byte("viejo"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := Build([]Company{mkCompany("A", "Software", 100, 20, 20)}, 3)
	if err := Write(path, p); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !json.Valid(raw) {
		t.Fatalf("no es JSON: %.40s", raw)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(left) != 0 {
		t.Errorf("quedaron temporales: %v", left)
	}
	if err := Write(filepath.Join(path, "sub", "x.json"), p); err == nil {
		t.Error("debería fallar")
	}
	if after, _ := os.ReadFile(path); !json.Valid(after) {
		t.Error("el JSON anterior se perdió")
	}
}

func TestWriteProducesValidJSON(t *testing.T) {
	cs := []Company{mkCompany("A", "Software", 100, 20, 20), mkCompany("B", "Software", 50, 25, 15)}
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
