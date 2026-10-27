// Package etfq es la sección de ETFs por gestora: los ETFs de iShares, Vanguard, SPDR (State Street), Invesco y Schwab con su
// calidad (costos y liquidez), sin backtest.
//
// Es independiente de la sección de ETFs de Vanguard (package etf), de la que solo reutiliza el cálculo del score de momentum
// (etf.ScoreEtfs, con los plazos, la referencia libre de riesgo y la volatilidad mínima de [etf]). Todo sale de Yahoo Finance:
// la lista de cada gestora, el TER y el AUM vienen del screener (filtrado por fundFamily) y la categoría de Info.
package etfq

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"top3/internal/config"
	"top3/internal/console"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/etf"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/repo"
	"top3/internal/yahoo"
)

// NoCategory es la categoría de los ETFs para los que Yahoo no informa ninguna.
const NoCategory = "Sin categoría"

// Fund es un ETF de una gestora. TER en % anual y AUM en US$ (nil = Yahoo no lo informa).
type Fund struct {
	Ticker string
	Name   string
	Issuer string // nombre para mostrar de la gestora
	TER    *float64
	AUM    *float64
}

// FetchFamily baja del screener de Yahoo todos los ETFs de EE. UU. de una gestora, con su TER y su AUM.
func FetchFamily(ctx context.Context, y *yahoo.Client, family config.Family) ([]Fund, error) {
	label := family.Label
	if label == "" {
		label = family.Name
	}
	var out []Fund
	for offset := 0; ; {
		res, err := y.Screen(ctx, gohoo.ScreenParams{
			Query:     gohoo.And(gohoo.Eq("fundfamilyname", family.Name), gohoo.Eq("region", "us")),
			QuoteType: "ETF", Offset: offset, Size: gohoo.ScreenMaxSize, SortField: "fundnetassets",
		})
		if err != nil {
			return nil, err
		}
		for _, q := range res.Quotes {
			if f, ok := fundOf(q, label); ok {
				out = append(out, f)
			}
		}
		offset += len(res.Quotes)
		if len(res.Quotes) == 0 || offset >= res.Total {
			return out, nil
		}
	}
}

func fundOf(q gohoo.ScreenQuote, issuer string) (Fund, bool) {
	ticker := strings.ToUpper(strings.TrimSpace(q.Symbol))
	if ticker == "" {
		return Fund{}, false
	}
	name := q.LongName
	if name == "" {
		name = q.ShortName
	}
	f := Fund{Ticker: ticker, Name: name, Issuer: issuer}
	if v, ok := gohoo.AsFloat(q.Raw["netExpenseRatio"]); ok && v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
		f.TER = &v
	}
	if v, ok := gohoo.AsFloat(q.Raw["netAssets"]); ok && v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
		f.AUM = &v
	}
	return f, true
}

// LoadFunds baja los ETFs de todas las gestoras. Una que falla o no devuelve nada se avisa (TOP-GES-001) y se sigue con las
// demás. Un ticker que aparece en dos gestoras queda con la primera. Devuelve también los ETFs listados por gestora.
func LoadFunds(ctx context.Context, y *yahoo.Client, families []config.Family) ([]Fund, map[string]int) {
	var funds []Fund
	seen := map[string]bool{}
	listed := map[string]int{}
	for _, fam := range families {
		list, err := FetchFamily(ctx, y, fam)
		label := fam.Label
		if label == "" {
			label = fam.Name
		}
		if err != nil || len(list) == 0 {
			logx.Warnf(errs.GesNoFamily.Msg("%s (%q): %v"), label, fam.Name, errOrEmpty(err))
			continue
		}
		for _, f := range list {
			if !seen[f.Ticker] {
				seen[f.Ticker] = true
				funds = append(funds, f)
				listed[label]++
			}
		}
		logx.Infof("ETFs por gestora: %s: %d ETFs", label, listed[label])
	}
	return funds, listed
}

func errOrEmpty(err error) any {
	if err != nil {
		return err
	}
	return "Yahoo no devolvió ETFs"
}

