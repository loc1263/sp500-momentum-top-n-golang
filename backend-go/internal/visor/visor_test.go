// Pruebas del Visor: retornos, YTD, drawdown máximo, beta, ETFs apalancados, formato y clasificación
// de acciones.
//
// Fecha: 2026-11-19

package visor

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"top/internal/config"
)

func series(start string, values ...float64) Series {
	d, _ := time.Parse(time.DateOnly, start)
	var s Series
	for _, v := range values {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		s.Dates = append(s.Dates, d.Format(time.DateOnly))
		s.Close = append(s.Close, v)
		d = d.AddDate(0, 0, 1)
	}
	return s
}

func near(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-9 {
		t.Errorf("%s = %v, quiero %v", name, got, want)
	}
}

func TestPeriodReturn(t *testing.T) {
	p := []float64{100, 110, 121}
	near(t, "1 rueda", PeriodReturn(p, 1), 10)
	near(t, "2 ruedas", PeriodReturn(p, 2), 21)
	if PeriodReturn([]float64{100, 110}, 2) != nil {
		t.Error("sin historial debe dar nil")
	}
}

func TestYTDUsaElUltimoCierreDelAnioAnterior(t *testing.T) {
	s := series("2025-12-29", 100, 100, 120, 150)
	if s.Dates[2][:4] != "2025" || s.Dates[3][:4] != "2026" {
		t.Fatalf("fechas inesperadas: %v", s.Dates)
	}
	near(t, "ytd", YTD(s), 25)
	if YTD(series("2026-01-05", 100, 110)) != nil {
		t.Error("sin año anterior debe dar nil")
	}
}

func TestMaxDrawdown(t *testing.T) {
	if got := MaxDrawdown([]float64{100, 80, 90, 120, 90}); math.Abs(got+25) > 1e-9 {
		t.Errorf("max drawdown = %v, quiero -25", got)
	}
}

func TestBetaDeUnaSerieIgualADobleDeSpyEsDos(t *testing.T) {
	n := TradingDays*3 + 10
	spy, twice := make([]float64, n), make([]float64, n)
	spy[0], twice[0] = 100, 100
	for i := 1; i < n; i++ {
		r := 0.01 * math.Sin(float64(i))
		spy[i], twice[i] = spy[i-1]*(1+r), twice[i-1]*(1+2*r)
	}
	s, ref := series("2020-01-01", twice...), series("2020-01-01", spy...)
	m, ok := Compute(s, nil, ReturnsOf(ref, TradingDays*3))
	if !ok {
		t.Fatal("Compute falló")
	}
	if m.Beta == nil || math.Abs(*m.Beta-2) > 0.01 {
		t.Errorf("beta = %v, quiero ≈ 2", m.Beta)
	}
	if m.Volatility == nil || m.CAGR3 == nil || m.VsMA200 == nil {
		t.Error("con más de 3 años deben calcularse volatilidad, CAGR y media de 200")
	}
}

func TestComputeSinHistorialLargoDejaNilLoQueNoPuede(t *testing.T) {
	m, ok := Compute(series("2026-01-05", 100, 101, 102), map[string]float64{"2026-01-06": 1}, Returns{})
	if !ok {
		t.Fatal("con dos cierres o más debe calcular")
	}
	if m.Beta != nil || m.Volatility != nil || m.VsMA200 != nil || m.RetYears[1] != nil {
		t.Error("lo que necesita más historial debe quedar nil")
	}
	near(t, "dividend yield", m.DivYield, 1.0/102*100)
	if _, ok := Compute(series("2026-01-05", 100), nil, Returns{}); ok {
		t.Error("con un solo cierre no hay métricas")
	}
}

func TestIsLeveraged(t *testing.T) {
	for _, c := range [][2]string{
		{"ProShares UltraPro QQQ", "Trading--Leveraged Equity"},
		{"Direxion Daily S&P 500 Bear 3X Shares", "Trading--Inverse Equity"},
		{"ProShares Short S&P500", "Trading--Inverse Equity"},
	} {
		if !IsLeveraged(c[0], c[1]) {
			t.Errorf("%q debe ser apalancado", c[0])
		}
	}
	for _, c := range [][2]string{
		{"SPDR S&P 500 ETF Trust", "Large Blend"},
		{"Vanguard Short-Term Bond ETF", "Short-Term Bond"},
		{"PIMCO Ultra-Short Income ETF", "Ultrashort Bond"},
	} {
		if IsLeveraged(c[0], c[1]) {
			t.Errorf("%q no es apalancado", c[0])
		}
	}
}

func TestIsSignColYFormat(t *testing.T) {
	for _, c := range []string{"ret_3m", "ytd", "rel_spy_1y", "vs_ma200", "ytd_vs_spy"} {
		if !IsSignCol(c) {
			t.Errorf("%s debe ser de signo", c)
		}
	}
	for _, c := range []string{"ytd_spy", "sharpe"} {
		if IsSignCol(c) {
			t.Errorf("%s no debe ser de signo", c)
		}
	}
	if Format("aum_B") != "int" || Format("sharpe") != "dec" || Format("dividend_yield") != "pct" || Format("nombre") != "" {
		t.Error("formatos inesperados")
	}
}

