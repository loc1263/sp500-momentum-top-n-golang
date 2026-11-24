// Universos del Visor: acciones y ETFs, datos estáticos de cotización y detección de ETFs apalancados.
//
// Fecha: 2026-11-19

package visor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/yahoo"
)

var usExchanges = map[string]bool{"NMS": true, "NGM": true, "NCM": true, "NYQ": true, "ASE": true, "PCX": true, "BTS": true}

type Static struct {
	Name         string
	MarketCap    *float64
	AUM          *float64
	ExpenseRatio *float64
	// Founded es el año de fundación de la empresa (0 si no se pudo leer).
	Founded int
}

func StaticOf(symbol, longName, shortName string, marketCap float64, raw map[string]any) Static {
	s := Static{Name: longName}
	if s.Name == "" {
		s.Name = shortName
	}
	if s.Name == "" {
		s.Name = symbol
	}
	if marketCap > 0 {
		s.MarketCap = ptr(marketCap)
	}
	if v, ok := gohoo.AsFloat(raw["netAssets"]); ok && v > 0 {
		s.AUM = ptr(v)
	}
	if v, ok := gohoo.AsFloat(raw["netExpenseRatio"]); ok && v >= 0 {
		s.ExpenseRatio = ptr(v)
	}
	return s
}

func screen(ctx context.Context, y *yahoo.Client, q gohoo.Query, quoteType, sortField string, limit int) (map[string]Static, []string, error) {
	statics := map[string]Static{}
	var order []string
	for offset := 0; limit <= 0 || len(order) < limit; {
		res, err := y.Screen(ctx, gohoo.ScreenParams{
			Query: q, QuoteType: quoteType, Offset: offset, Size: gohoo.ScreenMaxSize, SortField: sortField,
		})
		if err != nil {
			return nil, nil, err
		}
		for _, r := range res.Quotes {
			if !usExchanges[r.Exchange] || r.Symbol == "" {
				continue
			}
			if _, dup := statics[r.Symbol]; dup {
				continue
			}
			statics[r.Symbol] = StaticOf(r.Symbol, r.LongName, r.ShortName, r.MarketCap, r.Raw)
			order = append(order, r.Symbol)
		}
		offset += len(res.Quotes)
		if len(res.Quotes) == 0 || offset >= res.Total {
			break
		}
	}
	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}
	return statics, order, nil
}

func StockUniverse(ctx context.Context, y *yahoo.Client, capMin float64) (map[string]Static, []string, error) {
	q := gohoo.And(gohoo.Eq("region", "us"), gohoo.Gt("intradaymarketcap", capMin))
	return screen(ctx, y, q, "EQUITY", "intradaymarketcap", 0)
}

// VeteranUniverse son las acciones de EE. UU. con capitalización mayor a capMin fundadas hace al menos years años al año de now.
// Yahoo no tiene un campo de fundación: el año sale del resumen del negocio (FoundedYear). Como pedir el Info de miles de acciones
// topa con el límite de Yahoo y el año no cambia, se guarda en cachePath y cada corrida solo consulta las acciones que faltan; las
// que Yahoo no contesta quedan fuera hoy y se reintentan en la próxima corrida. Las cuyo resumen no dice el año también quedan fuera.
func VeteranUniverse(ctx context.Context, y *yahoo.Client, capMin float64, years, workers int, cachePath string, now time.Time) (map[string]Static, []string, error) {
	statics, order, err := StockUniverse(ctx, y, capMin)
	if err != nil {
		return nil, nil, err
	}
	cache := loadFoundedCache(cachePath)
	var todo []string
	for _, t := range order {
		if _, ok := cache[t]; !ok {
			todo = append(todo, t)
		}
	}
	if len(todo) > 0 {
		logx.Infof("Visor: leyendo el año de fundación de %d acciones (%d ya estaban en el caché)...", len(todo), len(order)-len(todo))
		res := gohoo.ParallelMap(ctx, todo, workers, func(ctx context.Context, t string) (int, error) {
			info, err := y.Info(ctx, t)
			if err != nil {
				return 0, err
			}
			return FoundedYear(gohoo.AsString(info["longBusinessSummary"])), nil
		})
		failed := 0
		for i, r := range res {
			if r.Err != nil {
				failed++
				continue
			}
			cache[todo[i]] = r.Value
		}
		if failed > 0 {
			logx.Warnf(errs.VisNoData.Msg("Más de 50 años: Yahoo no contestó el año de fundación de %d de %d acciones; se reintenta en la próxima corrida"), failed, len(todo))
		}
		if err := saveFoundedCache(cachePath, cache); err != nil {
			logx.Warnf("Visor: no se pudo guardar el caché de años de fundación en %s: %v", cachePath, err)
		}
	}
	for _, t := range order {
		st := statics[t]
		st.Founded = cache[t]
		statics[t] = st
	}
	statics, order = filterVeterans(statics, order, years, now)
	return statics, order, nil
}