// FetchCategories baja de Yahoo la categoría (Morningstar) de cada ETF; los que no la informan quedan fuera del mapa.
func FetchCategories(ctx context.Context, y *yahoo.Client, tickers []string, workers int) map[string]string {
	results := yahoo.ParallelMap(ctx, tickers, workers, func(ctx context.Context, ticker string) (string, error) {
		info, err := y.Info(ctx, ticker)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(gohoo.AsString(info["category"])), nil
	})
	out := map[string]string{}
	for i, r := range results {
		if r.Err == nil && r.Value != "" {
			out[tickers[i]] = r.Value
		}
	}
	return out
}

// Options son los parámetros de Run.
type Options struct {
	TopN int // 0 = [etfq] top_n
}

// Run baja las listas de las gestoras, calcula momentum y calidad, guarda el resultado en SQLite y lo imprime.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, opt Options) error {
	q := cfg.Etfq
	topN := opt.TopN
	if topN == 0 {
		topN = q.TopN
	}
	horizons := append([]int(nil), cfg.Etf.HorizonsMonths...)
	sort.Ints(horizons)

	funds, listed := LoadFunds(ctx, y, q.Families)
	if len(funds) == 0 {
		return errs.GesEmpty.Errorf("Ninguna gestora devolvió ETFs.")
	}
	logx.Infof("ETFs por gestora: %d ETFs | plazos %v meses", len(funds), horizons)

	endDay := dateutil.Today()
	tickers := make([]string, len(funds))
	for i, f := range funds {
		tickers[i] = f.Ticker
	}
	start := dateutil.Day(endDay.AddDate(0, 0, -(horizons[len(horizons)-1]*31 + 60))) // margen para las ruedas necesarias
	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	prices, err := dl.DownloadRecent(ctx, tickers, start, dateutil.Day(endDay.AddDate(0, 0, 1)), cfg.Data.BatchSize)
	if err != nil {
		return err
	}
	// Sin las ruedas finales sin cierre (Yahoo tarda en publicar la de hoy): dejarían con hueco a casi todos los ETFs
	prices = prices.Slice(0, sort.Search(prices.Rows(), func(i int) bool { return prices.Dates[i].After(endDay) })).TrimEmptyTail().DropAllNaN()
	if prices.Rows() == 0 {
		return errs.GesEmpty.Errorf("Yahoo no devolvió precios.")
	}

	riskFree := strings.ToUpper(cfg.Etf.RiskFreeTicker)
	scores, excluded := etf.ScoreEtfs(prices, horizons, cfg.Backtest.TradingDaysPerMonth, riskFree, cfg.Etf.MinVol)
	if len(scores.Rows) == 0 {
		return errs.GesEmpty.Errorf("Ningún ETF tiene historia suficiente para el plazo más largo.")
	}
	if riskFree != "" && scores.RiskFree == "" {
		logx.Warnf(errs.EtfNoRiskFree.Msg("La referencia libre de riesgo %s no está disponible: el score se calcula sin restarla"), riskFree)
	}
	logx.Infof("ETFs por gestora: %d rankeados, %d sin historia suficiente o con huecos, %d de tipo efectivo", len(scores.Rows), len(excluded), len(scores.CashLike))

	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	ranked := make([]string, len(scores.Rows))
	for i, r := range scores.Rows {
		ranked[i] = r.Ticker
	}
	categories, err := resolveCategories(ctx, d, y, ranked, q.CategoryTTLDays, cfg.Data.SharesWorkers)
	if err != nil {
		return err
	}

	payload := Assemble(funds, scores, categories, listed, Params{Config: q, Horizons: horizons, NotRanked: len(excluded) + len(scores.CashLike)},
		dateutil.Day(prices.Dates[prices.Rows()-1]), time.Now().Format("2006-01-02T15:04:05"))
	raw, err := payload.JSON()
	if err != nil {
		return err
	}
	if err := repo.SaveLatestEtfq(d, payload.GeneratedAt, payload.AsOf, raw); err != nil {
		return err
	}
	present(payload, topN)
	return nil
}

