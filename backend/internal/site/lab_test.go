// Pruebas de Quant, del Lab (backtest, plazos y N-0 vs N-1) y del gráfico SVG, con JSON de ejemplo.
//
// Fecha: 2026-11-19

package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const quantJSON = `{
 "as_of":"2026-11-18",
 "settings":{"top_n":2,"vol_target":0.2,"w_max":0.25,"w_min":0.1,"w_step":0.01,"sector_max":0.4,"mu_shrink":0.25,"risk_free":0.04,"prior":0.09,"weights":[1,2,1]},
 "coverage":{"universe":3,"ranked":2,"excluded":{"sin_momentum":["NEW"],"sin_datos_financieros":[],"sin_datos_tecnicos":[]}},
 "ranking":[
  {"rank":1,"ticker":"REGN","name":"Regeneron <X>","sector":"Healthcare","score":88.44,"momentum":75.7,"health":94.2,"technical":95.4,"groups":4,
   "health_groups":{"caja":56.1,"liquidez":93.9,"rentabilidad":67,"solvencia":93.8},"technical_groups":{"impulso":82,"posicion":93.4,"tendencia":95.1,"volumen":88.1},
   "rsi":67,"macd_hist":0.0016,"vs_ema50":0.124,"vs_ema200":0.258,"mom_12_1":0.152,"mom_6_1":0.244,"roe":0.149,"operating_margin":0.2,"debt_to_equity":55,
   "current_ratio":1.9,"fcf_margin":0.1,"fcf_yield":0.03,"cash_conversion":0.9,"fcf_positive_years":0.75},
  {"rank":2,"ticker":"ABC","name":"Abc","sector":"Real Estate","score":null,"momentum":null,"health":null,"technical":null,"groups":2,"health_groups":{},"technical_groups":{},"rsi":null}
 ],
 "markowitz":{
  "tickers":[{"ticker":"REGN","exp_return":0.1587,"vol":0.34},{"ticker":"ABC","exp_return":null,"vol":null}],
  "portfolios":[{"key":"max_sharpe","label":"Máx. Sharpe","exp_return":0.12,"vol":0.2,"sharpe":0.4,"weights":[{"ticker":"REGN","weight":0.25}],"sectors":[{"sector":"Healthcare","weight":0.4}]}],
  "min_vol":0.18,"vol_target":0.2,"w_max":0.25,"sector_max":0.4,"sector_relaxed":true,"main":"max_sharpe"}
}`

// term arma un plazo con dos calendarios; halves es el JSON de las mitades.
func term(months int, median, base float64, halves string) string {
	cal := func(offset int, final, cagr float64, history string) string {
		return `{"offset":` + itoa(offset) + `,"start_date":"2024-01-01","end_date":"2026-01-01","final_capital":` + fnum(final) + `,"total_return":0.3333,"cagr":` + fnum(cagr) +
			`,"rebalances":6,"first_rebalance":"2024-04-01","equity":[{"date":"2024-01-01","capital":15000},{"date":"2025-01-01","capital":17000},{"date":"2026-01-01","capital":` + fnum(final) +
			`}],"history":` + history + `}`
	}
	hist := `[{"date":"2024-04-01","picks":[{"rank":1,"ticker":"AAPL","return":0.2},{"rank":2,"ticker":"BRK-B","return":-0.1}],"capital":15500,"gain":0.0333}]`
	return `{"months":` + itoa(months) + `,"calendars":[` + cal(0, 20000, base, hist) + `,` + cal(1, 18000, base-0.05, `[]`) + `],"summary":{"calendars":2,"cagr_median":` + fnum(median) +
		`,"cagr_min":0.1,"cagr_max":0.15,"capital_median":19000,"capital_min":18000,"capital_max":20000,"halves":` + halves + `}}`
}

func itoa(n int) string { return fnum(float64(n)) }

func labReport() string {
	period := func(terms ...string) string {
		return `{"years":2,"start_date":"2024-01-01","end_date":"2026-01-01","terms":[` + strings.Join(terms, ",") + `]}`
	}
	uni := func(p string) string {
		return `[{"min_cap_billions":200,"label":"mega-cap","periods":[` + p + `]}]`
	}
	r := reportJSON[:strings.LastIndex(reportJSON, "}")]
	return r + `,"quant":` + quantJSON +
		`,"backtest":` + uni(period(term(4, 0.12, 0.15, `{"split":"2025-01-01","first":0.1,"second":0.2}`), term(6, 0.08, 0.2, `{"split":"2025-01-01","first":0.1,"second":0.2}`))) +
		`,"backtest_skip":` + uni(period(term(4, 0.14, 0.15, `{"split":"2025-01-01","first":0.12,"second":0.25}`), term(6, 0.05, 0.1, `{"split":"2025-01-01","first":null,"second":null}`))) + `}`
}

