// Descarga, actualiza y carga los precios históricos de los tickers.
//
// Fecha: 2026-11-19

package market

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/config"
	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/model"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
)

type Downloader struct {
	Y       *yahoo.Client
	Workers int
}

type FetchResult struct {
	Rows   []model.PriceRow
	OHLCV  []repo.OHLCVRow // solo si se pidió con fetch(..., true): las últimas OHLCVDays de cada ticker
	NoData []string
	Gone   map[string]bool
}

// OHLCVDays es la ventana de OHLCV que usa el modelo por score. Sale de la misma descarga que los precios ajustados.
const OHLCVDays = 460

// ohlcvRows convierte las barras de un ticker en filas OHLCV desde el día from (inclusive); si una fecha se repite gana la última.
func ohlcvRows(t string, bars []gohoo.Bar, from string) []repo.OHLCVRow {
	var rs []repo.OHLCVRow
	seen := map[string]int{}
	for _, b := range bars {
		day := dateutil.Day(b.Date())
		if math.IsNaN(b.Close) || day < from {
			continue
		}
		v := b.Volume
		if math.IsNaN(v) {
			v = 0
		}
		r := repo.OHLCVRow{Ticker: t, Date: day, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: v}
		if k, dup := seen[day]; dup {
			rs[k] = r
			continue
		}
		seen[day] = len(rs)
		rs = append(rs, r)
	}
	return rs
}

func dayRange(start, end string) (time.Time, time.Time, error) {
	s, err := dateutil.Parse(start)
	if err != nil {
		return s, s, err
	}
	e, err := dateutil.Parse(end)
	return s, e, err
}

func (dl *Downloader) Fetch(ctx context.Context, tickers []string, start, end string, batchSize int) (FetchResult, error) {
	return dl.fetch(ctx, tickers, start, end, batchSize, false)
}

// fetch baja las barras diarias y saca de ellas los precios ajustados de [start, end). Con withOHLCV pide además la ventana de
// OHLCVDays hasta hoy (si start es más reciente la adelanta) y devuelve también esas filas: una sola petición por ticker.
func (dl *Downloader) fetch(ctx context.Context, tickers []string, start, end string, batchSize int, withOHLCV bool) (FetchResult, error) {
	from, to, err := dayRange(start, end)
	if err != nil {
		return FetchResult{}, err
	}
	ohlcvFrom := ""
	if withOHLCV {
		w := dateutil.Today().AddDate(0, 0, -OHLCVDays)
		ohlcvFrom = dateutil.Day(w)
		if w.Before(from) {
			from = w
		}
	}
	res := FetchResult{Gone: map[string]bool{}}
	nBatches := (len(tickers) + batchSize - 1) / batchSize
	var limited []string
	for i := 0; i < len(tickers); i += batchSize {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		batch := tickers[i:min(i+batchSize, len(tickers))]
		type fetched struct {
			prices []model.PriceRow
			ohlcv  []repo.OHLCVRow
		}
		out := gohoo.ParallelMap(ctx, batch, dl.Workers, func(ctx context.Context, t string) (fetched, error) {
			bars, err := dl.Y.DailyBars(ctx, t, from, to)
			if err != nil {
				return fetched{}, err
			}
			byDay := map[string]float64{}
			for _, b := range bars {
				if day := dateutil.Day(b.Date()); day >= start && day < end && !math.IsNaN(b.AdjClose) {
					byDay[day] = b.AdjClose
				}
			}
			rows := make([]model.PriceRow, 0, len(byDay))
			for day, c := range byDay {
				rows = append(rows, model.PriceRow{Ticker: t, Date: day, Close: c})
			}
			sort.Slice(rows, func(a, b int) bool { return rows[a].Date < rows[b].Date })
			f := fetched{prices: rows}
			if withOHLCV {
				f.ohlcv = ohlcvRows(t, bars, ohlcvFrom)
			}
			return f, nil
		})
		got, nRows := 0, 0
		for k, r := range out {
			t := batch[k]
			switch {
			case r.Err == nil && len(r.Value.prices) > 0:
				got++
				nRows += len(r.Value.prices)
				res.Rows = append(res.Rows, r.Value.prices...)
				res.OHLCV = append(res.OHLCV, r.Value.ohlcv...)
				continue
			case r.Err == nil || yahoo.Classify(r.Err) == yahoo.KindAbsent:
				res.Gone[t] = true
				logx.Debugf("%s: sin datos de precios (%v)", t, r.Err)
			case yahoo.Classify(r.Err) == yahoo.KindRateLimit:
				limited = append(limited, t)
			default:
				logx.Warnf(errs.NetYahoo.Msg("%s: %v"), t, r.Err)
			}
			res.NoData = append(res.NoData, t)
		}
		logx.Infof("Lote %d/%d: %d tickers con datos, %d filas", i/batchSize+1, nBatches, got, nRows)
	}
	if len(limited) > 0 {
		logx.Errorf(errs.NetRateLimit.Msg("Yahoo siguió limitando %d tickers tras %d reintentos; quedan sin datos: %s"),
			len(limited), yahoo.RateLimitRetries, strings.Join(limited, ", "))
	}
	return res, nil
}

