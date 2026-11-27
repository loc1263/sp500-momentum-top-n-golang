// Arma los datos del sitio: lee los JSON que dejó el backend y escribe, para el proyecto Astro de frontend/, el JSON ya formateado de
// cada página (src/data) y los archivos de datos que lee el navegador (public). Astro es quien renderiza el HTML.
// Si falta un JSON, esa página muestra qué comando correr en lugar de romper el resto del sitio.
//
// Fecha: 2026-11-27

package site

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	data     string // src/data del proyecto Astro: un JSON por página
	public   string // public/ del proyecto Astro: archivos que carga el navegador (empresas/, lab/, evaluador.xlsx)
	meta     Meta
	fallback string
}

// Build escribe los datos de todas las páginas en el proyecto Astro de siteDir y devuelve la carpeta de los JSON de página.
func Build(in Inputs, siteDir string) (string, error) {
	b := &builder{data: filepath.Join(siteDir, "src", "data"), public: filepath.Join(siteDir, "public"), meta: metaOf(in.Report)}
	if in.Visor != nil {
		b.fallback = strings.Replace(in.Visor.GeneratedAt, "T", " ", 1)
	}
	for _, dir := range []string{b.data, b.public} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
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
	return b.data, nil
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
	book, err := evaluatorXLSX(*p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.public, xlsxFile), book, 0o644); err != nil {
		return err
	}
	return b.companies(p)
}

func (b *builder) about(r *report.Report) error {
	if r == nil {
		return b.missing("about.html", "about", "No se pudo cargar el reporte. Ejecute topwx export (o el pipeline) en backend/ y vuelva a generar el sitio.")
	}
	return b.write("about.html", "about", "about", newAboutPage(b.meta))
}

func (b *builder) missing(file, tabID, msg string) error { return b.write(file, "missing", tabID, msg) }

// write arma los datos de file (el nombre de la página, relativo a la raíz del sitio) con la cáscara; name es el componente de Astro
// que pinta el contenido y tabID elige la pestaña activa.
func (b *builder) write(file, name, tabID string, body any) error {
	return b.writeShell(file, name, newShell(tabID, b.meta, b.fallback, body))
}

// pageFile es el JSON de una página: qué contenido pintar (Page) y la cáscara con ese contenido en Body.
type pageFile struct {
	Page string
	shell
}

// writeShell escribe los datos de file con una cáscara ya armada (la del Lab tiene pestañas propias).
func (b *builder) writeShell(file, name string, sh shell) error {
	return writeJSON(filepath.Join(b.data, strings.TrimSuffix(file, ".html")+".json"), pageFile{Page: name, shell: sh})
}

// writeJSON escribe v sin escapar <, > ni &: el HTML de las celdas va tal cual y Astro lo inserta con set:html.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writePublic escribe public/<rel> como JSON (sin escapar <, > ni &). Son los archivos que el navegador pide por fetch cuando hacen
// falta, uno por empresa y uno por vista, universo y período del Lab, en lugar de bajar todo junto.
func (b *builder) writePublic(rel string, v any) error {
	path := filepath.Join(b.public, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSON(path, v)
}

// resetPublic borra lo que una corrida anterior dejó en public/<dir>, y el .js de datos de las versiones que lo bajaban todo junto.
func (b *builder) resetPublic(dir, oldScript string) error {
	if err := os.RemoveAll(filepath.Join(b.public, dir)); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(b.public, oldScript)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (b *builder) quant(q *quantData) error {
	if q == nil {
		return b.missing("quant.html", "quant", "Sin modelo por score. Ejecute topwx quant y topwx export (en backend/) y vuelva a generar el sitio.")
	}
	return b.write("quant.html", "quant", "quant", newQuantPage(*q))
}

// labData es lo que lee lab.js: la página ya armada de cada vista por universo y período (clave labKey) y la clave por defecto.
type labData struct {
	Default string                    `json:"default"`
	Pages   map[string]map[string]any `json:"pages"`
}

// lab genera las tres páginas del Lab (backtest, plazos y N-0 vs N-1) y public/lab/<vista>/<universo>-<años>.json con los datos de cada universo y período.
// El Lab no figura en las pestañas: se llega por la dirección. Sin backtest, las tres páginas dicen qué comando correr.
func (b *builder) lab(r *report.Report) error {
	if err := b.resetPublic("lab", "lab-datos.js"); err != nil {
		return err
	}
	missing := func() error {
		for _, v := range labViews {
			sh := newLabShell(v.ID, b.meta, b.fallback, "Sin datos del Lab. Ejecute topwx backtest (en backend/) y vuelva a generar el sitio.")
			if err := b.writeShell(labPage(v.ID), "missing", sh); err != nil {
				return err
			}
		}
		return nil
	}
	if r == nil || len(r.Backtest) == 0 {
		return missing()
	}
	pages := map[string]func(*report.Report, labCtx) any{
		"backtest": func(r *report.Report, c labCtx) any { return newBacktestPage(r, c) },
		"sweep":    func(r *report.Report, c labCtx) any { return newSweepPage(r, c) },
		"compare":  func(r *report.Report, c labCtx) any { return newComparePage(r, c) },
	}
	data := labData{Pages: map[string]map[string]any{}}
	for _, v := range labViews {
		data.Pages[v.ID] = map[string]any{}
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
				data.Pages[v.ID][labKey(key, p.Years)] = pages[v.ID](r, ctx)
			}
		}
	}
	best := r.Backtest[0]
	for _, u := range r.Backtest {
		if capOf(u.MinCapBillions) > capOf(best.MinCapBillions) {
			best = u
		}
	}
	if len(best.Periods) == 0 {
		return missing()
	}
	data.Default = labKey(capKeyOf(best.MinCapBillions), defaultYears(best, r.Settings.BacktestYears, r.Settings))
	for _, v := range labViews {
		if err := b.writeShell(labPage(v.ID), "lab_"+v.ID, newLabShell(v.ID, b.meta, b.fallback, nil)); err != nil {
			return err
		}
	}
	for view, byKey := range data.Pages {
		for key, page := range byKey {
			if err := b.writePublic("lab/"+view+"/"+key+".json", page); err != nil {
				return err
			}
		}
	}
	return b.writePublic("lab/default.json", map[string]string{"default": data.Default})
}

// companies escribe empresa, la única página de detalle, y public/empresas/<TICKER>.json con la ficha ya formateada de cada empresa.
func (b *builder) companies(p *evaluator.Payload) error {
	if err := b.write("empresa.html", "empresa", "evaluador", nil); err != nil {
		return err
	}
	if err := b.resetPublic("empresas", "empresas.js"); err != nil {
		return err
	}
	for _, c := range p.Companies {
		if err := b.writePublic("empresas/"+companyKey(c.Ticker)+".json", newCompanyPage(c, *p)); err != nil {
			return err
		}
	}
	return nil
}
