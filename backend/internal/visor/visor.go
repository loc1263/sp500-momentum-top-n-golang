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
	"sync"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/company"
	"topwx/internal/config"
	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
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

// session reparte entre las secciones del Visor lo que ya se bajó de Yahoo en la corrida: el screener de acciones, el Info de cada
// empresa y el historial de cada instrumento. Las acciones grandes están en «Acciones» y en «Más de 50 años»: se piden una vez.
type session struct {
	y   *yahoo.Client
	cfg config.Config

	mu     sync.Mutex
	infos  map[string]map[string]any
	charts map[string]fetched
	// cats son las categorías de ETFs que etfq ya guardó en SQLite (vigentes según su TTL): no hace falta pedir su Info.
	cats map[string]string
	// sectors son los sectores que ya están en el caché de datos de empresas (company): evitan pedir el Info de cada acción.
	sectors map[string]string

	stocks       map[string]Static
	stocksOrder  []string
	stocksCapMin float64
	stocksLoaded bool
}

func newSession(cfg config.Config, y *yahoo.Client) *session {
	s := &session{y: y, cfg: cfg, infos: map[string]map[string]any{}, charts: map[string]fetched{}}
	s.loadCaches()
	return s
}

// loadCaches lee de SQLite lo que otros comandos ya bajaron y sigue vigente: las categorías de ETFs de etfq y los sectores de las
// empresas del S&P 500 (quant y evaluador). Sin base o sin tablas quedan vacíos y el Visor pide el Info a Yahoo.
func (s *session) loadCaches() {
	s.cats, s.sectors = map[string]string{}, map[string]string{}
	d, err := db.Open(s.cfg.Data.DBPath)
	if err != nil {
		logx.Debugf("Visor: sin cachés de SQLite: %v", err)
		return
	}
	defer d.Close()
	s.sectors = company.Sectors(d, s.cfg.Quant.FundamentalsTTLDays)
	saved, err := repo.EtfqCategories(d)
	if err != nil {
		logx.Debugf("Visor: sin caché de categorías de etfq: %v", err)
		return
	}
	oldest := dateutil.Today().AddDate(0, 0, -s.cfg.Etfq.CategoryTTLDays)
	for t, c := range saved {
		if day, err := dateutil.Parse(c[1]); err == nil && !day.Before(oldest) {
			s.cats[t] = c[0]
		}
	}
}

// saveCategories deja en SQLite las categorías que el Visor bajó, para que etfq no las pida de nuevo.
func (s *session) saveCategories(cats map[string]string) {
	if len(cats) == 0 {
		return
	}
	d, err := db.Open(s.cfg.Data.DBPath)
	if err != nil {
		logx.Debugf("Visor: no se pudieron guardar las categorías: %v", err)
		return
	}
	defer d.Close()
	if err := repo.SaveEtfqCategories(d, cats); err != nil {
		logx.Debugf("Visor: no se pudieron guardar las categorías: %v", err)
	}
}

func (s *session) info(ctx context.Context, ticker string) (map[string]any, error) {
	s.mu.Lock()
	in, ok := s.infos[ticker]
	s.mu.Unlock()
	if ok {
		return in, nil
	}
	in, err := s.y.Info(ctx, ticker)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.infos[ticker] = in
	s.mu.Unlock()
	return in, nil
}

func (s *session) fetch(ctx context.Context, ticker string) (fetched, error) {
	s.mu.Lock()
	f, ok := s.charts[ticker]
	s.mu.Unlock()
	if ok {
		return f, nil
	}
	res, err := s.y.ChartRange(ctx, ticker, gohoo.ChartParams{Range: historyRange})
	if err != nil {
		return fetched{}, err
	}
	f = fetched{series: SeriesOf(res.Bars), divs: res.Dividends}
	if len(f.series.Close) < 2 {
		return fetched{}, fmt.Errorf("sin historial")
	}
	if cat, ok := s.cats[ticker]; ok {
		f.category = cat
	} else if sec, ok := s.sectors[ticker]; ok {
		f.sector = sec
	} else if info, err := s.info(ctx, ticker); err == nil {
		f.sector = strings.TrimSpace(gohoo.AsString(info["sector"]))
		f.category = strings.TrimSpace(gohoo.AsString(info["category"]))
	} else {
		logx.Debugf("Visor: sin sector ni categoría para %s: %v", ticker, err)
	}
	s.mu.Lock()
	s.charts[ticker] = f
	s.mu.Unlock()
	return f, nil
}