func (dl *Downloader) DownloadRecent(ctx context.Context, tickers []string, start, end string, batchSize int) (*model.Prices, error) {
	res, err := dl.Fetch(ctx, tickers, start, end, batchSize)
	if err != nil {
		return nil, err
	}
	return model.FromRows(res.Rows), nil
}

func uniqueTickers(rows []model.PriceRow) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if !seen[r.Ticker] {
			seen[r.Ticker] = true
			out = append(out, r.Ticker)
		}
	}
	return out
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// saveOHLCV guarda el OHLCV que salió de la descarga de precios y anota que quedó al día.
func saveOHLCV(d *sql.DB, rows []repo.OHLCVRow) error {
	if len(rows) == 0 {
		return nil
	}
	oldest := dateutil.Day(dateutil.Today().AddDate(0, 0, -OHLCVDays))
	if err := repo.SaveOHLCV(d, rows, oldest); err != nil {
		return err
	}
	return db.SetMeta(d, OHLCVFetchedKey, db.Today())
}

// OHLCVFetchedKey es la marca (fecha) de la última vez que se guardó el OHLCV.
const OHLCVFetchedKey = "ohlcv_fetched"

func (dl *Downloader) DownloadPrices(ctx context.Context, d *sql.DB, tickers []string, start, end string, s config.Data) error {
	known, err := repo.KnownNoData(d, s.NoDataTTLDays)
	if err != nil {
		return err
	}
	var skip, todo []string
	for _, t := range tickers {
		if known[t] {
			skip = append(skip, t)
		} else {
			todo = append(todo, t)
		}
	}
	if len(skip) > 0 {
		logx.Infof("Precios: %d tickers sin datos en Yahoo (consultados hace menos de %d días) no se vuelven a pedir",
			len(skip), s.NoDataTTLDays)
	}
	logx.Infof("Descargando precios de %d tickers (%s a %s) en lotes de %d...", len(todo), start, end, s.BatchSize)
	res, err := dl.fetch(ctx, todo, start, end, s.BatchSize, true)
	if err != nil {
		return err
	}
	if len(res.Rows) == 0 && len(todo) > 0 && repo.HasPrices(d) {
		return errs.NetYahoo.Errorf("No se descargó ningún precio; se conservan los guardados (revisa la conexión)")
	}
	if err := repo.ClearPrices(d); err != nil {
		return err
	}
	if err := repo.SavePrices(d, res.Rows); err != nil {
		return err
	}
	if err := saveOHLCV(d, res.OHLCV); err != nil {
		return err
	}
	if err := repo.RememberNoData(d, keys(res.Gone), uniqueTickers(res.Rows)); err != nil {
		return err
	}
	for k, v := range map[string]string{"prices_fetched": db.Today(), "prices_start": start, "prices_end": end} {
		if err := db.SetMeta(d, k, v); err != nil {
			return err
		}
	}
	logx.Infof("Precios guardados: %d filas | tickers sin datos: %d", len(res.Rows), len(res.NoData)+len(skip))
	sort.Strings(res.NoData)
	logx.Debugf("Tickers sin datos (delisted/renombrados): %v", res.NoData)
	return nil
}

