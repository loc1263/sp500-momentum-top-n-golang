package actual

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"top3/internal/config"
	"top3/internal/console"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/engine"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/numfmt"
	"top3/internal/repo"
	"top3/internal/yahoo"
)

// CurrentTickers es la lista actual del S&P 500 (descargada hoy o la guardada) con los alias aplicados.
func CurrentTickers(d *sql.DB, aliases map[string]string, useCache bool, s config.Data) ([]string, error) {
	if !(useCache && db.FetchedToday(d, "constituents_current_fetched")) {
		if err := market.DownloadCurrent(d, s); err != nil {
			logx.Errorf(errs.NetCurrentList.Msg("No se pudo descargar la lista actual; se usa la guardada en SQLite (%v)"), err)
		}
	} else {
		logx.Infof("Lista actual del S&P 500: usando caché SQLite (descargada hoy)")
	}
	current, err := repo.Current(d)
	if err != nil {
		return nil, err
	}
	if len(current) == 0 {
		return nil, errs.DatNoCurrent.Errorf("No hay lista actual del S&P 500 (ni descargada ni en SQLite).")
	}
	seen := map[string]bool{}
	var tickers []string
	for _, t := range current {
		if a, ok := aliases[t]; ok {
			t = a
		}
		if !seen[t] {
			seen[t] = true
			tickers = append(tickers, t)
		}
	}
	logx.Infof("Tickers del S&P 500 actual: %d", len(tickers))
	return tickers, nil
}

// GetPrices son los precios desde start: usa la base SQLite si se descargó hoy (la deja el backtest) y completa lo que falte.
func GetPrices(ctx context.Context, dl *market.Downloader, d *sql.DB, tickers []string, start, end string, useCache bool,
	s config.Data) (*model.Prices, error) {
	var parts []*model.Prices
	missing := tickers
	cachedStart, _ := db.GetMeta(d, "prices_start")
	if useCache && db.FetchedToday(d, "prices_fetched") && cachedStart != "" && cachedStart <= start {
		stored, err := repo.PriceMatrixSince(d, start)
		if err != nil {
			return nil, err
		}
		stored = stored.Select(tickers)
		last := "None"
		if stored.Rows() > 0 {
			last = dateutil.Day(stored.Dates[stored.Rows()-1])
		}
		logx.Infof("Precios: %d tickers desde la caché SQLite (hasta %s)", stored.Cols(), last)
		parts = append(parts, stored)
		missing = nil
		for _, t := range tickers {
			if !stored.Has(t) {
				missing = append(missing, t)
			}
		}
	}
	if len(missing) > 0 {
		logx.Infof("Descargando precios de %d tickers (%s a %s)...", len(missing), start, end)
		p, err := dl.DownloadRecent(ctx, missing, start, end, s.BatchSize)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return model.MergeColumns(parts...), nil
}

// ComputeTop son los topN (+ extra suplentes) tickers con mayor retorno en los últimos `months` meses y la ventana usada
// (solo los tickers sin huecos en ella).
func ComputeTop(prices *model.Prices, months, topN, tradingDaysPerMonth int, universe *market.Universe, extra int) ([]engine.Ranked, *model.Prices, error) {
	lookback := months * tradingDaysPerMonth
	prices = prices.DropAllNaN()
	if prices.Rows() == 0 {
		return nil, nil, errs.ActShortWindow.Errorf("Solo hay 0 ruedas de precios; se necesitan %d.", lookback+1)
	}
	if universe != nil {
		all := prices.Tickers
		last := engine.NewMatrix(prices).LastPrices(all, prices.Dates[max(prices.Rows()-1, 0)])
		big := universe.Select(all, last)
		logx.Infof("Filtro de capitalización (>= US$ %.0f B): %d -> %d tickers", universe.MinCapUSD()/1e9, len(all), len(big))
		prices = prices.Select(big)
	}
	window := prices.Tail(lookback + 1)
	if window.Rows() < lookback+1 {
		return nil, nil, errs.ActShortWindow.Errorf("Solo hay %d ruedas de precios; se necesitan %d.", window.Rows(), lookback+1)
	}
	// Misma regla que el backtest: un ticker con huecos en la ventana no puede rankearse
	complete := window.CompleteCols()
	if n := window.Cols() - len(complete); n > 0 {
		in := map[string]bool{}
		for _, t := range complete {
			in[t] = true
		}
		var incomplete []string
		for _, t := range window.Tickers {
			if !in[t] {
				incomplete = append(incomplete, t)
			}
		}
		more := ""
		if len(incomplete) > 15 {
			more, incomplete = " ...", incomplete[:15]
		}
		logx.Warnf(errs.ActGaps.Msg("%d tickers con huecos en la ventana quedan fuera del ranking: %s%s"), n, strings.Join(incomplete, ", "), more)
	}
	m := engine.NewMatrix(prices)
	ranked, ok := engine.RankReturns(m, prices.Tickers, prices.Dates[prices.Rows()-1], lookback, nil, 0)
	if !ok {
		return nil, nil, errs.ActShortWindow.Errorf("Solo hay %d ruedas de precios; se necesitan %d.", prices.Rows(), lookback+1)
	}
	window = window.Select(complete)
	logx.Debugf("Ventana: %s a %s, %d ruedas, %d tickers completos", dateutil.Day(window.Dates[0]),
		dateutil.Day(window.Dates[window.Rows()-1]), window.Rows(), window.Cols())
	return ranked[:min(topN+extra, len(ranked))], window, nil
}

// Result es el Top actual calculado: los elegidos, los suplentes y la ventana de precios usada.
type Result struct {
	Months     int
	TopN       int
	MinCapB    float64
	AsOf       time.Time
	Window     *model.Prices
	Picks      []repo.Pick
	Alternates []repo.Pick
}

// Compute baja (o toma de SQLite) los datos y calcula el Top actual y sus suplentes. No guarda ni imprime nada.
func Compute(ctx context.Context, dl *market.Downloader, d *sql.DB, cfg config.Config, months, topN int, minCapB float64, useCache bool) (*Result, error) {
	s := cfg.Data
	today := dateutil.Today()
	start := dateutil.Day(today.AddDate(0, 0, -(months*31 + 45))) // margen para cubrir las ruedas necesarias
	end := dateutil.Day(today.AddDate(0, 0, 1))
	tickers, err := CurrentTickers(d, cfg.Tickers.Aliases, useCache, s)
	if err != nil {
		return nil, err
	}
	prices, err := GetPrices(ctx, dl, d, tickers, start, end, useCache, s)
	if err != nil {
		return nil, err
	}

	var known map[string]float64
	var universe *market.Universe
	if minCapB != 0 {
		if known, err = dl.EnsureShares(ctx, d, prices.Tickers, useCache, s, nil); err != nil {
			return nil, err
		}
		universe = &market.Universe{Shares: known, MinCapBillions: minCapB}
	}
	ranked, window, err := ComputeTop(prices, months, topN, cfg.Backtest.TradingDaysPerMonth, universe, cfg.Backtest.Alternates)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ranked))
	for i, r := range ranked {
		names[i] = r.Ticker
	}
	shares, err := dl.EnsureShares(ctx, d, names, useCache, s, known)
	if err != nil {
		return nil, err
	}
	picks := make([]repo.Pick, len(ranked))
	for i, r := range ranked {
		j, _ := window.Col(r.Ticker)
		price := window.At(window.Rows()-1, j)
		p := repo.Pick{Rank: i + 1, Ticker: r.Ticker, ReturnLookback: r.Return, Price: price}
		if sh, ok := shares[r.Ticker]; ok {
			cap := price * sh
			p.MarketCap = &cap
		}
		picks[i] = p
	}
	n := min(topN, len(picks))
	return &Result{months, topN, minCapB, window.Dates[window.Rows()-1], window, picks[:n], picks[n:]}, nil
}

