// Package market baja y guarda los datos de mercado que necesitan el backtest, el Top actual y el modelo por score:
// composición Point-in-Time del S&P 500, precios ajustados, acciones en circulación y universos por capitalización.
package market

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"top3/internal/config"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/repo"
)

// ParseTickers lee la lista de tickers del texto del CSV ("['A', 'BRK.B']"); los puntos pasan a guión (formato Yahoo).
func ParseTickers(s string) []string {
	r := strings.NewReplacer("[", "", "]", "", "'", "")
	var out []string
	for _, t := range strings.Split(r.Replace(s), ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, strings.ReplaceAll(t, ".", "-"))
		}
	}
	return out
}

// GetText baja una URL como texto, con el user agent y el timeout de la configuración.
func GetText(url, userAgent string, timeout time.Duration) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	logx.Debugf("GET %s -> HTTP %d, %d bytes", url, resp.StatusCode, len(body))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d en %s", resp.StatusCode, url)
	}
	return string(body), nil
}

func seconds(n int) time.Duration { return time.Duration(n) * time.Second }

var dateLayouts = []string{"2006-01-02", "2006/01/02", "01/02/2006", "1/2/2006", "2006-01-02 15:04:05", "January 2, 2006"}

func parseAnyDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// DownloadHistorical baja el CSV con la composición histórica del índice y lo guarda.
func DownloadHistorical(d *sql.DB, s config.Data) error {
	logx.Infof("Descargando listas históricas del S&P 500...")
	text, err := GetText(s.HistoricalURL, s.UserAgent, seconds(s.HistoricalTimeout))
	if err != nil {
		return err
	}
	r := csv.NewReader(strings.NewReader(text))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	if len(records) < 2 {
		return fmt.Errorf("el CSV histórico está vacío")
	}
	header := make([]string, len(records[0]))
	for i, c := range records[0] {
		header[i] = strings.ToLower(strings.TrimSpace(c))
	}
	logx.Debugf("CSV histórico: %d filas, columnas=%v", len(records)-1, header)
	dateCol := slices.IndexFunc(header, func(c string) bool { return strings.Contains(c, "date") })
	if dateCol < 0 {
		return fmt.Errorf("el CSV histórico no tiene columna de fecha")
	}
	tickCol := slices.IndexFunc(header, func(c string) bool {
		return strings.Contains(c, "ticker") || strings.Contains(c, "symbol") || strings.Contains(c, "constituent")
	})
	if tickCol < 0 {
		for i := range header {
			if i != dateCol {
				tickCol = i
				break
			}
		}
	}
	logx.Debugf("Columnas usadas: fecha=%s, tickers=%s", header[dateCol], header[tickCol])

	var rows []repo.HistoryRow
	for _, rec := range records[1:] {
		if len(rec) <= max(dateCol, tickCol) {
			continue
		}
		day, ok := parseAnyDate(rec[dateCol])
		if !ok {
			continue
		}
		rows = append(rows, repo.HistoryRow{Date: day, Tickers: strings.Join(ParseTickers(rec[tickCol]), ",")})
	}
	if len(rows) == 0 {
		return fmt.Errorf("el CSV histórico no tiene filas con fecha")
	}
	if err := repo.ReplaceHistory(d, rows); err != nil {
		return err
	}
	if err := db.SetMeta(d, "constituents_history_fetched", db.Today()); err != nil {
		return err
	}
	logx.Infof("Listas históricas guardadas: %s a %s (%d filas)", rows[0].Date, rows[len(rows)-1].Date, len(rows))
	return nil
}

// DownloadCurrent baja la lista actual del índice (Wikipedia) y la guarda.
func DownloadCurrent(d *sql.DB, s config.Data) error {
	logx.Infof("Obteniendo lista actual del S&P 500 (Wikipedia)...")
	text, err := GetText(s.WikipediaURL, s.UserAgent, seconds(s.HTTPTimeout))
	if err != nil {
		return err
	}
	tickers, err := parseCurrent(text)
	if err != nil {
		return err
	}
	if err := repo.ReplaceCurrent(d, tickers); err != nil {
		return err
	}
	if err := db.SetMeta(d, "constituents_current_fetched", db.Today()); err != nil {
		return err
	}
	logx.Infof("Lista actual guardada: %d tickers", len(tickers))
	return nil
}

