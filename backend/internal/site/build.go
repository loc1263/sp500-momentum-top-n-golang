// Arma el sitio completo: lee los JSON que dejó el backend, genera cada página con la cáscara común y copia los archivos estáticos.
// Si falta un JSON, esa página muestra qué comando correr en lugar de romper el resto del sitio.
//
// Fecha: 2026-11-19

package site

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"topwx/internal/etfq"
	"topwx/internal/evaluator"
	"topwx/internal/report"
	"topwx/internal/visor"
)

// Inputs son los datos de las páginas; un puntero nulo significa que ese JSON no existe todavía.
type Inputs struct {
	Visor     *visor.Payload
	Report    *report.Report
	Etfq      *etfq.Payload
	Evaluator *evaluator.Payload
	Quant     *quantData
}

// Paths son las rutas de los JSON que lee LoadInputs.
type Paths struct{ Visor, Report, Evaluator string }

var staticFiles = []string{"site.css", "site.js", "table.js", "chart.js", "visor.js"}

func readJSON[T any](path string) (*T, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("%s no es un JSON válido: %w", path, err)
	}
	return &v, nil
}

// LoadInputs lee los JSON; los que no existen quedan nulos y los que están rotos devuelven error.
func LoadInputs(p Paths) (Inputs, error) {
	var in Inputs
	var err error
	if in.Visor, err = readJSON[visor.Payload](p.Visor); err != nil {
		return in, err
	}
	if in.Report, err = readJSON[report.Report](p.Report); err != nil {
		return in, err
	}
	if in.Evaluator, err = readJSON[evaluator.Payload](p.Evaluator); err != nil {
		return in, err
	}
	if in.Report != nil && len(in.Report.Etfq) > 0 && string(in.Report.Etfq) != "null" {
		var e etfq.Payload
		if err := json.Unmarshal(in.Report.Etfq, &e); err != nil {
			return in, fmt.Errorf("el bloque etfq de %s no es válido: %w", p.Report, err)
		}
		in.Etfq = &e
	}
	if in.Report != nil && len(in.Report.Quant) > 0 && string(in.Report.Quant) != "null" {
		var q quantData
		if err := json.Unmarshal(in.Report.Quant, &q); err != nil {
			return in, fmt.Errorf("el bloque quant de %s no es válido: %w", p.Report, err)
		}
		in.Quant = &q
	}
	return in, nil
}

type builder struct {
	out      string
	meta     Meta
	fallback string
	cache    map[string]*template.Template
}