// Save guarda el Top actual (repetir la corrida con los mismos datos reemplaza la anterior) y devuelve las notas de
// rotación: qué entró y salió contra la corrida anterior y cuándo toca rebalancear.
func Save(d *sql.DB, r *Result, today time.Time) ([]string, error) {
	asOf := dateutil.Day(r.AsOf)
	prevAsOf, prev, err := repo.PreviousPicks(d, asOf, r.Months, r.MinCapB)
	if err != nil {
		return nil, err
	}
	if err := repo.ReplacePicks(d, time.Now().Format("2006-01-02T15:04:05"), asOf, r.Months, r.MinCapB, r.Picks, r.Alternates); err != nil {
		return nil, err
	}
	logx.Infof("Resultado guardado en SQLite (tabla actual_picks)")
	tickers := make([]string, len(r.Picks))
	for i, p := range r.Picks {
		tickers[i] = p.Ticker
	}
	anchor, err := repo.AnchorDate(d, r.Months, r.MinCapB)
	if err != nil {
		return nil, err
	}
	return Describe(prevAsOf, DiffPicks(prev, tickers), anchor, r.Months, today), nil
}

// Present imprime el Top actual por consola.
func Present(r *Result, notes []string) {
	weight := 100 / float64(len(r.Picks))
	headers := []string{"#", "Ticker", fmt.Sprintf("Retorno %d meses", r.Months), "Cap. aprox. (US$ B)", "Precio (US$)", "Peso"}
	row := func(p repo.Pick, w string) []string {
		cap := "n/d"
		if p.MarketCap != nil && *p.MarketCap != 0 {
			cap = numfmt.Comma(*p.MarketCap/1e9, 0)
		}
		return []string{fmt.Sprint(p.Rank), p.Ticker, numfmt.Signed(p.ReturnLookback*100, 1) + "%", cap, numfmt.Comma(p.Price, 2), w}
	}
	var table, alts [][]string
	for _, p := range r.Picks {
		table = append(table, row(p, fmt.Sprintf("%.1f%%", weight)))
	}
	for _, p := range r.Alternates {
		alts = append(alts, row(p, "-"))
	}
	console.Actual(r.TopN, r.Months, r.MinCapB, dateutil.Day(r.AsOf), dateutil.Day(r.Window.Dates[0]), r.Window.Rows(),
		headers, table, weight, notes, alts)
}

// Run calcula el Top actual, lo guarda en SQLite y lo imprime.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, months, topN int, minCapB float64, useCache bool) error {
	logx.Infof("Plazo: %d meses | Top %d | Universo: %s", months, topN, market.Label(minCapB))
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	r, err := Compute(ctx, dl, d, cfg, months, topN, minCapB, useCache)
	if err != nil {
		return err
	}
	notes, err := Save(d, r, dateutil.Today())
	if err != nil {
		return err
	}
	Present(r, notes)
	return nil
}
