// Pruebas de las páginas del sitio: Visor, Home, ETFs, evaluador (listado y empresa) y About, a partir de JSON de ejemplo,
// más el formato es-AR y la cáscara común.
//
// Fecha: 2026-11-19

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"topwx/internal/evaluator"
)

func ptr[T any](v T) *T { return &v }

const reportJSON = `{
 "generated_at":"2026-11-18T20:30:04","run_date":"2026-11-18",
 "settings":{"top_n":4,"months":4,"initial_capital":15000,"commission":0.0005},
 "actual":[
  {"min_cap_billions":null,"label":"índice completo","months":4,"as_of":"2026-11-18","previous_as_of":null,"changes":null,"anchor":"2026-10-01","next_rebalance":"2027-02-01",
   "picks":[{"rank":1,"ticker":"AAPL","return":0.25,"price":200.5,"market_cap":3e12}],
   "alternates":[{"rank":2,"ticker":"MSFT","return":-0.1,"price":400,"market_cap":null}],
   "tracking":[{"as_of":"2026-10-01","until":"2026-11-18","days":48,"closed":false,"picks":[{"ticker":"AAPL","price":1,"now":2,"return":0.05}],"return":0.05}]},
  {"min_cap_billions":200,"label":"mega-cap","months":4,"as_of":"2026-11-18","previous_as_of":"2026-10-01","changes":{"entered":["NVDA"],"exited":[],"kept":["AAPL"]},
   "picks":[],"alternates":[],"tracking":[]}
 ],
 "etfq":{"as_of":"2026-11-18","horizons":[3,12],"risk_free":"VUSB",
  "settings":{"max_ter":0.5,"min_aum_millions":100,"quality_weight":0.25,"ter_weight":4,"aum_weight":1},
  "issuers":[{"name":"iShares","funds":2,"ranked":2,"scored":1}],"not_ranked":3,
  "rows":[
   {"rank":1,"ticker":"IVV","name":"iShares <Core>","issuer":"iShares","category":"Large Blend","group_size":5,"returns":{"3":0.05,"12":0.2},"ter":0.03,"aum":5e11,"quality":80,"momentum":90,"final":87,"excluded":[]},
   {"rank":null,"ticker":"XYZ","name":"Xyz","issuer":"iShares","category":"Árbol","group_size":1,"returns":{"3":-0.02},"ter":0.9,"aum":5e7,"excluded":["ter","aum"]}
  ]}
}`