func loadFoundedCache(path string) map[string]int {
	cache := map[string]int{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &cache); err != nil {
			logx.Warnf("Visor: el caché de años de fundación %s no se pudo leer (%v); se rehace", path, err)
			cache = map[string]int{}
		}
	}
	return cache
}

func saveFoundedCache(path string, cache map[string]int) error {
	raw, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func filterVeterans(statics map[string]Static, order []string, years int, now time.Time) (map[string]Static, []string) {
	limit := now.Year() - years
	kept := map[string]Static{}
	var out []string
	for _, t := range order {
		if preferredRe.MatchString(t) {
			continue
		}
		if f := statics[t].Founded; f > 0 && f <= limit {
			kept[t] = statics[t]
			out = append(out, t)
		}
	}
	return kept, out
}

// preferredRe reconoce los tickers de acciones preferentes (WFC-PC, PCG-PI): no son la empresa común y repiten su historia.
var preferredRe = regexp.MustCompile(`-P[A-Z]?$`)

var (
	foundedRe = regexp.MustCompile(`\b(founded|established)\b[^.]{0,40}?\b(1[6-9]\d\d|20\d\d)\b`)
	incorpRe  = regexp.MustCompile(`\b(incorporated|organized|formed)\b[^.]{0,40}?\b(1[6-9]\d\d|20\d\d)\b`)
)

// FoundedYear saca el año de fundación del resumen del negocio de Yahoo («founded in 1886», «incorporated in 1911»...). Prefiere
// «founded» / «established» sobre «incorporated», y devuelve 0 si no encuentra ninguno. Los verbos se buscan en minúscula para no
// confundirlos con «Incorporated» del nombre de la empresa.
func FoundedYear(summary string) int {
	for _, re := range []*regexp.Regexp{foundedRe, incorpRe} {
		if m := re.FindStringSubmatch(summary); m != nil {
			y, _ := strconv.Atoi(m[2])
			return y
		}
	}
	return 0
}

func ETFUniverse(ctx context.Context, y *yahoo.Client, top int, aumMin float64) (map[string]Static, []string, error) {
	q := gohoo.And(gohoo.Eq("region", "us"), gohoo.Gt("fundnetassets", aumMin))
	return screen(ctx, y, q, "ETF", "fundnetassets", top)
}

func QuoteStatics(ctx context.Context, y *yahoo.Client, symbols []string) (map[string]Static, error) {
	qs, err := y.Quotes(ctx, symbols)
	out := map[string]Static{}
	for _, q := range qs {
		out[q.Symbol] = StaticOf(q.Symbol, q.LongName, q.ShortName, q.MarketCap, q.Raw)
	}
	return out, err
}

var (
	leverPlain = regexp.MustCompile(`\b[1-9](\.\d+)?x\b|leveraged|inverse|\bbear\b|\bdaily (2|3)x\b`)
	leverUltra = regexp.MustCompile(`\bultra(pro|short)?\b`)
	leverShort = regexp.MustCompile(`\bshort\b`)
	shortSafe  = []string{"term", "duration", "maturity", "bond", "income", "treasury", "govern"}
)

func IsLeveraged(name, category string) bool {
	text := strings.ToLower(name + " " + category)
	if leverPlain.MatchString(text) {
		return true
	}
	for _, m := range leverUltra.FindAllStringIndex(text, -1) {
		rest := text[m[1]:]
		if !shortAfter(rest) && !strings.HasPrefix(rest, " bond") {
			return true
		}
	}
	for _, m := range leverShort.FindAllStringIndex(text, -1) {
		if !followedBy(text[m[1]:], shortSafe) {
			return true
		}
	}
	return false
}

func shortAfter(rest string) bool {
	return followedBy(rest, []string{"short"})
}

func followedBy(rest string, words []string) bool {
	if len(rest) < 2 || (rest[0] != '-' && rest[0] != ' ') {
		return false
	}
	for _, w := range words {
		if strings.HasPrefix(rest[1:], w) {
			return true
		}
	}
	return false
}
