// Package ipsa aplica el modelo por score a las 30 acciones del IPSA (Bolsa de Santiago), en pesos chilenos. La lista sale de la
// tabla de Wikipedia y se guarda en un archivo que se renueva una vez al mes; los precios y datos financieros vienen de Yahoo Finance.
package ipsa

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"top3/internal/config"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
)

// Stock es una acción del IPSA: nemotécnico de la Bolsa de Santiago y nombre de la empresa.
type Stock struct {
	Nemo string `json:"nemo"`
	Name string `json:"name"`
}

// Universe es la lista de acciones guardada, con la fecha y la fuente de donde salió.
type Universe struct {
	UpdatedAt string  `json:"updated_at"`
	Source    string  `json:"source"`
	Stocks    []Stock `json:"stocks"`
}

var nemo = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)

// ParseUniverse lee la tabla de Wikipedia (columnas Nemotécnico y Empresa) y devuelve las acciones en su orden.
func ParseUniverse(page string) ([]Stock, error) {
	tables, err := market.ParseTables(strings.NewReader(page))
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		for h, head := range t.Rows {
			col, name := -1, -1
			for i, c := range head.Cells {
				switch {
				case strings.HasPrefix(c, "Nemot"):
					col = i
				case c == "Empresa":
					name = i
				}
			}
			if col < 0 || name < 0 {
				continue
			}
			var out []Stock
			seen := map[string]bool{}
			for _, r := range t.Rows[h+1:] {
				if r.Header || col >= len(r.Cells) || name >= len(r.Cells) {
					continue
				}
				n := strings.ToUpper(strings.TrimSpace(r.Cells[col]))
				if !nemo.MatchString(n) || seen[n] {
					continue
				}
				seen[n] = true
				out = append(out, Stock{Nemo: n, Name: r.Cells[name]})
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	return nil, errors.New("no se encontró la tabla con las columnas Nemotécnico y Empresa")
}

// YahooTicker es el símbolo de Yahoo Finance de una acción: el alias si lo hay y el sufijo de la Bolsa de Santiago.
func YahooTicker(s Stock, cfg config.Ipsa) string {
	n := s.Nemo
	if a, ok := cfg.Aliases[n]; ok && a != "" {
		n = a
	}
	return n + cfg.Suffix
}

func readUniverse(path string) (*Universe, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var u Universe
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &u, nil
}

func writeUniverse(path string, u *Universe) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// EnsureUniverse devuelve la lista de acciones: la guardada si tiene menos de universe_ttl_days días (y useCache), o la que baja
// de Wikipedia y guarda. Si Wikipedia falla, usa la guardada aunque esté vencida; sin ninguna de las dos es un error.
func EnsureUniverse(cfg config.Config, useCache bool, now time.Time) (*Universe, error) {
	c := cfg.Ipsa
	stored, readErr := readUniverse(c.UniverseFile)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		logx.Warnf("No se pudo leer %s: %v", c.UniverseFile, readErr)
	}
	if readErr == nil && len(stored.Stocks) > 0 && useCache {
		if t, err := time.Parse("2006-01-02", stored.UpdatedAt); err == nil && now.Sub(t) < time.Duration(c.UniverseTTLDays)*24*time.Hour {
			logx.Infof("Lista del IPSA: %d acciones, actualizada el %s (%s)", len(stored.Stocks), stored.UpdatedAt, c.UniverseFile)
			return stored, nil
		}
	}
	logx.Infof("Obteniendo la lista del IPSA (Wikipedia)...")
	stocks, err := fetchUniverse(cfg)
	if err != nil {
		if readErr == nil && len(stored.Stocks) > 0 {
			logx.Warnf(errs.IpsNoUniverse.Msg("%v; se usa la lista guardada del %s"), err, stored.UpdatedAt)
			return stored, nil
		}
		return nil, errs.IpsNoUniverse.Errorf("%v", err)
	}
	u := &Universe{UpdatedAt: now.Format("2006-01-02"), Source: c.WikipediaURL, Stocks: stocks}
	if err := writeUniverse(c.UniverseFile, u); err != nil {
		logx.Warnf("No se pudo guardar %s: %v", c.UniverseFile, err)
	}
	logx.Infof("Lista del IPSA: %d acciones (guardada en %s)", len(stocks), c.UniverseFile)
	return u, nil
}

func fetchUniverse(cfg config.Config) ([]Stock, error) {
	page, err := market.GetText(cfg.Ipsa.WikipediaURL, cfg.Data.UserAgent, time.Duration(cfg.Data.HTTPTimeout)*time.Second)
	if err != nil {
		return nil, err
	}
	return ParseUniverse(page)
}