// resolveCategories devuelve la categoría de cada ticker: la guardada si es reciente y, si no, la que informa Yahoo ahora (que se
// guarda). Los que Yahoo no informa quedan con NoCategory (sin guardar, para reintentar en la próxima corrida).
func resolveCategories(ctx context.Context, d *sql.DB, y *yahoo.Client, tickers []string, ttlDays, workers int) (map[string]string, error) {
	saved, err := repo.EtfqCategories(d)
	if err != nil {
		return nil, err
	}
	oldest := dateutil.Today().AddDate(0, 0, -ttlDays)
	out := map[string]string{}
	var todo []string
	for _, t := range tickers {
		if s, ok := saved[t]; ok {
			if day, err := dateutil.Parse(s[1]); err == nil && !day.Before(oldest) {
				out[t] = s[0]
				continue
			}
		}
		todo = append(todo, t)
	}
	if len(todo) > 0 {
		fresh := FetchCategories(ctx, y, todo, workers)
		if err := repo.SaveEtfqCategories(d, fresh); err != nil {
			return nil, err
		}
		var missing []string
		for _, t := range todo {
			if c, ok := fresh[t]; ok {
				out[t] = c
			} else if s, ok := saved[t]; ok {
				out[t] = s[0] // Yahoo no respondió: mejor la categoría vieja que ninguna
			} else {
				missing = append(missing, t)
			}
		}
		if len(missing) > 0 {
			logx.Warnf(errs.GesNoCategory.Msg("%d ETFs sin categoría: %s"), len(missing), strings.Join(missing[:min(len(missing), 15)], ", "))
		}
		logx.Infof("ETFs por gestora: categoría de %d ETFs descargada (%d ya estaban guardadas)", len(fresh), len(tickers)-len(todo))
	}
	return out, nil
}

// present imprime el ranking final por consola.
func present(p Payload, topN int) {
	console.Banner(fmt.Sprintf("ETFS POR GESTORA - momentum y calidad (%d rankeados)", len(p.Rows)), 100)
	console.Line("Datos al cierre del " + p.AsOf)
	var parts []string
	for _, is := range p.Issuers {
		parts = append(parts, fmt.Sprintf("%s %d", is.Name, is.Funds))
	}
	console.Line("ETFs listados: " + strings.Join(parts, " | "))
	s := p.Settings
	console.Linef("Score final = %s%% momentum + %s%% calidad (TER y AUM dentro de su categoría). Elegibles: TER <= %s%% y AUM >= US$ %s M. Top %d:\n",
		trim((1-s.QualityWeight)*100), trim(s.QualityWeight*100), trim(s.MaxTER), trim(s.MinAumMillions), topN)
	var rows [][]string
	for _, r := range p.Rows {
		if r.Rank == nil || *r.Rank > topN {
			continue
		}
		rows = append(rows, []string{strconv.Itoa(*r.Rank), r.Ticker, r.Issuer, r.Category, fmt.Sprintf("%.1f", *r.Final),
			fmt.Sprintf("%.1f", *r.Momentum), fmt.Sprintf("%.1f", *r.Quality), optional(r.TER, "%.2f%%", 1), optional(r.AUM, "%.1f B", 1e-9)})
	}
	console.Table([]string{"#", "Ticker", "Gestora", "Categoría", "Final", "Momentum", "Calidad", "TER", "AUM"}, rows, 4)
	excluded := 0
	for _, r := range p.Rows {
		if len(r.Excluded) > 0 {
			excluded++
		}
	}
	console.Linef("\nFuera del ranking final por el filtro de TER o AUM: %d. Sin historia suficiente, con huecos o de tipo efectivo: %d.", excluded, p.NotRanked)
}

func trim(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func optional(v *float64, format string, scale float64) string {
	if v == nil {
		return "n/d"
	}
	return fmt.Sprintf(format, *v*scale)
}
