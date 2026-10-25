// Package etf es el ranking de momentum de los ETFs de Vanguard: retorno ajustado por riesgo, sin backtest.
//
// Para cada ETF y cada plazo de [etf] horizons_months se calcula el retorno del plazo dividido por la volatilidad del plazo
// (anualizada y llevada a la escala del plazo): cuánto subió por cada unidad de riesgo que corrió. El score es el promedio
// de esos cocientes. Es un ranking global de la lista de [etf] csv.
package etf

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"top3/internal/actual"
	"top3/internal/config"
	"top3/internal/console"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/numfmt"
	"top3/internal/repo"
	"top3/internal/stats"
	"top3/internal/yahoo"
)

const (
	TradingDaysYear = 252
	SMADays         = 200 // la tendencia se mide contra la media simple de 200 ruedas
)

// LoadUniverse lee la lista de ETFs (columnas ticker, name, category) sin duplicados.
func LoadUniverse(path string) ([]Fund, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errs.EtfNoList.Errorf("No existe la lista de ETFs: %s", path)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, errs.EtfNoList.Errorf("No se pudo leer %s: %v", path, err)
	}
	if len(records) == 0 {
		return nil, errs.EtfNoList.Errorf("La lista de ETFs está vacía: %s", path)
	}
	header := records[0]
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	col := map[string]int{}
	for _, name := range []string{"ticker", "name", "category"} {
		i := slices.Index(header, name)
		if i < 0 {
			var missing []string
			for _, n := range []string{"category", "name", "ticker"} {
				if !slices.Contains(header, n) {
					missing = append(missing, n)
				}
			}
			return nil, errs.EtfNoList.Errorf("A %s le faltan las columnas: %s", path, strings.Join(missing, ", "))
		}
		col[name] = i
	}
	cell := func(rec []string, name string) string {
		if i := col[name]; i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	seen := map[string]bool{}
	var out []Fund
	for _, rec := range records[1:] {
		t := strings.ToUpper(cell(rec, "ticker"))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, Fund{t, cell(rec, "name"), cell(rec, "category")})
	}
	if len(out) == 0 {
		return nil, errs.EtfNoList.Errorf("La lista de ETFs está vacía: %s", path)
	}
	return out, nil
}

// Score es la fila del ranking de un ETF.
type Score struct {
	Ticker  string
	Rank    int
	Score   float64
	Returns map[int]float64 // retorno de cada plazo, por meses
	Vol     float64         // volatilidad anualizada del plazo más largo
	MaxDD   float64         // caída máxima del plazo más largo
	SMADist float64         // distancia a la media de 200 ruedas
}

// Scores es el ranking de mayor a menor score.
type Scores struct {
	Rows []Score
	// RiskFree es la referencia libre de riesgo realmente usada ("" si no estaba disponible).
	RiskFree string
	// CashLike son los ETFs de tipo efectivo que quedaron fuera por su baja volatilidad.
	CashLike []string
}