// Build escribe todas las páginas y los archivos estáticos en outDir y devuelve la ruta del index.
func Build(in Inputs, outDir string) (string, error) {
	b := &builder{out: outDir, meta: metaOf(in.Report), cache: map[string]*template.Template{}}
	if in.Visor != nil {
		b.fallback = strings.Replace(in.Visor.GeneratedAt, "T", " ", 1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	if err := os.RemoveAll(filepath.Join(outDir, "evaluador")); err != nil {
		return "", err
	}
	steps := []func() error{
		func() error { return b.visor(in.Visor) },
		func() error { return b.actual(in.Report) },
		func() error { return b.etfq(in.Etfq) },
		func() error { return b.evaluator(in.Evaluator) },
		func() error { return b.quant(in.Quant) },
		func() error { return b.lab(in.Report) },
		func() error { return b.about(in.Report) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return "", err
		}
	}
	for _, name := range staticFiles {
		data, err := assets.ReadFile("static/" + name)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(outDir, name), data, 0o644); err != nil {
			return "", err
		}
	}
	return filepath.Join(outDir, "index.html"), nil
}

func (b *builder) visor(p *visor.Payload) error {
	if p == nil {
		return b.missing("visor.html", "visor", "No se pudo cargar el JSON del Visor. Ejecute topwx visor (o el pipeline) en backend/ y vuelva a generar el sitio.")
	}
	body, err := newVisorPage(*p)
	if err != nil {
		return err
	}
	return b.write("visor.html", "visor", "visor", body)
}

func (b *builder) actual(r *report.Report) error {
	if r == nil || len(r.Actual) == 0 {
		return b.missing("index.html", "actual", "Sin Top actual. Ejecute topwx actual y topwx export (en backend/) y vuelva a generar el sitio.")
	}
	return b.write("index.html", "actual", "actual", newActualPage(r.Actual))
}

func (b *builder) etfq(p *etfq.Payload) error {
	if p == nil {
		return b.missing("etfq.html", "etfq", "Sin ETFs por gestora. Ejecute topwx etfq y topwx export (en backend/) y vuelva a generar el sitio.")
	}
	return b.write("etfq.html", "etfq", "etfq", newEtfqPage(*p))
}

func (b *builder) evaluator(p *evaluator.Payload) error {
	if p == nil {
		return b.missing("evaluador.html", "evaluador", "No se pudo cargar el JSON del evaluador. Ejecute topwx evaluator (o el pipeline) en backend/ y vuelva a generar el sitio.")
	}
	if err := b.write("evaluador.html", "evaluador", "evaluador", newEvaluatorList(*p)); err != nil {
		return err
	}
	for _, c := range p.Companies {
		if err := b.write("evaluador/"+companyFile(c.Ticker), "empresa", "evaluador", newCompanyPage(c, *p)); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) about(r *report.Report) error {
	if r == nil {
		return b.missing("about.html", "about", "No se pudo cargar el reporte. Ejecute topwx export (o el pipeline) en backend/ y vuelva a generar el sitio.")
	}
	return b.write("about.html", "about", "about", newAboutPage(b.meta))
}

func (b *builder) missing(file, tabID, msg string) error { return b.write(file, "missing", tabID, msg) }

// write arma file (relativo a la salida) con la cáscara y el contenido de templates/<name>.html; tabID elige la pestaña activa.
func (b *builder) write(file, name, tabID string, body any) error {
	root := strings.Repeat("../", strings.Count(file, "/"))
	return b.writeShell(file, name, newShell(tabID, b.meta, b.fallback, root, body))
}

// writeShell escribe file con una cáscara ya armada (la del Lab tiene pestañas propias).
func (b *builder) writeShell(file, name string, sh shell) error {
	t, ok := b.cache[name]
	if !ok {
		var err error
		t, err = template.ParseFS(assets, "templates/layout.html", "templates/partials.html", "templates/"+name+".html")
		if err != nil {
			return err
		}
		b.cache[name] = t
	}
	target := filepath.Join(b.out, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	if err := t.ExecuteTemplate(f, "layout.html", sh); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (b *builder) quant(q *quantData) error {
	if q == nil {
		return b.missing("quant.html", "quant", "Sin modelo por score. Ejecute topwx quant y topwx export (en backend/) y vuelva a generar el sitio.")
	}
	return b.write("quant.html", "quant", "quant", newQuantPage(*q))
}

// lab genera las páginas del Lab (backtest, plazos y N-0 vs N-1) por universo y período, más lab.html, que lleva al backtest
// por defecto. El Lab no figura en las pestañas: se llega por la dirección.
func (b *builder) lab(r *report.Report) error {
	if r == nil || len(r.Backtest) == 0 {
		return nil
	}
	if err := os.RemoveAll(filepath.Join(b.out, "lab")); err != nil {
		return err
	}
	pages := map[string]func(*report.Report, labCtx) any{
		"backtest": func(r *report.Report, c labCtx) any { return newBacktestPage(r, c) },
		"sweep":    func(r *report.Report, c labCtx) any { return newSweepPage(r, c) },
		"compare":  func(r *report.Report, c labCtx) any { return newComparePage(r, c) },
	}
	for _, u := range r.Backtest {
		if len(u.Periods) == 0 {
			continue
		}
		key := capKeyOf(u.MinCapBillions)
		for _, p := range u.Periods {
			if len(p.Terms) == 0 {
				continue
			}
			ctx := labCtx{Key: key, Universe: u, Period: p, Settings: r.Settings}
			for _, bu := range r.BacktestSkip {
				if capKeyOf(bu.MinCapBillions) != key {
					continue
				}
				for i := range bu.Periods {
					if bu.Periods[i].Years == p.Years {
						ctx.Skip = &bu.Periods[i]
					}
				}
			}
			for _, v := range labViews {
				sh := newLabShell(v.ID, ctx, b.meta, b.fallback, pages[v.ID](r, ctx))
				if err := b.writeShell(labFile(v.ID, key, p.Years), "lab_"+v.ID, sh); err != nil {
					return err
				}
			}
		}
	}
	best := r.Backtest[0]
	for _, u := range r.Backtest {
		if capValue2(u) > capValue2(best) {
			best = u
		}
	}
	if len(best.Periods) == 0 {
		return nil
	}
	target := labFile("backtest", capKeyOf(best.MinCapBillions), defaultYears(best, r.Settings.BacktestYears, r.Settings))
	redirect := `<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0; url=` + target + `"><a href="` + target + `">Lab</a>`
	return os.WriteFile(filepath.Join(b.out, "lab.html"), []byte(redirect), 0o644)
}

func capValue2(u report.Universe) float64 {
	if u.MinCapBillions == nil {
		return 0
	}
	return *u.MinCapBillions
}
