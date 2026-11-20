// Corrida del Visor: descarga el historial, calcula métricas por instrumento y escribe el resultado.
//
// Fecha: 2026-11-19

package visor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"top3/internal/config"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/yahoo"
)

const RefTicker = "SPY"

const historyRange = "6y"

const NoCategory = "Sin categoría"

type Payload struct {
	GeneratedAt string    `json:"generated_at"`
	Date        string    `json:"date"`
	Ref         string    `json:"ref"`
	RefYTD      float64   `json:"ref_ytd"`
	Rf          float64   `json:"rf"`
	Sections    []Section `json:"sections"`
}

type fetched struct {
	series   Series
	divs     map[string]float64
	sector   string
	category string
}

func fetch(ctx context.Context, y *yahoo.Client, ticker string) (fetched, error) {
	res, err := y.ChartRange(ctx, ticker, gohoo.ChartParams{Range: historyRange})
	if err != nil {
		return fetched{}, err
	}
	f := fetched{series: SeriesOf(res.Bars), divs: res.Dividends}
	if len(f.series.Close) < 2 {
		return fetched{}, fmt.Errorf("sin historial")
	}
	if info, err := y.Info(ctx, ticker); err == nil {
		f.sector = strings.TrimSpace(gohoo.AsString(info["sector"]))
		f.category = strings.TrimSpace(gohoo.AsString(info["category"]))
	} else {
		logx.Debugf("Visor: sin sector ni categoría para %s: %v", ticker, err)
	}
	return f, nil
}

func universe(ctx context.Context, cfg config.Config, y *yahoo.Client, p Profile) (map[string]Static, []string, error) {
	v := cfg.Visor
	if !p.IsETF() {
		return StockUniverse(ctx, y, v.CapMinBillions*1e9)
	}
	var statics map[string]Static
	var order []string
	if v.EtfTop > 0 {
		var err error
		if statics, order, err = ETFUniverse(ctx, y, v.EtfTop, v.EtfAumMinMillions*1e6); err != nil {
			return nil, nil, err
		}
	} else {
		statics = map[string]Static{}
	}
	var missing []string
	for _, t := range explicitTickers(v) {
		if _, ok := statics[t]; !ok {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		extra, err := QuoteStatics(ctx, y, missing)
		if err != nil {
			logx.Warnf(errs.VisNoData.Msg("No se pudieron cotizar los ETFs pedidos %v: %v"), missing, err)
		}
		maps.Copy(statics, extra)
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range append(explicitTickers(v), order...) {
		if _, ok := statics[t]; ok && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return statics, out, nil
}

func explicitTickers(v config.Visor) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range v.Tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func section(ctx context.Context, cfg config.Config, y *yahoo.Client, p Profile, ref Ref, spy Returns) (Section, error) {
	statics, tickers, err := universe(ctx, cfg, y, p)
	if err != nil {
		return Section{}, err
	}
	if len(tickers) == 0 {
		return Section{}, errs.VisNoUniverse.Errorf("%s: el screener no devolvió instrumentos", p.Label)
	}
	logx.Infof("Visor: %s: descargando el historial de %d instrumentos...", p.Label, len(tickers))
	res := gohoo.ParallelMap(ctx, tickers, cfg.Data.SharesWorkers, func(ctx context.Context, t string) (fetched, error) {
		return fetch(ctx, y, t)
	})
	explicit := map[string]bool{}
	if p.IsETF() {
		for _, t := range explicitTickers(cfg.Visor) {
			explicit[t] = true
		}
	}
	var ins []Instrument
	var missing []string
	for i, r := range res {
		t := tickers[i]
		if r.Err != nil {
			logx.Debugf("Visor: sin historial para %s: %v", t, r.Err)
			missing = append(missing, t)
			continue
		}
		m, ok := Compute(r.Value.series, r.Value.divs, spy)
		if !ok {
			missing = append(missing, t)
			continue
		}
		in := Instrument{Ticker: t, Static: statics[t], Metrics: m, Sector: r.Value.sector, Category: r.Value.category}
		if p.IsETF() {
			if in.Category == "" {
				in.Category = NoCategory
			}
			in.Leveraged = IsLeveraged(in.Name, in.Category)
		}
		ins = append(ins, in)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		logx.Warnf(errs.VisNoData.Msg("%s: sin historial para %d de %d instrumentos: %v"), p.Label, len(missing), len(tickers), missing)
	}
	if float64(len(ins)) < cfg.Visor.MinCoverage*float64(len(tickers)) {
		return Section{}, errs.VisTooFew.Errorf("%s: solo %d de %d instrumentos tienen datos (mínimo %.0f%%)", p.Label, len(ins), len(tickers), cfg.Visor.MinCoverage*100)
	}
	return Build(p, ins, ref, cfg.Visor, explicit), nil
}

func Run(ctx context.Context, cfg config.Config, y *yahoo.Client) (*Payload, error) {
	res, err := y.ChartRange(ctx, RefTicker, gohoo.ChartParams{Range: historyRange})
	if err != nil {
		return nil, errs.VisNoRef.Errorf("%v", err)
	}
	spy := SeriesOf(res.Bars)
	spyYTD := YTD(spy)
	if spyYTD == nil {
		return nil, errs.VisNoRef.Errorf("el historial de %s no llega al año anterior", RefTicker)
	}
	ref := Ref{YTD: *spyYTD, Rets: map[int]*float64{}}
	for _, yr := range RelYears {
		ref.Rets[yr] = PeriodReturn(spy.Close, TradingDays*yr)
	}
	spyRet := ReturnsOf(spy, TradingDays*3)

	now := time.Now()
	p := &Payload{GeneratedAt: now.Format("2006-01-02T15:04:05"), Date: now.Format(time.DateOnly), Ref: RefTicker, RefYTD: ref.YTD, Rf: cfg.Visor.Rf}
	for _, prof := range Profiles {
		sec, err := section(ctx, cfg, y, prof, ref, spyRet)
		if err != nil {
			return nil, err
		}
		p.Sections = append(p.Sections, sec)
	}
	if err := Write(cfg.Visor.Output, p); err != nil {
		return nil, err
	}
	logx.Infof("Visor: %s guardado en %s", p.Summary(), cfg.Visor.Output)
	return p, nil
}

func (p *Payload) Summary() string {
	parts := make([]string, len(p.Sections))
	for i, s := range p.Sections {
		parts[i] = fmt.Sprintf("%s: %d", s.Label, len(s.Rows))
	}
	return strings.Join(parts, ", ")
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