// pending cuenta los tickers cuyo historial todavía no se bajó en esta corrida.
func (s *session) pending(tickers []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, t := range tickers {
		if _, ok := s.charts[t]; !ok {
			n++
		}
	}
	return n
}

// stockUniverse son las acciones con capitalización mayor a capMin. El screener se consulta una sola vez, con la menor
// capitalización que piden las secciones, y de ahí se filtra para cada una.
func (s *session) stockUniverse(ctx context.Context, capMin float64) (map[string]Static, []string, error) {
	if !s.stocksLoaded {
		v := s.cfg.Visor
		s.stocksCapMin = min(v.CapMinBillions, v.VeteranCapMinB) * 1e9
		var err error
		if s.stocks, s.stocksOrder, err = StockUniverse(ctx, s.y, s.stocksCapMin); err != nil {
			return nil, nil, err
		}
		s.stocksLoaded = true
	}
	if capMin <= s.stocksCapMin {
		return s.stocks, s.stocksOrder, nil
	}
	statics := map[string]Static{}
	var order []string
	for _, t := range s.stocksOrder {
		if c := s.stocks[t].MarketCap; c != nil && *c > capMin {
			statics[t] = s.stocks[t]
			order = append(order, t)
		}
	}
	return statics, order, nil
}

func (s *session) universe(ctx context.Context, p Profile) (map[string]Static, []string, error) {
	v := s.cfg.Visor
	if p.IsVeteran() {
		statics, order, err := s.stockUniverse(ctx, v.VeteranCapMinB*1e9)
		if err != nil {
			return nil, nil, err
		}
		return VeteranUniverse(ctx, statics, order, s.info, v.VeteranYears, s.cfg.Data.SharesWorkers, v.VeteranCache, time.Now())
	}
	if !p.IsETF() {
		return s.stockUniverse(ctx, v.CapMinBillions*1e9)
	}
	y := s.y
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

func (s *session) section(ctx context.Context, p Profile, ref Ref, spy Returns) (Section, error) {
	cfg := s.cfg
	statics, tickers, err := s.universe(ctx, p)
	if err != nil {
		return Section{}, err
	}
	if len(tickers) == 0 {
		return Section{}, errs.VisNoUniverse.Errorf("%s: el screener no devolvió instrumentos", p.Label)
	}
	logx.Infof("Visor: %s: %d instrumentos (%d por descargar, el resto ya se bajó en esta corrida)...", p.Label, len(tickers), s.pending(tickers))
	res := gohoo.ParallelMap(ctx, tickers, cfg.Data.SharesWorkers, func(ctx context.Context, t string) (fetched, error) {
		return s.fetch(ctx, t)
	})
	explicit := map[string]bool{}
	if p.IsETF() {
		for _, t := range explicitTickers(cfg.Visor) {
			explicit[t] = true
		}
	}
	var ins []Instrument
	var missing []string
	newCats := map[string]string{}
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
		if _, known := s.cats[t]; p.IsETF() && !known && r.Value.category != "" {
			newCats[t] = r.Value.category
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
	s.saveCategories(newCats)
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
	ref := Ref{LastYear: time.Now().Year() - 1, YTD: *spyYTD, Rets: map[int]*float64{}}
	for _, yr := range RelYears {
		ref.Rets[yr] = PeriodReturn(spy.Close, TradingDays*yr)
	}
	spyRet := ReturnsOf(spy, TradingDays*3)

	now := time.Now()
	p := &Payload{GeneratedAt: now.Format("2006-01-02T15:04:05"), Date: now.Format(time.DateOnly), Ref: RefTicker, RefYTD: ref.YTD, Rf: cfg.Visor.Rf}
	sess := newSession(cfg, y)
	for _, prof := range Profiles {
		sec, err := sess.section(ctx, prof, ref, spyRet)
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
