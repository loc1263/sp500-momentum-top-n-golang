package market

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/model"
	"top3/internal/repo"
	"top3/internal/yahoo"
)

func sharesFromMap(m map[string]any) float64 {
	for _, k := range []string{"sharesOutstanding", "impliedSharesOutstanding"} {
		if v, ok := gohoo.AsFloat(gohoo.RawValue(m[k])); ok && v > 0 {
			return v
		}
	}
	return 0
}

// FetchShares baja las acciones en circulación actuales de los tickers: primero en lotes por el endpoint de cotizaciones y,
// para los que ese no trae, de a uno por quoteSummary. Los que Yahoo no informa no figuran en el resultado.
func (dl *Downloader) FetchShares(ctx context.Context, tickers []string) map[string]float64 {
	out := map[string]float64{}
	if len(tickers) == 0 {
		return out
	}
	quotes, err := dl.Y.Quotes(ctx, tickers)
	if err != nil {
		logx.Debugf("Cotizaciones en lote incompletas: %v", err)
	}
	for _, q := range quotes {
		if v := sharesFromMap(q.Raw); v > 0 {
			out[q.Symbol] = v
		}
	}
	var rest []string
	for _, t := range tickers {
		if _, ok := out[t]; !ok {
			rest = append(rest, t)
		}
	}
	for k, r := range yahoo.ParallelMap(ctx, rest, dl.Workers, func(ctx context.Context, t string) (float64, error) {
		info, err := dl.Y.Info(ctx, t)
		if err != nil {
			return 0, err
		}
		return sharesFromMap(info), nil
	}) {
		if r.Err != nil {
			logx.Debugf("Sin acciones en circulación para %s: %v", rest[k], r.Err)
			continue
		}
		if r.Value > 0 {
			out[rest[k]] = r.Value
		}
	}
	return out
}

// EnsureShares son las acciones en circulación (actuales) de los tickers. Las guardadas hace menos de shares_ttl_days días
// se reutilizan (cambian por trimestre, no a diario); con --no-cache se vuelven a pedir. known son datos ya obtenidos en
// esta corrida (no se vuelven a pedir, tampoco con --no-cache).
func (dl *Downloader) EnsureShares(ctx context.Context, d *sql.DB, tickers []string, useCache bool, s config.Data,
	known map[string]float64) (map[string]float64, error) {
	cutoff := dateutil.Day(time.Now().AddDate(0, 0, -s.SharesTTLDays))
	cached, err := repo.FreshShares(d, cutoff)
	if err != nil {
		return nil, err
	}
	shares := map[string]float64{}
	noInfo := map[string]bool{} // los que Yahoo ya dijo hoy que no informa: se reintentan mañana, no en cada corrida
	if useCache {
		noInfo = loadNoShares(d)
	}
	var todo, skipped []string
	for _, t := range tickers {
		if v, ok := known[t]; ok {
			shares[t] = v
		} else if v, ok := cached[t]; ok && useCache {
			shares[t] = v
		} else if noInfo[t] {
			skipped = append(skipped, t)
		} else {
			todo = append(todo, t)
		}
	}
	logx.Infof("Acciones en circulación: %d ya disponibles, %d a descargar", len(shares), len(todo))
	fetched := dl.FetchShares(ctx, todo)
	missing := skipped
	for _, t := range todo {
		if v, ok := fetched[t]; ok {
			shares[t] = v
		} else {
			missing = append(missing, t)
		}
	}
	if err := repo.SaveShares(d, fetched, db.Today()); err != nil {
		return nil, err
	}
	if len(missing) > len(skipped) {
		saveNoShares(d, noInfo, missing)
	}
	if len(missing) > 0 {
		logx.Warnf(errs.DatNoShares.Msg("Sin dato de acciones en circulación para %d tickers (capitalización n/d)"), len(missing))
		sort.Strings(missing)
		logx.Debugf("Sin acciones en circulación: %v", missing)
	}
	return shares, nil
}

// noSharesKey es la clave de meta con los tickers sin dato de acciones: "YYYY-MM-DD|T1,T2,...". Solo vale el día en que se
// anotó.
const noSharesKey = "shares_missing"

func loadNoShares(d *sql.DB) map[string]bool {
	out := map[string]bool{}
	v, ok := db.GetMeta(d, noSharesKey)
	day, list, found := strings.Cut(v, "|")
	if !ok || !found || day != db.Today() {
		return out
	}
	for _, t := range strings.Split(list, ",") {
		if t != "" {
			out[t] = true
		}
	}
	return out
}

func saveNoShares(d *sql.DB, before map[string]bool, missing []string) {
	set := map[string]bool{}
	for t := range before {
		set[t] = true
	}
	for _, t := range missing {
		set[t] = true
	}
	list := make([]string, 0, len(set))
	for t := range set {
		list = append(list, t)
	}
	sort.Strings(list)
	if err := db.SetMeta(d, noSharesKey, db.Today()+"|"+strings.Join(list, ",")); err != nil {
		logx.Debugf("No se pudo anotar los tickers sin acciones en circulación: %v", err)
	}
}

// AddMarketCaps agrega a cada rebalanceo la capitalización aprox. de sus selecciones: precio de esa fecha × acciones actuales.
func AddMarketCaps(histories [][]model.Rebalance, prices *model.Prices, shares map[string]float64) {
	for _, history := range histories {
		for i := range history {
			h := &history[i]
			day, err := dateutil.Parse(h.Date)
			if err != nil {
				continue
			}
			row := sort.Search(prices.Rows(), func(k int) bool { return prices.Dates[k].After(day) }) - 1
			caps := map[string]float64{}
			for _, p := range h.Picks {
				j, ok := prices.Col(p.Ticker)
				if !ok || row < 0 {
					continue
				}
				if px := prices.LastValid(j, row); px == px { // no NaN
					if sh, ok := shares[p.Ticker]; ok {
						caps[p.Ticker] = px * sh
					}
				}
			}
			h.Caps = caps
		}
	}
}