func parseCurrent(page string) ([]string, error) {
	tables, err := ParseTables(strings.NewReader(page))
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if len(t.Rows) == 0 {
			continue
		}
		col := slices.Index(t.Rows[0].Cells, "Symbol")
		if col < 0 {
			continue
		}
		var out []string
		for _, r := range t.Rows[1:] {
			if col < len(r.Cells) && r.Cells[col] != "" && !r.Header {
				out = append(out, strings.ReplaceAll(r.Cells[col], ".", "-"))
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, fmt.Errorf("no se encontró la tabla con la columna Symbol en Wikipedia")
}

// DownloadIndexChanges baja las altas/bajas del índice según Wikipedia; completan lo que el CSV histórico no alcanza.
func DownloadIndexChanges(d *sql.DB, s config.Data) error {
	logx.Infof("Descargando cambios del índice (Wikipedia)...")
	text, err := GetText(s.WikipediaChangesURL, s.UserAgent, seconds(s.HTTPTimeout))
	if err != nil {
		return err
	}
	rows, bad, err := parseChanges(text)
	if err != nil {
		return err
	}
	if bad > 0 {
		logx.Warnf(errs.DatBadDates.Msg("%d filas con fecha no interpretable, se ignoran"), bad)
	}
	if err := repo.ReplaceChanges(d, rows); err != nil {
		return err
	}
	if err := db.SetMeta(d, "index_changes_fetched", db.Today()); err != nil {
		return err
	}
	lo, hi := rows[0].Date, rows[0].Date
	for _, r := range rows {
		lo, hi = min(lo, r.Date), max(hi, r.Date)
	}
	logx.Infof("Cambios del índice guardados: %d filas (%s a %s)", len(rows), lo, hi)
	return nil
}

func parseChanges(page string) (rows []repo.IndexChange, bad int, err error) {
	tables, err := ParseTables(strings.NewReader(page))
	if err != nil {
		return nil, 0, err
	}
	for _, t := range tables {
		if len(t.Rows) < 2 || !t.Rows[0].Header || !slices.Contains(t.Rows[0].Cells, "Added") {
			continue
		}
		norm := func(cells []string, i int) *string {
			if i >= len(cells) || strings.TrimSpace(cells[i]) == "" {
				return nil
			}
			v := strings.ReplaceAll(strings.TrimSpace(cells[i]), ".", "-")
			return &v
		}
		for _, r := range t.Rows[1:] {
			if r.Header || len(r.Cells) < 5 {
				continue
			}
			day, ok := parseAnyDate(r.Cells[0])
			if !ok {
				bad++
				continue
			}
			var reason *string
			if len(r.Cells) > 5 && strings.TrimSpace(r.Cells[5]) != "" {
				v := strings.TrimSpace(r.Cells[5])
				reason = &v
			}
			rows = append(rows, repo.IndexChange{Date: day, Added: norm(r.Cells, 1), Removed: norm(r.Cells, 3), Reason: reason})
		}
		if len(rows) == 0 {
			return nil, bad, errs.NetWikiFormat.Errorf("No se encontró la tabla de cambios en Wikipedia (¿cambió el formato de la página?)")
		}
		return rows, bad, nil
	}
	return nil, 0, errs.NetWikiFormat.Errorf("No se encontró la tabla de cambios en Wikipedia (¿cambió el formato de la página?)")
}

// EnsureConstituents descarga (o reutiliza del mismo día) el histórico (CSV), los cambios posteriores y la lista actual
// (Wikipedia). Si una descarga falla, queda lo que haya en SQLite.
func EnsureConstituents(d *sql.DB, s config.Data, useCache bool) {
	steps := []struct {
		key      string
		download func(*sql.DB, config.Data) error
		label    string
	}{
		{"constituents_history_fetched", DownloadHistorical, "histórica (CSV)"},
		{"index_changes_fetched", DownloadIndexChanges, "cambios del índice (Wikipedia)"},
		{"constituents_current_fetched", DownloadCurrent, "actual (Wikipedia)"},
	}
	for _, st := range steps {
		if useCache && db.FetchedToday(d, st.key) {
			logx.Infof("Composición %s: usando caché SQLite (descargada hoy)", st.label)
			continue
		}
		if err := st.download(d, s); err != nil {
			logx.Errorf(errs.NetComposition.Msg("Error descargando composición %s: %v"), st.label, err)
			logx.Warnf(errs.NetFallback.Msg("Se usará lo que haya en SQLite para la composición %s"), st.label)
		}
	}
}

// Constituents es la composición Point-in-Time del S&P 500 leída desde SQLite.
type Constituents struct {
	aliases map[string]string
	dates   []string
	lists   [][]string
	Current []string
}

func (c *Constituents) alias(t string) string {
	if a, ok := c.aliases[t]; ok {
		return a
	}
	return t
}

// canon aplica los alias y elimina duplicados (manteniendo el orden).
func (c *Constituents) canon(tickers []string) []string {
	seen := make(map[string]bool, len(tickers))
	out := make([]string, 0, len(tickers))
	for _, t := range tickers {
		t = c.alias(t)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// NewConstituents lee la composición de SQLite. aliases: símbolo viejo -> símbolo actual en Yahoo (renombres).
func NewConstituents(d *sql.DB, aliases map[string]string) (*Constituents, error) {
	c := &Constituents{aliases: aliases}
	rows, err := repo.History(d)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		c.dates = append(c.dates, r.Date)
		var list []string
		if r.Tickers != "" {
			list = c.canon(strings.Split(r.Tickers, ","))
		}
		c.lists = append(c.lists, list)
	}
	cur, err := repo.Current(d)
	if err != nil {
		return nil, err
	}
	c.Current = c.canon(cur)
	if err := c.applyChanges(d); err != nil {
		return nil, err
	}
	first, last := "None", "None"
	if len(c.dates) > 0 {
		first, last = c.dates[0], c.dates[len(c.dates)-1]
	}
	logx.Debugf("Constituents: %d filas históricas (%s a %s), %d actuales", len(c.dates), first, last, len(c.Current))
	return c, nil
}

// NewConstituentsFrom arma la composición con listas ya cargadas (para tests): dates ordenadas y sus listas.
func NewConstituentsFrom(dates []string, lists [][]string, current []string) *Constituents {
	return &Constituents{dates: dates, lists: lists, Current: current}
}

// applyChanges extiende el CSV con las altas/bajas de Wikipedia posteriores a su última fecha.
func (c *Constituents) applyChanges(d *sql.DB) error {
	if len(c.dates) == 0 {
		return nil
	}
	csvLast := c.dates[len(c.dates)-1]
	changes, err := repo.ChangesAfter(d, csvLast)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		logx.Infof("Composición: el CSV llega a %s y no hay cambios posteriores en Wikipedia", csvLast)
		return nil
	}
	members := map[string]bool{}
	for _, t := range c.lists[len(c.lists)-1] {
		members[t] = true
	}
	byDate := map[string][][2]string{}
	for _, ch := range changes {
		byDate[ch.Date] = append(byDate[ch.Date], [2]string{c.alias(ch.Added), c.alias(ch.Removed)})
	}
	days := make([]string, 0, len(byDate))
	for day := range byDate {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		for _, pair := range byDate[day] {
			added, removed := pair[0], pair[1]
			if removed != "" {
				if members[removed] {
					delete(members, removed)
				} else {
					logx.Debugf("%s: baja %s no estaba en la lista", day, removed)
				}
			}
			if added != "" {
				members[added] = true
			}
		}
		c.dates = append(c.dates, day)
		c.lists = append(c.lists, sortedMembers(members))
	}
	logx.Infof("Composición: CSV hasta %s + %d cambios de Wikipedia hasta %s (%d tickers)",
		csvLast, len(changes), c.dates[len(c.dates)-1], len(members))
	if len(c.Current) > 0 {
		cur := map[string]bool{}
		for _, t := range c.Current {
			cur[t] = true
		}
		var missing, extra []string
		for t := range cur {
			if !members[t] {
				missing = append(missing, t)
			}
		}
		for t := range members {
			if !cur[t] {
				extra = append(extra, t)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		logx.Infof("Verificación vs. lista actual de Wikipedia: faltan %d, sobran %d", len(missing), len(extra))
		logx.Debugf("Faltan: %v | Sobran: %v", missing, extra)
	}
	return nil
}

func sortedMembers(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for t := range m {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// OnDate son los tickers vigentes en target (YYYY-MM-DD). Más allá del último dato histórico usa la lista actual.
func (c *Constituents) OnDate(target string) []string {
	if len(c.dates) == 0 {
		logx.Debugf("%s: sin histórico, uso lista actual", target)
		return c.Current
	}
	last := c.dates[len(c.dates)-1]
	if target > last && len(c.Current) > 0 {
		logx.Debugf("%s: posterior al último histórico (%s), uso lista actual", target, last)
		return c.Current
	}
	i := sort.SearchStrings(c.dates, target+"\x00") - 1 // bisect_right
	if i < 0 {
		logx.Warnf(errs.DatNoHistory.Msg("Sin datos históricos <= %s"), target)
		return nil
	}
	logx.Debugf("%s -> fila histórica %s (%d tickers)", target, c.dates[i], len(c.lists[i]))
	return c.lists[i]
}

// AllBetween es la unión de todos los tickers que estuvieron en el índice entre start y end.
func (c *Constituents) AllBetween(start, end string) []string {
	set := map[string]bool{}
	if len(c.dates) > 0 {
		i0 := max(sort.SearchStrings(c.dates, start+"\x00")-1, 0) // fila vigente al inicio
		i1 := sort.SearchStrings(c.dates, end+"\x00")
		for _, lst := range c.lists[i0:i1] {
			for _, t := range lst {
				set[t] = true
			}
		}
	}
	for _, t := range c.Current {
		set[t] = true
	}
	return sortedMembers(set)
}