// ScoreEtfs arma el ranking por retorno ajustado por riesgo. prices son los precios ajustados (fecha x ticker).
//
// Con riskFree (un ticker de la propia tabla, p. ej. un ETF de bonos ultracortos) el retorno de cada plazo es el exceso sobre
// el de esa referencia: sin eso, un ETF casi sin riesgo siempre ganaría. Si la referencia no está disponible se calcula sin
// ella y Scores.RiskFree queda vacío.
//
// Con minVol (volatilidad anual) los ETFs de tipo efectivo (letras del Tesoro, bonos ultracortos) quedan fuera del ranking:
// con tan poco riesgo cualquier exceso diminuto sobre la referencia da un score enorme. Se listan en Scores.CashLike (la
// referencia libre de riesgo, aunque no rankee, sigue restándose).
//
// Devuelve también {ticker: motivo} de los excluidos: los ETFs con huecos de precios en la ventana o con menos historia que
// el plazo más largo, y los de volatilidad nula.
func ScoreEtfs(prices *model.Prices, horizons []int, tradingDaysPerMonth int, riskFree string, minVol float64) (Scores, map[string]string) {
	hs := append([]int(nil), horizons...)
	sort.Ints(hs)
	longest := hs[len(hs)-1] * tradingDaysPerMonth
	window := prices.Tail(longest + 1)
	excluded := map[string]string{}
	if window.Rows() < longest+1 {
		for _, t := range prices.Tickers {
			excluded[t] = fmt.Sprintf("solo hay %d ruedas, se necesitan %d", window.Rows(), longest+1)
		}
		return Scores{}, excluded
	}
	var keep []string
	for _, t := range window.Tickers {
		j, _ := window.Col(t)
		gap := false
		for i := 0; i < window.Rows(); i++ {
			if math.IsNaN(window.At(i, j)) {
				gap = true
				break
			}
		}
		if gap {
			excluded[t] = "huecos o historia menor al plazo más largo"
		} else {
			keep = append(keep, t)
		}
	}
	window = window.Select(keep)
	if !window.Has(riskFree) {
		riskFree = ""
	}

	type row struct {
		ticker           string
		rets             map[int]float64
		ratios           []float64
		vol, maxDD, smaD float64
	}
	n := window.Rows()
	rows := make([]row, window.Cols())
	for j, t := range window.Tickers {
		series := window.Series(t)
		rows[j] = row{ticker: t, rets: map[int]float64{}}
		rows[j].vol = stats.Std(stats.PctChange(series)) * math.Sqrt(TradingDaysYear)
		peak, dd := series[0], 0.0
		for _, p := range series {
			peak = math.Max(peak, p)
			dd = math.Min(dd, p/peak-1)
		}
		rows[j].maxDD = dd
		tail := series[max(n-SMADays, 0):]
		rows[j].smaD = series[n-1]/stats.Mean(tail) - 1
	}
	for _, h := range hs {
		w := window.Tail(h*tradingDaysPerMonth + 1)
		rets := make([]float64, window.Cols())
		for j, t := range window.Tickers {
			s := w.Series(t)
			rets[j] = s[len(s)-1]/s[0] - 1
			rows[j].rets[h] = rets[j]
		}
		rf := 0.0
		if riskFree != "" {
			j, _ := window.Col(riskFree)
			rf = rets[j]
		}
		for j, t := range window.Tickers {
			vol := stats.Std(stats.PctChange(w.Series(t))) * math.Sqrt(TradingDaysYear)
			scaled := vol * math.Sqrt(float64(h)/12) // la volatilidad anual llevada a la escala del plazo
			ratio := math.NaN()
			if scaled > 0 {
				ratio = (rets[j] - rf) / scaled
			}
			rows[j].ratios = append(rows[j].ratios, ratio)
		}
	}

	var ranked []Score
	var cashLike []string
	for _, r := range rows {
		score := stats.Mean(r.ratios) // NaN si algún plazo no tiene volatilidad
		switch {
		case math.IsNaN(score):
			excluded[r.ticker] = "volatilidad nula"
		case r.vol < minVol:
			cashLike = append(cashLike, r.ticker)
		default:
			ranked = append(ranked, Score{Ticker: r.ticker, Score: score, Returns: r.rets, Vol: r.vol, MaxDD: r.maxDD, SMADist: r.smaD})
		}
	}
	sort.SliceStable(ranked, func(a, b int) bool { return ranked[a].Score > ranked[b].Score })
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	sort.Strings(cashLike)
	return Scores{Rows: ranked, RiskFree: riskFree, CashLike: cashLike}, excluded
}

// TableRows son los encabezados y las filas de texto del ranking (para la consola).
func TableRows(scores Scores, universe []Fund, horizons []int) ([]string, [][]string) {
	hs := append([]int(nil), horizons...)
	sort.Ints(hs)
	info := map[string]Fund{}
	for _, f := range universe {
		info[f.Ticker] = f
	}
	headers := []string{"#", "Ticker", "Nombre", "Categoría", "Score"}
	for _, h := range hs {
		headers = append(headers, fmt.Sprintf("Ret. %dm", h))
	}
	headers = append(headers, "Vol. anual", "Caída máx.", "vs SMA200")
	rows := make([][]string, len(scores.Rows))
	for i, s := range scores.Rows {
		f := info[s.Ticker]
		row := []string{strconv.Itoa(s.Rank), s.Ticker, f.Name, f.Category, fmt.Sprintf("%.2f", s.Score)}
		for _, h := range hs {
			row = append(row, numfmt.Frac(s.Returns[h], true))
		}
		rows[i] = append(row, numfmt.Frac(s.Vol, false), numfmt.Frac(s.MaxDD, true), numfmt.Frac(s.SMADist, true))
	}
	return headers, rows
}