const evaluatorJSON = `{
 "generated_at":"2026-11-18T20:29:00",
 "universes":[{"id":"sp500","label":"S&P 500","currency":"USD","count":2}],
 "sections":[
  {"id":"prof","title":"Rentabilidad","why":"Porque sí","labels":["Bien","Atención","Alerta"]},
  {"id":"peers","title":"Vs. competidores","intro":"Intro","why":"w","labels":["Mejor","Similar","Peor"]}
 ],
 "metrics":{"roe":{"name":"ROE","unit":"pct","why":"Rentabilidad del capital"}},
 "companies":[
  {"ticker":"BRK-B","name":"Berkshire <Hathaway>","universe":"sp500","sector":"Financial","currency":"USD","price":450.5,"market_cap":1e12,"verdict":"good",
   "categories":[
    {"id":"prof","verdict":"good","years":["2021","2022","2023","2024"],"metrics":[{"id":"roe","v":18.2,"l":"good","s":[10,12.5,null,18.2],"lo":0,"hi":30,"n":"Nota"}]},
    {"id":"peers","verdict":"na"}
   ]},
  {"ticker":"ZZZ","name":"Zzz","universe":"sp500","price":null,"market_cap":null,"verdict":"na","categories":[]}
 ]
}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantAll(t *testing.T, page, html string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("%s no contiene %q", page, w)
		}
	}
}

func buildAll(t *testing.T) string {
	t.Helper()
	src, out := t.TempDir(), t.TempDir()
	v := sample()
	in, err := LoadInputs(Paths{
		Visor:     writeVisor(t, src, v),
		Report:    writeFile(t, src, "report.json", reportJSON),
		Evaluator: writeFile(t, src, "evaluator.json", evaluatorJSON),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(in, out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBuildPaginas(t *testing.T) {
	out := buildAll(t)
	for _, f := range []string{"index.html", "visor.html", "etfq.html", "evaluador.html", "empresa.html", "empresas.js", "about.html", "site.css", "site.js", "table.js", "visor.js", "alpine.min.js", "empresa.js"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("falta %s: %v", f, err)
		}
	}

	wantAll(t, "visor.html", read(t, out, "visor.html"),
		"https://stockanalysis.com/stocks/brk.b/", "https://stockanalysis.com/etf/voo/", `Acciones (<span class="n">2</span>)`, `class="pos r"`, "5122", "12.345",
		`<script src="visor.js" defer>`, "<title>Momentum y Scores · Visor de acciones y ETFs</title>", `<a class="tab active" href="visor.html" aria-current="page">`,
		"Corrida del 2026-11-18.", "comisión de 0,05% por lado")
	visorHTML := read(t, out, "visor.html")
	if strings.Contains(visorHTML, "<Hathaway>") || strings.Contains(visorHTML, "Berkshire <Hathaway>") {
		t.Error("el nombre con < > no quedó escapado en el Visor")
	}

	wantAll(t, "index.html", read(t, out, "index.html"),
		"Top 4 del S&amp;P 500", `<option value="0">índice completo</option>`, `<option value="200" selected>mega-cap</option>`, "&#43;25,0%", "3.000", "200,50", "100,0%",
		"Primera corrida guardada", "Cambios vs. 2026-10-01:", `<span class="pos">NVDA</span>`, "Suplentes", "-10,0%", "Seguimiento de selecciones anteriores", "en curso",
		"Ventana de retorno", `<a class="tab active" href="index.html" aria-current="page">`)

	etfq := read(t, out, "etfq.html")
	wantAll(t, "etfq.html", etfq,
		"iShares &lt;Core&gt;", `data-f-scored="0"`, `data-f-scored="1"`, `data-v="87"`, "TER &gt; 0,50% y AUM &lt; 100 M", "Ret. 3m", "Ret. 12m", "&#43;5,0%", "-2,0%", "n/d",
		"<option>Árbol</option>", "500,0 B", `aria-sort="ascending"`, "Score final = 75% momentum &#43; 25% calidad", "Referencia sin riesgo</strong> VUSB", "3 ETFs más quedaron fuera")
	if strings.Contains(etfq, "iShares <Core>") {
		t.Error("el nombre del ETF no quedó escapado")
	}

	wantAll(t, "evaluador.html", read(t, out, "evaluador.html"),
		`data-href="empresa.html#BRK-B"`, `data-q="brk-b berkshire &lt;hathaway&gt;"`, `<span class="dotv good"`, `<span class="pill good">Bien</span>`, "1,0 B"[:0]+"1.000 B USD",
		"Pares", "2 de S&amp;P 500"[:0]+"2 de S&amp;P 500", `<option value="good">Bien</option>`)

	pagina := read(t, out, "empresa.html")
	wantAll(t, "empresa.html", pagina, `href="site.css"`, `href="evaluador.html"`, `src="empresas.js"`, `src="alpine.min.js"`, `x-data="empresa"`)

	datos := read(t, out, "empresas.js")
	wantAll(t, "empresas.js", datos,
		"window.EMPRESAS=", `"BRK-B":{`, `"ZZZ":{`, `\u003csvg viewBox`, "18,2 %", "peers-none", `Berkshire \u003cHathaway\u003e`,
		`"Left":"18.2"`, "Nota", "Precio 450,50 USD", "Capitalización 1.000 B USD", "Sin sector", "Precio n/d")
	if strings.Contains(datos, "Berkshire <Hathaway>") {
		t.Error("el nombre de la empresa no quedó escapado en empresas.js")
	}

	wantAll(t, "about.html", read(t, out, "about.html"), "capital inicial: US$ 15.000", "Comisión de 0,05% por lado", "Límites a tener presentes", `<a class="btn active" href="about.html">About</a>`)
}

func TestBuildSinJSON(t *testing.T) {
	out := t.TempDir()
	stale := filepath.Join(out, "evaluador")
	os.MkdirAll(stale, 0o755)
	writeFile(t, stale, "VIEJA.html", "x")
	if _, err := Build(Inputs{}, out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"index.html", "visor.html", "etfq.html", "evaluador.html", "about.html"} {
		wantAll(t, f, read(t, out, f), "Ejecute topwx", `class="best error"`)
	}
	if _, err := os.Stat(filepath.Join(stale, "VIEJA.html")); err == nil {
		t.Error("quedó una página vieja de empresa en evaluador/")
	}
}

func TestLoadInputs(t *testing.T) {
	dir := t.TempDir()
	in, err := LoadInputs(Paths{Visor: filepath.Join(dir, "a.json"), Report: filepath.Join(dir, "b.json"), Evaluator: filepath.Join(dir, "c.json")})
	if err != nil || in.Visor != nil || in.Report != nil || in.Etfq != nil || in.Evaluator != nil {
		t.Errorf("sin archivos: %+v, %v", in, err)
	}
	in, err = LoadInputs(Paths{Report: writeFile(t, dir, "report.json", reportJSON)})
	if err != nil || in.Report == nil || in.Etfq == nil || len(in.Etfq.Rows) != 2 {
		t.Errorf("con reporte: %+v, %v", in, err)
	}
	if _, err := LoadInputs(Paths{Visor: writeFile(t, dir, "roto.json", "no es json")}); err == nil {
		t.Error("esperaba un error con un JSON roto")
	}
}

func TestVisorFilaIncompleta(t *testing.T) {
	p := sample()
	p.Sections[0].Rows[0] = []any{"AAPL"}
	if _, err := Build(Inputs{Visor: &p}, t.TempDir()); err == nil {
		t.Error("esperaba un error por una fila con menos celdas que columnas")
	}
}

func TestFormatosEs(t *testing.T) {
	str := map[string]string{
		esNum(1234.567, 2):                         "1.234,57",
		esNum(1234, 0):                             "1.234",
		esNum(999, 0):                              "999",
		esNum(-1234.5, 1):                          "-1.234,5",
		pctFrac(ptr(0.123), 1):                     "+12,3%",
		pctFrac(ptr(-0.05), 1):                     "-5,0%",
		pctFrac(nil, 1):                            "n/d",
		share(0.25, 0):                             "25%",
		aum(ptr(1.5e9)):                            "1,5 B",
		aum(ptr(4.5e8)):                            "450 M",
		aum(nil):                                   "n/d",
		billions(ptr(3e12)):                        "3.000",
		money(ptr(5e11), "USD"):                    "500 B USD",
		money(ptr(1.234e9), ""):                    "1,2 B",
		money(ptr(-3e6), ""):                       "-3,0 M",
		money(ptr(12345.0), ""):                    "12.345",
		price(ptr(450.5), "USD"):                   "450,50 USD",
		price(nil, ""):                             "n/d",
		signClass(ptr(1.0)) + signClass(ptr(-1.0)): "posneg",
	}
	for got, want := range str {
		if got != want {
			t.Errorf("got %q, quiero %q", got, want)
		}
	}
}

func TestFormatMetric(t *testing.T) {
	cases := []struct {
		m    evaluator.Metric
		def  evaluator.MetricDef
		want string
	}{
		{evaluator.Metric{ID: "roe", Value: ptr(18.24)}, evaluator.MetricDef{Unit: "pct"}, "18,2 %"},
		{evaluator.Metric{ID: "target_upside", Value: ptr(5.0)}, evaluator.MetricDef{Unit: "pct"}, "+5,0 %"},
		{evaluator.Metric{ID: "pe", Value: ptr(2.5)}, evaluator.MetricDef{Unit: "x"}, "2,50×"},
		{evaluator.Metric{ID: "pb", Value: ptr(3.14)}, evaluator.MetricDef{Unit: "years"}, "3,1 años"},
		{evaluator.Metric{ID: "cash", Value: ptr(-2e9), Label: "Caja neta"}, evaluator.MetricDef{Unit: "money"}, "2,0 B USD"},
		{evaluator.Metric{ID: "x", Text: "texto propio"}, evaluator.MetricDef{Unit: "pct"}, "texto propio"},
		{evaluator.Metric{ID: "x"}, evaluator.MetricDef{Unit: "pct"}, "n/d"},
		{evaluator.Metric{ID: "x", Value: ptr(1.0)}, evaluator.MetricDef{}, "1,00"},
	}
	for _, c := range cases {
		if got := formatMetric(c.m, c.def, "USD"); got != c.want {
			t.Errorf("formatMetric(%+v) = %q, quiero %q", c.m, got, c.want)
		}
	}
}

func TestPillText(t *testing.T) {
	labels := [3]string{"Mejor", "Similar", "Peor"}
	for _, c := range []struct {
		level evaluator.Level
		own   string
		want  string
	}{
		{evaluator.Good, "", "Mejor"}, {evaluator.Bad, "", "Peor"}, {evaluator.Info, "", "Informativo"}, {evaluator.NA, "", "Sin datos"}, {evaluator.Warn, "Caja neta", "Caja neta"},
	} {
		if got := pillText(c.level, c.own, labels); got != c.want {
			t.Errorf("pillText(%s, %q) = %q, quiero %q", c.level, c.own, got, c.want)
		}
	}
}

func TestSparkSVG(t *testing.T) {
	svg := sparkSVG([]*float64{ptr(-1.0), nil, ptr(2.0)}, []string{"2022", "2023", "2024"}, "ROE")
	wantAll(t, "svg", svg, "<polyline", "<circle", `class="zero"`, "<title>2024: 2,0</title>", "<span>2023</span>")
	if got := sparkSVG([]*float64{ptr(1.0)}, []string{"2024"}, "x"); got != "" {
		t.Errorf("con un solo punto esperaba vacío, obtuve %q", got)
	}
}

func TestOrdenYArchivos(t *testing.T) {
	if collateEs("Árbol", "Zebra") >= 0 || collateEs("ñu", "nz") <= 0 || collateEs("a", "B") >= 0 {
		t.Error("collateEs no ordena como localeCompare('es')")
	}
	for in, want := range map[string]string{"BRK-B": "BRK-B", "a/b c": "A_B_C", "BF.B": "BF.B"} {
		if got := companyKey(in); got != want {
			t.Errorf("companyKey(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestShell(t *testing.T) {
	sh := newShell("visor", Meta{TopN: 4}, "2026-11-18 20:29:17", "../", nil)
	if len(sh.Groups) != 5 || sh.Root != "../" {
		t.Errorf("grupos = %d, raíz = %q", len(sh.Groups), sh.Root)
	}
	if sh.Home.Active || sh.Home.Href != "index.html" || sh.About.Href != "about.html" {
		t.Errorf("Home = %+v, About = %+v", sh.Home, sh.About)
	}
	if !strings.Contains(sh.Footer, "Generado el 2026-11-18 20:29:17.") || !strings.Contains(sh.Footer, "Visor: foto de hoy") {
		t.Errorf("pie inesperado: %s", sh.Footer)
	}
	if got := newShell("actual", Meta{TopN: 4}, "", "", nil); got.Title != "Top 4 del S&P 500" || !got.Home.Active {
		t.Errorf("Home con reporte: %+v", got)
	}
	if got := newShell("actual", Meta{}, "", "", nil).Title; got != "Top N del S&P 500" {
		t.Errorf("título de Home sin reporte = %q", got)
	}
	if got := newShell("about", Meta{RunDate: "2026-11-18"}, "", "", nil); !got.About.Active || strings.Contains(got.Sub, "Corrida") {
		t.Errorf("About: %+v", got)
	}
	if got := newShell("etfq", Meta{}, "", "", nil).Groups[1].Links[0].Href; got != "etfq.html" {
		t.Errorf("enlace de ETFs = %q", got)
	}
	if got := newShell("quant", Meta{}, "", "", nil).Groups[2].Links[0].Href; got != "quant.html" {
		t.Errorf("enlace de Quant = %q", got)
	}
}
