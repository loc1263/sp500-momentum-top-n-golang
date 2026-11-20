// Universos del Visor: acciones y ETFs, datos estáticos de cotización y detección de ETFs apalancados.
//
// Fecha: 2026-11-19

package visor

import (
	"context"
	"regexp"
	"strings"

	"github.com/loc1263/gohoo"

	"topwx/internal/yahoo"
)

var usExchanges = map[string]bool{"NMS": true, "NGM": true, "NCM": true, "NYQ": true, "ASE": true, "PCX": true, "BTS": true}

type Static struct {
	Name         string
	MarketCap    *float64
	AUM          *float64
	ExpenseRatio *float64
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