func toRepoRows(scores Scores) []repo.EtfScoreRow {
	rows := make([]repo.EtfScoreRow, len(scores.Rows))
	for i, s := range scores.Rows {
		returns := map[string]float64{}
		for h, v := range s.Returns {
			returns[strconv.Itoa(h)] = v
		}
		rows[i] = repo.EtfScoreRow{Rank: s.Rank, Ticker: s.Ticker, Score: s.Score, Vol: s.Vol, MaxDD: s.MaxDD, SMADist: s.SMADist, Returns: returns}
	}
	return rows
}

// DescribeChanges es la nota con los cambios del top contra el ranking anterior.
func DescribeChanges(previousAsOf string, changes actual.Changes, topN int) string {
	switch {
	case previousAsOf == "":
		return fmt.Sprintf("Cambios del top %d: primera corrida guardada, sin ranking anterior para comparar.", topN)
	case len(changes.Entered) == 0 && len(changes.Exited) == 0:
		return fmt.Sprintf("Cambios del top %d vs. %s: ninguno.", topN, previousAsOf)
	}
	dash := func(l []string) string {
		if len(l) == 0 {
			return "-"
		}
		return strings.Join(l, ", ")
	}
	return fmt.Sprintf("Cambios del top %d vs. %s: entran %s; salen %s.", topN, previousAsOf, dash(changes.Entered), dash(changes.Exited))
}

// RefreshList agrega a la lista ([etf] csv) los ETFs nuevos de Vanguard. Si no se puede, sigue con la lista guardada.
func RefreshList(cfg config.Config) {
	e, data := cfg.Etf, cfg.Data
	etfs, err := FetchEtfs(e.ListURL, data.UserAgent, time.Duration(data.HTTPTimeout)*time.Second)
	var added, gone []string
	if err == nil {
		added, gone, err = UpdateList(e.CSV, etfs)
	}
	if err != nil {
		logx.Warnf(errs.EtfListUpdate.Msg("No se pudo actualizar la lista desde Vanguard: %v"), err)
		return
	}
	if len(added) > 0 {
		logx.Infof("Lista de ETFs: se agregaron %d de Vanguard: %s", len(added), strings.Join(added, ", "))
	} else {
		logx.Infof("Lista de ETFs: al día con Vanguard (%d ETFs)", len(etfs))
	}
	if len(gone) > 0 {
		logx.Infof("Lista de ETFs: Vanguard ya no lista %s (quítalos de %s si dejaron de cotizar)", strings.Join(gone, ", "), e.CSV)
	}
}

// DownloadPrices baja los precios de los ETFs de la lista hasta endDay; avisa de los que Yahoo no devolvió.
func DownloadPrices(ctx context.Context, dl *market.Downloader, cfg config.Config, universe []Fund, horizons []int, endDay time.Time) (*model.Prices, error) {
	longest := horizons[len(horizons)-1]
	start := dateutil.Day(endDay.AddDate(0, 0, -(longest*31 + 60))) // margen para las ruedas necesarias
	tickers := make([]string, len(universe))
	for i, f := range universe {
		tickers[i] = f.Ticker
	}
	prices, err := dl.DownloadRecent(ctx, tickers, start, dateutil.Day(endDay.AddDate(0, 0, 1)), cfg.Data.BatchSize)
	if err != nil {
		return nil, err
	}
	n := sort.Search(prices.Rows(), func(i int) bool { return prices.Dates[i].After(endDay) })
	prices = prices.Slice(0, n).DropAllNaN()
	var without []string
	for _, t := range tickers {
		if !prices.Has(t) {
			without = append(without, t)
		}
	}
	if len(without) > 0 {
		logx.Warnf(errs.EtfNoPrices.Msg("%d ETFs de la lista sin precios en Yahoo: %s"), len(without), strings.Join(without, ", "))
	}
	return prices, nil
}