func TestQuant(t *testing.T) {
	src, out := t.TempDir(), t.TempDir()
	in, err := LoadInputs(Paths{Report: writeFile(t, src, "report.json", labReport())})
	if err != nil || in.Quant == nil {
		t.Fatalf("LoadInputs: %v, %+v", err, in.Quant)
	}
	if _, err := Build(in, out); err != nil {
		t.Fatal(err)
	}
	quant := page(t, out, "quant")
	wantAll(t, "quant", quant,
		"Regeneron <X>", `"Views":"ind tec"`, `"Views":"sal"`, `"Views":"tec"`, `"Class":"top"`, "88,4", "n/d", "(pesos 1 / 2 / 1)",
		"Sin un año de precios (no tienen momentum): NEW.", "No hay cartera de máximo retorno", "el tope se subió a 40%", `"Class":"main"`, `"Class":"atcap"`,
		"Realty", `"SectorTitle":"Real Estate"`, "Retorno esperado de la cartera", "Sharpe (tasa libre de riesgo de 4%)", "Máx. Sharpe", `"TopN":2`,
		"+15,9%", "55%", "4/4", `{"Value":"sal","Label":"Salud: score por grupo"`, `"Href":"quant.html","Active":true`, "Mom. 12-1")
}

func TestLab(t *testing.T) {
	src, out := t.TempDir(), t.TempDir()
	in, err := LoadInputs(Paths{Report: writeFile(t, src, "report.json", labReport())})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(in, out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"src/data/lab-backtest.json", "src/data/lab-sweep.json", "src/data/lab-compare.json", "public/lab/default.json", "public/lab/backtest/200-2.json"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("falta %s: %v", f, err)
		}
	}

	bt := page(t, out, "lab-backtest")
	wantAll(t, "backtest", bt,
		`"DocTitle":"Momentum y Scores · Lab: Backtest"`, `"Label":"Volver a la app","Href":"index.html"`, `"Href":"lab-backtest.html","Active":true`,
		`"Href":"lab-sweep.html"`, `"Href":"lab-compare.html"`)
	if strings.Contains(bt, `"Label":"About"`) {
		t.Error("el Lab no debería tener el botón About")
	}

	wantAll(t, "lab/default.json", pub(t, out, "lab/default.json"), `"default":"200-2"`)
	datos := pub(t, out, "lab/backtest/200-2.json") + pub(t, out, "lab/sweep/200-2.json") + pub(t, out, "lab/compare/200-2.json")
	wantAll(t, "lab/<vista>/200-2.json", datos,
		`"Value":"200-2","Label":"mega-cap","Selected":true`, `"Label":"2 años (2024–2026)"`, `"Months":4,"Selected":true`, `"Cap0":"15.000,00"`, `"Cap1":"20.000,00"`,
		"data-chart=", "AAPL", "+20,0%", "-10,0%", `"CalCount":2`, "primer rebalanceo efectivo"[:0]+`"Late":true`, `"First":"2024-04-01"`,
		"Plazo más consistente"[:0]+`"HasBest":true`, `"SameWinner":true`, "Arranques probados: 4m: 2 · 6m: 2", `"Headline":"Empate: cada modelo gana en 1 de 2 plazos."`,
		`"HasHalves":true`, `"Split":"2025-01-01"`, "N-0 gana las dos mitades en 0 de 2 plazos, N-1 en 1", "N-1 en las dos", "4-0 (actual)", "6-1 (skip-month)")
}

func TestLabSinSkip(t *testing.T) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(labReport()), &raw); err != nil {
		t.Fatal(err)
	}
	raw["backtest_skip"] = json.RawMessage(`[]`)
	b, _ := json.Marshal(raw)
	src, out := t.TempDir(), t.TempDir()
	in, err := LoadInputs(Paths{Report: writeFile(t, src, "report.json", string(b))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(in, out); err != nil {
		t.Fatal(err)
	}
	wantAll(t, "compare", pub(t, out, "lab/compare/200-2.json"), `"Bar":`, `"Empty":true`)
}

func TestChartHTML(t *testing.T) {
	series := []chartSeries{
		{Name: "otro", Color: "var(--muted)", Width: 1, Opacity: 0.45, Points: []chartPoint{{"2024-01-01", 100}, {"2026-01-01", 120}}},
		{Name: "Base <x>", Color: "var(--accent)", Points: []chartPoint{{"2024-01-01", 100}, {"2025-01-01", 150}, {"2026-01-01", 200}}},
	}
	html := string(chartHTML(series, "Capital (US$)"))
	wantAll(t, "chart", html, `data-chart="`, "<path", `stroke-opacity="0.45"`, `stroke-width="2.5"`[:0]+`stroke-width="2"`, ">200<", ">2025<", "Base &lt;x&gt;", `class="legend"`, `style="stroke:var(--accent)"`)
	if strings.Contains(html, ">otro<") {
		t.Error("la serie de fondo no debería estar en la leyenda")
	}
	if strings.Contains(html, "Base <x>") {
		t.Error("el nombre de la serie no quedó escapado")
	}
	if got := chartHTML(nil, "x"); got != "" {
		t.Errorf("sin series esperaba vacío, obtuve %q", got)
	}
}

func TestRangeYAnios(t *testing.T) {
	if r := newRange("2020-01-01", "2024-01-01", 4, "2020-04-01"); !r.Late || r.YearsText != "(4 años)" {
		t.Errorf("rango tardío: %+v", r)
	}
	if r := newRange("2020-01-01", "2021-01-01", 1, "2020-02-01"); r.Late || r.YearsText != "(1 año)" {
		t.Errorf("rango normal: %+v", r)
	}
	if r := newRange("2020-01-01", "2021-01-01", 0, ""); r.Late || r.YearsText != "" {
		t.Errorf("sin años ni primer rebalanceo: %+v", r)
	}
	if got := labKey("0", 10); got != "0-10" {
		t.Errorf("labKey = %q", got)
	}
}