func (dl *Downloader) UpdatePrices(ctx context.Context, d *sql.DB, tickers []string, start, end string, s config.Data) error {
	last, err := repo.LastDates(d)
	if err != nil {
		return err
	}
	newestDay := ""
	for _, v := range last {
		newestDay = max(newestDay, v)
	}
	newest, err := dateutil.Parse(newestDay)
	if err != nil {
		return err
	}
	staleBefore := dateutil.Day(newest.AddDate(0, 0, -s.StaleDays))
	skip, err := repo.KnownNoData(d, s.NoDataTTLDays)
	if err != nil {
		return err
	}
	var fresh, active []string
	for _, t := range tickers {
		if l, ok := last[t]; !ok {
			if !skip[t] {
				fresh = append(fresh, t)
			}
		} else if l >= staleBefore {
			active = append(active, t)
		}
	}
	logx.Infof("Actualización incremental de precios: %d tickers nuevos, %d a actualizar, %d sin cotizar",
		len(fresh), len(active), len(tickers)-len(fresh)-len(active))

	refetch := append([]string(nil), fresh...)
	if len(active) > 0 {
		minLast := last[active[0]]
		for _, t := range active {
			minLast = min(minLast, last[t])
		}
		m, err := dateutil.Parse(minLast)
		if err != nil {
			return err
		}
		since := dateutil.Day(m.AddDate(0, 0, -s.PriceOverlapDays))
		res, err := dl.fetch(ctx, active, since, end, s.BatchSize, true)
		if err != nil {
			return err
		}
		stored, err := repo.StoredSince(d, since)
		if err != nil {
			return err
		}
		drift := map[string]float64{}
		for _, r := range res.Rows {
			if old, ok := stored[[2]string{r.Ticker, r.Date}]; ok {
				drift[r.Ticker] = max(drift[r.Ticker], math.Abs(r.Close/old-1))
			}
		}
		var changed, noOverlap []string
		for _, t := range active {
			v, ok := drift[t]
			switch {
			case !ok:
				noOverlap = append(noOverlap, t)
			case v > s.PriceDriftTolerance:
				changed = append(changed, t)
			}
		}
		sort.Strings(changed)
		sort.Strings(noOverlap)
		refetch = append(append(refetch, changed...), noOverlap...)
		logx.Infof("Precios: %d tickers cambiaron su ajuste (dividendo/split) y se vuelven a bajar completos; %d sin solape",
			len(changed), len(noOverlap))
		logx.Debugf("Ajuste cambiado: %v | sin solape: %v", changed, noOverlap)
		redo := map[string]bool{}
		for _, t := range append(append([]string(nil), changed...), noOverlap...) {
			redo[t] = true
		}
		var keep []model.PriceRow
		for _, r := range res.Rows {
			if !redo[r.Ticker] {
				keep = append(keep, r)
			}
		}
		if err := repo.SavePrices(d, keep); err != nil {
			return err
		}
		var keepOHLCV []repo.OHLCVRow
		for _, r := range res.OHLCV {
			if !redo[r.Ticker] {
				keepOHLCV = append(keepOHLCV, r)
			}
		}
		if err := saveOHLCV(d, keepOHLCV); err != nil {
			return err
		}
	}
	if len(refetch) > 0 {
		res, err := dl.fetch(ctx, refetch, start, end, s.BatchSize, true)
		if err != nil {
			return err
		}
		transient := map[string]bool{}
		for _, t := range res.NoData {
			if !res.Gone[t] {
				transient[t] = true
			}
		}
		var replace []string
		for _, t := range refetch {
			if !transient[t] {
				replace = append(replace, t)
			}
		}
		if err := repo.DeletePriceTickers(d, replace); err != nil {
			return err
		}
		if err := repo.SavePrices(d, res.Rows); err != nil {
			return err
		}
		if err := saveOHLCV(d, res.OHLCV); err != nil {
			return err
		}
		if err := repo.RememberNoData(d, keys(res.Gone), uniqueTickers(res.Rows)); err != nil {
			return err
		}
		sort.Strings(res.NoData)
		logx.Debugf("Tickers sin datos (delisted/renombrados): %v", res.NoData)
	}
	if err := db.SetMeta(d, "prices_fetched", db.Today()); err != nil {
		return err
	}
	return db.SetMeta(d, "prices_end", end)
}

func (dl *Downloader) EnsurePrices(ctx context.Context, d *sql.DB, tickers []string, start, end string, useCache bool, s config.Data) error {
	cachedStart, _ := db.GetMeta(d, "prices_start")
	hasData := repo.HasPrices(d)
	switch {
	case !useCache:
		return dl.DownloadPrices(ctx, d, tickers, start, end, s)
	case db.FetchedToday(d, "prices_fetched") && cachedStart != "" && cachedStart <= start:
		logx.Infof("Precios: usando caché SQLite (descargados hoy, desde %s)", cachedStart)
		return nil
	case !hasData || cachedStart == "":
		logx.Infof("Precios: no hay datos guardados, descarga completa")
		return dl.DownloadPrices(ctx, d, tickers, start, end, s)
	case start < cachedStart:
		logx.Infof("Precios: la caché empieza en %s y se piden desde %s, descarga completa", cachedStart, start)
		return dl.DownloadPrices(ctx, d, tickers, start, end, s)
	}
	return dl.UpdatePrices(ctx, d, tickers, start, end, s)
}

func LoadPrices(d *sql.DB) (*model.Prices, error) {
	p, err := repo.PriceMatrix(d)
	return logLoaded(p, err)
}

func LoadPricesSince(d *sql.DB, since string) (*model.Prices, error) {
	p, err := repo.PriceMatrixSince(d, since)
	return logLoaded(p, err)
}

func logLoaded(p *model.Prices, err error) (*model.Prices, error) {
	if err != nil {
		return nil, err
	}
	var first, last any = nil, nil
	if p.Rows() > 0 {
		first, last = dateutil.Day(p.Dates[0]), dateutil.Day(p.Dates[p.Rows()-1])
	}
	logx.Infof("Precios cargados desde SQLite: shape=(%d, %d) (%v a %v)", p.Rows(), p.Cols(), first, last)
	return p, nil
}
