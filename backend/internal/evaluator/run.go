// Package evaluator es el evaluador de empresas: un análisis fundamental (rentabilidad, crecimiento, caja, deuda, valoración,
// tendencia y comparación con competidores) de cada acción del S&P 500, con un veredicto por categoría.
//
// Es una sección aparte: no usa los scores, el ranking ni las tablas de los otros modelos. De ellos toma solo las listas de
// tickers; los datos vienen de Yahoo Finance (gohoo) y el resultado es un JSON propio para el sitio.
//
// Fecha: 2026-11-19
package evaluator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/company"
	"topwx/internal/config"
	"topwx/internal/db"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/market"
	"topwx/internal/yahoo"
)

const SP500 = "sp500"

type UniverseInfo struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Currency string `json:"currency"`
	Count    int    `json:"count"`
}

type SectionInfo struct {
	ID string `json:"id"`
	SectionDef
}

type Payload struct {
	GeneratedAt string               `json:"generated_at"`
	Universes   []UniverseInfo       `json:"universes"`
	Sections    []SectionInfo        `json:"sections"`
	Metrics     map[string]MetricDef `json:"metrics"`
	Glossary    map[string]string    `json:"glossary"`
	Companies   []Company            `json:"companies"`
	Missing     map[string][]string  `json:"missing,omitempty"`
}

type target struct {
	ticker, universe string
}

func targets(d *sql.DB, cfg config.Config) ([]target, error) {
	var out []target
	if cfg.Evaluator.Sp500 {
		c, err := market.NewConstituents(d, cfg.Tickers.Aliases)
		if err != nil {
			return nil, err
		}
		for _, t := range c.Current {
			out = append(out, target{t, SP500})
		}
	}
	return out, nil
}

func emptyUniverses(ts []target, cfg config.Config) []string {
	have := map[string]bool{}
	for _, t := range ts {
		have[t.universe] = true
	}
	var out []string
	if cfg.Evaluator.Sp500 && !have[SP500] {
		out = append(out, "S&P 500")
	}
	return out
}

func Run(ctx context.Context, cfg config.Config, y *yahoo.Client) (*Payload, error) {
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	ts, err := targets(d, cfg)
	if err != nil {
		return nil, err
	}
	for _, u := range emptyUniverses(ts, cfg) {
		logx.Warnf(errs.EvaNoList.Msg("No hay empresas de %s en la lista; se evalúan solo las demás"), u)
	}
	if len(ts) == 0 {
		return nil, errs.EvaTooFew.Errorf("no hay empresas que evaluar (revisa [evaluator] sp500)")
	}
	tickers := make([]string, len(ts))
	for i, t := range ts {
		tickers[i] = t.ticker
	}
	logx.Infof("Evaluador: datos fundamentales de %d empresas...", len(ts))
	snaps, err := company.Ensure(ctx, d, y, tickers, true, cfg.Quant.FundamentalsTTLDays, cfg.Data.SharesWorkers)
	if err != nil {
		return nil, err
	}
	bench := FetchBenchmark(ctx, y)
	if isNaN(bench.Ret3M) || isNaN(bench.Ret6M) {
		logx.Debugf("Sin historial del %s: los rendimientos se miden solo contra cero", benchmarkTicker)
	}
	stored := storedBars(d)
	res := gohoo.ParallelMap(ctx, ts, cfg.Data.SharesWorkers, func(ctx context.Context, t target) (Raw, error) {
		snap, ok := snaps[t.ticker]
		if !ok {
			return Raw{}, fmt.Errorf("sin datos fundamentales: %w", gohoo.ErrNotFound)
		}
		bars, ok := stored[t.ticker]
		if !ok {
			bars = fetchBars(ctx, y, t.ticker)
		}
		return FromSnapshot(t.ticker, snap, bars, bench)
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
	for _, u := range []UniverseInfo{{SP500, "S&P 500", "USD", 0}} {
		if u.Count = count[u.ID]; u.Count > 0 {
			p.Universes = append(p.Universes, u)
		}
	}
	for _, s := range Sections {
		p.Sections = append(p.Sections, SectionInfo{s.ID, s.SectionDef})
	}
	return p
}

func Write(path string, p *Payload) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(bytes.TrimRight(buf.Bytes(), "\n"))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

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