// Rank arma el ranking de los ETFs con los avisos de lo que quedó fuera; sin ninguno rankeable corta con un error.
func Rank(cfg config.Config, prices *model.Prices, horizons []int) (Scores, map[string]string, error) {
	riskFree := strings.ToUpper(cfg.Etf.RiskFreeTicker)
	scores, excluded := ScoreEtfs(prices, horizons, cfg.Backtest.TradingDaysPerMonth, riskFree, cfg.Etf.MinVol)
	if riskFree != "" && len(scores.Rows) > 0 && scores.RiskFree == "" {
		logx.Warnf(errs.EtfNoRiskFree.Msg("La referencia libre de riesgo %s no está disponible: el score se calcula sin restarla"), riskFree)
	}
	if len(excluded) > 0 {
		logx.Warnf(errs.EtfExcluded.Msg("%d ETFs fuera del ranking (huecos o poca historia): %s"), len(excluded), strings.Join(sortedKeys(excluded), ", "))
	}
	if len(scores.Rows) == 0 {
		return scores, excluded, errs.EtfEmpty.Errorf("Ningún ETF tiene historia suficiente para el plazo más largo.")
	}
	return scores, excluded, nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Present imprime el ranking por consola.
func Present(cfg config.Config, universe []Fund, scores Scores, excluded map[string]string, horizons []int, topN int, dataDay, note string, historical bool) {
	headers, rows := TableRows(scores, universe, horizons)
	excludedNote := ""
	if len(excluded) > 0 {
		parts := make([]string, 0, len(excluded))
		for _, t := range sortedKeys(excluded) {
			parts = append(parts, fmt.Sprintf("%s (%s)", t, excluded[t]))
		}
		excludedNote = "Fuera del ranking: " + strings.Join(parts, ", ")
	}
	cashNote := ""
	if len(scores.CashLike) > 0 {
		cashNote = fmt.Sprintf("Tipo efectivo, fuera del ranking (volatilidad anual < %s%%): %s", numfmt.G(cfg.Etf.MinVol*100), strings.Join(scores.CashLike, ", "))
	}
	console.Etf(dataDay, horizons, topN, len(scores.Rows), headers, rows, []string{note, excludedNote, cashNote}, historical, scores.RiskFree)
}

// Options son los parámetros de Run.
type Options struct {
	AsOf       string // "" = hoy
	TopN       int    // 0 = [etf] top_n
	CSV        string // "" = [etf] csv
	UpdateList bool
}

// Run baja los precios, calcula el ranking, lo guarda en SQLite y lo imprime. Con la lista del config (sin CSV propio) y
// [etf] update_list, antes agrega los ETFs nuevos de Vanguard.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, opt Options) error {
	if opt.UpdateList && opt.CSV == "" && cfg.Etf.UpdateList {
		RefreshList(cfg)
	}
	horizons := append([]int(nil), cfg.Etf.HorizonsMonths...)
	sort.Ints(horizons)
	topN := opt.TopN
	if topN == 0 {
		topN = cfg.Etf.TopN
	}
	csvPath := opt.CSV
	if csvPath == "" {
		csvPath = cfg.Etf.CSV
	}
	universe, err := LoadUniverse(csvPath)
	if err != nil {
		return err
	}
	endDay := dateutil.Today()
	if opt.AsOf != "" {
		if endDay, err = dateutil.Parse(opt.AsOf); err != nil {
			return fmt.Errorf("--as-of debe ser YYYY-MM-DD: %w", err)
		}
	}
	logx.Infof("ETFs de Vanguard: %d en la lista | plazos %v meses | datos hasta %s", len(universe), horizons, dateutil.Day(endDay))

	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	prices, err := DownloadPrices(ctx, dl, cfg, universe, horizons, endDay)
	if err != nil {
		return err
	}
	scores, excluded, err := Rank(cfg, prices, horizons)
	if err != nil {
		return err
	}
	dataDay := dateutil.Day(prices.Dates[prices.Rows()-1])
	note := "Consulta a una fecha pedida: no se guarda ni se compara con otras corridas."
	if opt.AsOf == "" {
		if note, err = saveRanking(cfg, dataDay, scores, topN); err != nil {
			return err
		}
	}
	Present(cfg, universe, scores, excluded, horizons, topN, dataDay, note, opt.AsOf != "")
	return nil
}

// saveRanking guarda el ranking del día y devuelve la nota con los cambios del top contra el ranking anterior.
func saveRanking(cfg config.Config, dataDay string, scores Scores, topN int) (string, error) {
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return "", err
	}
	defer d.Close()
	prevAsOf, prevTop, err := repo.PreviousEtfTop(d, dataDay, topN)
	if err != nil {
		return "", err
	}
	if err := repo.ReplaceEtfScores(d, dataDay, toRepoRows(scores)); err != nil {
		return "", err
	}
	top := make([]string, 0, topN)
	for _, s := range scores.Rows[:min(topN, len(scores.Rows))] {
		top = append(top, s.Ticker)
	}
	return DescribeChanges(prevAsOf, actual.DiffPicks(prevTop, top), topN), nil
}
