// Descarga y asegura los datos OHLCV de los tickers.
//
// Fecha: 2026-11-19

package quant

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/market"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
)

const HistoryDays = market.OHLCVDays

// EnsureOHLCV asegura el OHLCV de los tickers. Normalmente ya quedó guardado hoy por la descarga de precios (una sola petición por
// ticker da el cierre ajustado y el OHLCV), incluso con --no-cache, porque esa descarga corrió en esta misma corrida. Solo si no
// quedó al día (p. ej. base creada antes de este cambio) lo baja aparte.
func EnsureOHLCV(ctx context.Context, dl *market.Downloader, d *sql.DB, tickers []string, batchSize int) error {
	if db.FetchedToday(d, market.OHLCVFetchedKey) {
		logx.Infof("OHLCV: ya descargado hoy junto con los precios")
		return nil
	}
	today := dateutil.Today()
	start, end := today.AddDate(0, 0, -HistoryDays), today.AddDate(0, 0, 1)
	logx.Infof("Descargando OHLCV de %d tickers (%s a %s) en lotes de %d...", len(tickers), dateutil.Day(start), dateutil.Day(end), batchSize)
	var rows []repo.OHLCVRow
	var missing []string
	nBatches := (len(tickers) + batchSize - 1) / batchSize
	for i := 0; i < len(tickers); i += batchSize {
		batch := tickers[i:min(i+batchSize, len(tickers))]
		out := gohoo.ParallelMap(ctx, batch, dl.Workers, func(ctx context.Context, t string) ([]repo.OHLCVRow, error) {
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
		got := 0
		for k, r := range out {
			switch {
			case r.Err == nil && len(r.Value) > 0:
				got++
				rows = append(rows, r.Value...)
			case r.Err != nil && yahoo.Classify(r.Err) == yahoo.KindRateLimit:
				logx.Errorf(errs.NetRateLimit.Msg("Yahoo siguió limitando a %s tras %d reintentos; queda sin datos"), batch[k], yahoo.RateLimitRetries)
				missing = append(missing, batch[k])
			case r.Err != nil && yahoo.Classify(r.Err) == yahoo.KindOther:
				logx.Warnf(errs.NetYahoo.Msg("%s: %v"), batch[k], r.Err)
				missing = append(missing, batch[k])
			default:
				logx.Debugf("%s: sin OHLCV (%v)", batch[k], r.Err)
				missing = append(missing, batch[k])
			}
		}
		logx.Infof("Lote %d/%d: %d tickers con datos", i/batchSize+1, nBatches, got)
	}
	if len(rows) == 0 {
		logx.Warnf("%s", errs.DatNoOHLCV.Msg("No se pudo descargar ningún OHLCV; se usa lo guardado"))
		return nil
	}
	if err := repo.ReplaceOHLCV(d, rows); err != nil {
		return err
	}
	if err := db.SetMeta(d, market.OHLCVFetchedKey, time.Now().Format("2006-01-02")); err != nil {
		return err
	}
	if len(missing) > 0 {
		logx.Warnf(errs.DatNoOHLCV.Msg("Sin OHLCV para %d tickers (quedan fuera del score técnico)"), len(missing))
		sort.Strings(missing)
		logx.Debugf("Sin OHLCV: %v", missing)
	}
	logx.Infof("OHLCV guardado: %d filas", len(rows))
	return nil
}