func f(v float64) *float64 { return &v }

func instrument(ticker string, over func(*Instrument)) Instrument {
	in := Instrument{
		Ticker: ticker, Static: Static{Name: "Demo", MarketCap: f(250e9)},
		Metrics: Metrics{
			YTD: f(12), CAGR3: f(13), Volatility: f(10), MaxDrawdown: f(-20),
			RetMonths: map[int]*float64{}, RetYears: map[int]*float64{1: f(20), 3: f(60)},
		},
	}
	if over != nil {
		over(&in)
	}
	return in
}

var testRef = Ref{YTD: 8, Rets: map[int]*float64{1: f(10), 3: f(40)}}

func col(sec Section, name string) int {
	for i, c := range sec.Cols {
		if c == name {
			return i
		}
	}
	return -1
}

func TestBuildAccionesFiltraYClasifica(t *testing.T) {
	ins := []Instrument{
		instrument("AAA", func(i *Instrument) { i.MarketCap = f(150e9) }),
		instrument("BBB", nil),
		instrument("CCC", func(i *Instrument) { i.MarketCap = f(50e9) }),
		instrument("PREF", func(i *Instrument) { i.MarketCap = nil }),
	}
	sec := Build(Profiles[0], ins, testRef, config.Default().Visor, nil)
	if len(sec.Rows) != 2 || sec.Rows[0][0] != "BBB" || sec.Rows[1][0] != "AAA" {
		t.Fatalf("filas inesperadas (ordenadas por capitalización): %v", sec.Rows)
	}
	cat := col(sec, "categoria")
	if sec.Rows[0][cat] != "Mega cap" || sec.Rows[1][cat] != "Super large cap" {
		t.Errorf("categorías: %v, %v", sec.Rows[0][cat], sec.Rows[1][cat])
	}
	row := sec.Rows[0]
	if got := row[col(sec, "sharpe")].(float64); math.Abs(got-1) > 1e-9 {
		t.Errorf("sharpe = %v, quiero 1 ((13-3)/10)", got)
	}
	if row[col(sec, "ytd_vs_spy")] != 4.0 || row[col(sec, "vs_spy")] != Beats {
		t.Errorf("ytd vs spy: %v %v", row[col(sec, "ytd_vs_spy")], row[col(sec, "vs_spy")])
	}
	if row[col(sec, "rel_spy_1y")] != 10.0 || row[col(sec, "rel_spy_3y")] != 20.0 {
		t.Errorf("relativos: %v %v", row[col(sec, "rel_spy_1y")], row[col(sec, "rel_spy_3y")])
	}
}

func TestBuildETFDescartaApalancadosSalvoLosPedidos(t *testing.T) {
	etf := func(tk string, aum float64, lev bool) Instrument {
		return instrument(tk, func(i *Instrument) { i.MarketCap, i.AUM, i.Leveraged = nil, f(aum), lev })
	}
	ins := []Instrument{etf("SPY", 800e9, false), etf("LEV", 30e9, true), etf("MINE", 20e9, true)}
	tickers := func(s Section) (out []string) {
		for _, r := range s.Rows {
			out = append(out, r[0].(string))
		}
		return
	}
	cfg := config.Default().Visor
	got := tickers(Build(Profiles[1], ins, testRef, cfg, map[string]bool{"MINE": true}))
	if len(got) != 2 || got[0] != "SPY" || got[1] != "MINE" {
		t.Errorf("sin apalancados: %v", got)
	}
	cfg.IncludeLeveraged = true
	if got := tickers(Build(Profiles[1], ins, testRef, cfg, nil)); len(got) != 3 {
		t.Errorf("con apalancados: %v", got)
	}
}

func TestVerdesCuentaConLasReglasDelFrontend(t *testing.T) {
	v := map[string]any{
		"ret_1m": 5.0, "ret_2m": -2.0, "ret_3m": nil,
		"sharpe":       1.0,
		"max_drawdown": -20.0, "expense_ratio": 0.1, "dividend_yield": 1.0,
		"vs_spy": Beats,
	}
	if got := greens(v); got != 4 {
		t.Errorf("verdes = %d, quiero 4", got)
	}
}

func TestWriteEscribeJSONValido(t *testing.T) {
	sec := Build(Profiles[0], []Instrument{instrument("AAA", func(i *Instrument) { i.Name = "Demo </script>" })}, testRef, config.Default().Visor, nil)
	path := filepath.Join(t.TempDir(), "sub", "visor.json")
	if err := Write(path, &Payload{Ref: "SPY", Sections: []Section{sec}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var back Payload
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(back.Sections) != 1 || back.Sections[0].Rows[0][0] != "AAA" || back.Sections[0].Thr["sharpe"][0] != ">" {
		t.Errorf("contenido inesperado: %+v", back.Sections)
	}
}
