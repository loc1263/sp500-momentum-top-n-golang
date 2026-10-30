package ipsa

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/quant"
	"top3/internal/repo"
	"top3/internal/yahoo"
)

// Currency es la moneda de los precios y las capitalizaciones.
const Currency = "CLP"

// Source dice de dónde salió la lista de acciones.
type Source struct {
	UpdatedAt string `json:"updated_at"`
	URL       string `json:"url"`
}

// Payload es el resultado del modelo del IPSA: el del modelo por score más la moneda y el origen de la lista.
type Payload struct {
	quant.Payload
	Currency string `json:"currency"`
	Universe Source `json:"universe"`
}

// QuantConfig es la configuración del modelo por score para el IPSA: lo de [quant] con la tasa y los límites de [ipsa] encima.
func QuantConfig(cfg config.Config) config.Config {
	c := cfg.Ipsa
	cfg.Quant.TopN, cfg.Quant.VolTarget, cfg.Quant.WMax, cfg.Quant.WMin, cfg.Quant.WStep, cfg.Quant.SectorMax = c.TopN, c.VolTarget, c.WMax, c.WMin, c.WStep, c.SectorMax
	cfg.Quant.RiskFree, cfg.Quant.EquityPremium = c.RiskFree, c.EquityPremium
	return cfg
}

// fetchOHLCV baja el OHLCV de los tickers a memoria (no toca la tabla ohlcv del S&P 500).
func fetchOHLCV(ctx context.Context, dl *market.Downloader, tickers []string, start, end time.Time) (repo.OHLCVFrames, error) {
	out := yahoo.ParallelMap(ctx, tickers, dl.Workers, func(ctx context.Context, t string) ([]repo.OHLCVRow, error) {
		bars, err := dl.Y.DailyBars(ctx, t, start, end)
		if err != nil {
			return nil, err
		}
		var rs []repo.OHLCVRow
		seen := map[string]int{}
		for _, b := range bars {
			if math.IsNaN(b.Close) {
				continue
			}
			v := b.Volume
			if math.IsNaN(v) {
				v = 0
			}
			r := repo.OHLCVRow{Ticker: t, Date: dateutil.Day(b.Date()), Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: v}
			if k, dup := seen[r.Date]; dup {
				rs[k] = r
				continue
			}
			seen[r.Date] = len(rs)
			rs = append(rs, r)
		}
		return rs, nil
	})
	var rows []repo.OHLCVRow
	var missing []string
	for k, r := range out {
		if r.Err != nil || len(r.Value) == 0 {
			missing = append(missing, tickers[k])
			continue
		}
		rows = append(rows, r.Value...)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		logx.Warnf(errs.DatNoOHLCV.Msg("Sin OHLCV para %d acciones del IPSA (quedan fuera del score técnico): %v"), len(missing), missing)
	}
	return repo.OHLCVFramesFromRows(rows), nil
}

// Run arma la lista del IPSA, baja precios, OHLCV y datos financieros, calcula el modelo por score y lo guarda en SQLite.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, useCache bool) (*Payload, error) {
	uni, err := EnsureUniverse(cfg, useCache, time.Now())
	if err != nil {
		return nil, err
	}
	tickers := make([]string, len(uni.Stocks))
	names := map[string]string{}
	for i, s := range uni.Stocks {
		tickers[i] = YahooTicker(s, cfg.Ipsa)
		names[tickers[i]] = s.Name
	}
	sort.Strings(tickers)

	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	today := dateutil.Today()
	start, end := today.AddDate(0, 0, -quant.HistoryDays), today.AddDate(0, 0, 1)
	logx.Infof("Descargando precios de %d acciones del IPSA (%s a %s)...", len(tickers), dateutil.Day(start), dateutil.Day(end))
	prices, err := dl.DownloadRecent(ctx, tickers, dateutil.Day(start), dateutil.Day(end), cfg.Data.BatchSize)
	if err != nil {
		return nil, err
	}
	ohlcv, err := fetchOHLCV(ctx, dl, tickers, start, end)
	if err != nil {
		return nil, err
	}
	data, err := quant.EnsureFundamentals(ctx, d, y, tickers, useCache, cfg.Quant.FundamentalsTTLDays, cfg.Data.SharesWorkers)
	if err != nil {
		return nil, err
	}
	p, err := quant.Compute(quant.ComputeInput{Prices: prices, Tickers: tickers, Data: data, OHLCV: ohlcv}, QuantConfig(cfg))
	if err != nil {
		return nil, errs.IpsNoSignal.Errorf("%v", err)
	}
	for i := range p.Ranking {
		if p.Ranking[i].Name == "" {
			p.Ranking[i].Name = names[p.Ranking[i].Ticker]
		}
	}
	out := &Payload{Payload: *p, Currency: Currency, Universe: Source{UpdatedAt: uni.UpdatedAt, URL: uni.Source}}
	js, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if err := repo.SaveLatestIpsa(d, out.GeneratedAt, out.AsOf, js); err != nil {
		return nil, err
	}
	logx.Infof("Modelo del IPSA guardado en SQLite (tabla ipsa_runs)")
	return out, nil
}

// Summary es el texto de la consola con las mejores acciones.
func (p *Payload) Summary() []string {
	lines := []string{fmt.Sprintf("IPSA al %s (%d acciones en el ranking). Las mejores:", p.AsOf, p.Coverage.Ranked)}
	return append(lines, p.Payload.Summary()[1:]...)
}
