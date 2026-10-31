// Package evaluator es el evaluador de empresas: un análisis fundamental (rentabilidad, crecimiento, caja, deuda, valoración,
// tendencia y comparación con competidores) de cada acción del S&P 500 y del IPSA, con un veredicto por categoría.
//
// Es una sección aparte: no usa los scores, el ranking ni las tablas de los otros modelos. De ellos toma solo las listas de
// tickers; los datos vienen de Yahoo Finance (gohoo) y el resultado es un JSON propio para el frontend.
package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"top3/internal/config"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/ipsa"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/yahoo"
)

// Los universos que se evalúan.
const (
	SP500 = "sp500"
	IPSA  = "ipsa"
)

// UniverseInfo describe un universo en el JSON.
type UniverseInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Currency string `json:"currency"`
	Count    int    `json:"count"`
}

// SectionInfo es una categoría en el JSON.
type SectionInfo struct {
	ID string `json:"id"`
	SectionDef
}

// Payload es el JSON del evaluador.
type Payload struct {
	GeneratedAt string               `json:"generated_at"`
	Universes   []UniverseInfo       `json:"universes"`
	Sections    []SectionInfo        `json:"sections"`
	Metrics     map[string]MetricDef `json:"metrics"`
	Glossary    map[string]string    `json:"glossary"`
	Companies   []Company            `json:"companies"`
	Missing     map[string][]string  `json:"missing,omitempty"` // tickers sin datos, por universo
}

// target es una empresa a evaluar: su ticker de Yahoo y el universo al que pertenece.
type target struct {
	ticker, universe string
}

// targets arma la lista de empresas: las del S&P 500 actual (de la base) y las del IPSA (de su archivo de universo), según [evaluator].
func targets(cfg config.Config, useCache bool) ([]target, error) {
	var out []target
	if cfg.Evaluator.Sp500 {
		d, err := db.Open(cfg.Data.DBPath)
		if err != nil {
			return nil, err
		}
		defer d.Close()
		c, err := market.NewConstituents(d, cfg.Tickers.Aliases)
		if err != nil {
			return nil, err
		}
		for _, t := range c.Current {
			out = append(out, target{t, SP500})
		}
	}
	if cfg.Evaluator.Ipsa {
		u, err := ipsa.EnsureUniverse(cfg, useCache, time.Now())
		if err != nil {
			return nil, err
		}
		for _, s := range u.Stocks {
			out = append(out, target{ipsa.YahooTicker(s, cfg.Ipsa), IPSA})
		}
	}
	return out, nil
}

// Run evalúa todas las empresas y escribe el JSON de [evaluator] output.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, useCache bool) (*Payload, error) {
	ts, err := targets(cfg, useCache)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, errs.EvaTooFew.Errorf("no hay empresas que evaluar (revisa [evaluator] sp500 e ipsa)")
	}
	logx.Infof("Evaluador: descargando datos fundamentales de %d empresas...", len(ts))
	res := yahoo.ParallelMap(ctx, ts, cfg.Data.SharesWorkers, func(ctx context.Context, t target) (Raw, error) {
		return Fetch(ctx, y, t.ticker)
	})
	var cs []Company
	missing := map[string][]string{}
	for i, r := range res {
		if r.Err != nil {
			logx.Debugf("Sin datos fundamentales para %s: %v", ts[i].ticker, r.Err)
			missing[ts[i].universe] = append(missing[ts[i].universe], ts[i].ticker)
			continue
		}
		cs = append(cs, Analyze(r.Value, ts[i].universe))
	}
	n := 0
	for _, m := range missing {
		sort.Strings(m)
		n += len(m)
	}
	if n > 0 {
		logx.Warnf(errs.EvaNoData.Msg("Sin datos fundamentales para %d de %d empresas: %v"), n, len(ts), missing)
	}
	if float64(len(cs)) < cfg.Evaluator.MinCoverage*float64(len(ts)) {
		return nil, errs.EvaTooFew.Errorf("solo %d de %d empresas tienen datos (mínimo %.0f%%)", len(cs), len(ts), cfg.Evaluator.MinCoverage*100)
	}
	p := Build(cs, cfg.Evaluator.Peers)
	p.Missing = missing
	if err := Write(cfg.Evaluator.Output, p); err != nil {
		return nil, err
	}
	logx.Infof("Evaluador: %d empresas guardadas en %s", len(p.Companies), cfg.Evaluator.Output)
	return p, nil
}

// Build agrega los competidores a las empresas y arma el payload: ordenadas por universo y capitalización.
func Build(cs []Company, peers int) *Payload {
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].Universe != cs[j].Universe {
			return cs[i].Universe < cs[j].Universe
		}
		a, b := 0.0, 0.0
		if cs[i].MarketCap != nil {
			a = *cs[i].MarketCap
		}
		if cs[j].MarketCap != nil {
			b = *cs[j].MarketCap
		}
		return a > b
	})
	AttachPeers(cs, peers)
	count := map[string]int{}
	for _, c := range cs {
		count[c.Universe]++
	}
	p := &Payload{GeneratedAt: time.Now().Format("2006-01-02T15:04:05"), Metrics: Metrics, Glossary: Glossary, Companies: cs}
	for _, u := range []UniverseInfo{{SP500, "S&P 500", "USD", 0}, {IPSA, "IPSA (Chile)", "CLP", 0}} {
		if u.Count = count[u.ID]; u.Count > 0 {
			p.Universes = append(p.Universes, u)
		}
	}
	for _, s := range Sections {
		p.Sections = append(p.Sections, SectionInfo{s.ID, s.SectionDef})
	}
	return p
}

// Write guarda el payload como JSON (sin escapar HTML) creando la carpeta si hace falta.
func Write(path string, p *Payload) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimRight(buf.Bytes(), "\n"), 0o644)
}

// Summary es el texto de la consola con el recuento de veredictos por universo.
func (p *Payload) Summary() []string {
	type tally struct{ good, warn, bad, na int }
	by := map[string]*tally{}
	for _, c := range p.Companies {
		t := by[c.Universe]
		if t == nil {
			t = &tally{}
			by[c.Universe] = t
		}
		switch c.Verdict {
		case Good:
			t.good++
		case Warn:
			t.warn++
		case Bad:
			t.bad++
		default:
			t.na++
		}
	}
	var lines []string
	for _, u := range p.Universes {
		t := by[u.ID]
		lines = append(lines, fmt.Sprintf("%s: %d empresas (bien %d, atención %d, alerta %d, sin datos %d)", u.Label, u.Count, t.good, t.warn, t.bad, t.na))
	}
	return lines
}
