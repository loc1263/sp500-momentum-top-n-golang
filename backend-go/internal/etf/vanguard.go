package etf

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"top3/internal/logx"
	"top3/internal/market"
)

// MinEtfs: una respuesta con menos ETFs que esto no es la lista completa y no se toca el archivo.
const MinEtfs = 20

// Fund es un ETF de la lista: ticker, nombre y categoría.
type Fund struct{ Ticker, Name, Category string }

// FetchEtfs baja la lista de ETFs del servicio que alimenta la página de productos de Vanguard. Falla si la respuesta no
// tiene la forma esperada o trae demasiado pocos ETFs.
func FetchEtfs(url, userAgent string, timeout time.Duration) ([]Fund, error) {
	text, err := market.GetText(url, userAgent, timeout)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("la respuesta de Vanguard no es JSON: %w", err)
	}
	return ParseEtfs(data)
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// ParseEtfs lee los ETFs abiertos de la respuesta de Vanguard.
func ParseEtfs(data map[string]any) ([]Fund, error) {
	fund, _ := data["fund"].(map[string]any)
	entities, ok := fund["entity"].([]any)
	if !ok {
		return nil, fmt.Errorf("la respuesta de Vanguard no tiene la forma esperada (falta fund.entity)")
	}
	var etfs []Fund
	for _, e := range entities {
		entity, _ := e.(map[string]any)
		profile, ok := entity["profile"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("la respuesta de Vanguard no tiene la forma esperada (falta profile)")
		}
		isETF, _ := profile["isETF"].(bool)
		fact, _ := profile["fundFact"].(map[string]any)
		closed, _ := fact["isClosed"].(bool)
		ticker := strings.ToUpper(strings.TrimSpace(str(profile, "ticker")))
		if !isETF || ticker == "" || closed {
			continue
		}
		etfs = append(etfs, Fund{ticker, ShortName(str(profile, "longName")), Category(profile)})
	}
	if len(etfs) < MinEtfs {
		return nil, fmt.Errorf("solo %d ETFs en la respuesta de Vanguard (se esperaban al menos %d)", len(etfs), MinEtfs)
	}
	return etfs, nil
}

var (
	vanguardPrefix = regexp.MustCompile(`^Vanguard\s+`)
	etfSuffix      = regexp.MustCompile(`\s+ETF( Shares)?$`)
	bondIntl       = regexp.MustCompile(`International|Emerging|World|Global`)
)

// ShortName: 'Vanguard Total Stock Market ETF' -> 'Total Stock Market'.
func ShortName(longName string) string {
	return strings.TrimSpace(etfSuffix.ReplaceAllString(vanguardPrefix.ReplaceAllString(strings.TrimSpace(longName), ""), ""))
}

// Category es la categoría (las del archivo) para un ETF nuevo, según el estilo que informa Vanguard.
func Category(profile map[string]any) string {
	name, style, kind := str(profile, "longName"), str(profile, "style"), str(profile, "type")
	switch {
	case style == "Bond Funds":
		if bondIntl.MatchString(name) {
			return "Bonos internacionales"
		}
		return "Bonos EE. UU."
	case kind == "International/Global Stock":
		return "Acciones internacionales"
	case kind == "Domestic Stock - Sector-Specific":
		return "Sectores"
	}
	return "Acciones EE. UU. amplio"
}

func quote(field string) string {
	if strings.ContainsAny(field, ",\"\r\n") {
		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return field
}

// UpdateList agrega al CSV los ETFs que no están: nunca edita ni borra filas. Devuelve (tickers agregados, tickers del CSV
// que Vanguard ya no lista). El archivo se reemplaza de una vez: nunca queda a medio escribir.
func UpdateList(path string, etfs []Fund) (added, gone []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(raw)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	if len(records) > 0 {
		col := slices.Index(records[0], "ticker")
		if col < 0 {
			return nil, nil, fmt.Errorf("a %s le falta la columna ticker", path)
		}
		for _, rec := range records[1:] {
			if col < len(rec) {
				known[strings.ToUpper(strings.TrimSpace(rec[col]))] = true
			}
		}
	}
	theirs := map[string]bool{}
	var fresh []Fund
	for _, e := range etfs {
		theirs[e.Ticker] = true
		if !known[e.Ticker] {
			fresh = append(fresh, e)
		}
	}
	sort.Slice(fresh, func(a, b int) bool { return fresh[a].Ticker < fresh[b].Ticker })
	for t := range known {
		if !theirs[t] {
			gone = append(gone, t)
		}
	}
	sort.Strings(gone)
	if len(fresh) == 0 {
		return nil, gone, nil
	}
	body := text
	if !strings.HasSuffix(text, "\n") && !strings.HasSuffix(text, "\r") {
		body += newline
	}
	var extra strings.Builder
	for _, e := range fresh {
		extra.WriteString(quote(e.Ticker) + "," + quote(e.Name) + "," + quote(e.Category) + newline)
		added = append(added, e.Ticker)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "*.tmp")
	if err != nil {
		return nil, nil, err
	}
	if _, err := tmp.WriteString(body + extra.String()); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return nil, nil, err
	}
	logx.Debugf("Lista de ETFs: %d agregados a %s", len(added), path)
	return added, gone, nil
}
