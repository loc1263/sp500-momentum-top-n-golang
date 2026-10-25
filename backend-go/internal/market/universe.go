package market

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"top3/internal/config"
	"top3/internal/logx"
)

const (
	LargeCapB = 10  // umbral habitual de large-cap (miles de millones de US$)
	MegaCapB  = 200 // umbral habitual de mega-cap (miles de millones de US$)

	FullIndexLabel = "índice completo"
)

// Universe: solo compiten las empresas con capitalización aprox. (precio × acciones actuales) >= el umbral.
type Universe struct {
	Shares         map[string]float64
	MinCapBillions float64
}

// MinCapUSD es el umbral en dólares.
func (u *Universe) MinCapUSD() float64 { return u.MinCapBillions * 1e9 }

// Select son los tickers con acciones conocidas y capitalización >= el umbral, dado el último precio de cada uno
// (prices[i] es el de tickers[i]; NaN si no hay).
func (u *Universe) Select(tickers []string, prices []float64) []string {
	var out []string
	for i, t := range tickers {
		sh, ok := u.Shares[t]
		if !ok || math.IsNaN(prices[i]) {
			continue // sin precio o sin acciones nunca cumple el umbral
		}
		if prices[i]*sh >= u.MinCapUSD() {
			out = append(out, t)
		}
	}
	return out
}

// UniverseBuilder crea el Universe de cada umbral de capitalización reutilizando las acciones en circulación ya bajadas.
type UniverseBuilder struct {
	D           *sql.DB
	DL          *Downloader
	Tickers     []string
	UseCache    bool
	Settings    config.Data
	KnownShares map[string]float64 // se comparte con los contextos del backtest
	shares      map[string]float64
}

// Build devuelve nil para el índice completo (umbral 0).
func (b *UniverseBuilder) Build(ctx context.Context, minCapBillions float64) (*Universe, error) {
	if minCapBillions == 0 {
		return nil, nil
	}
	if b.shares == nil {
		sh, err := b.DL.EnsureShares(ctx, b.D, b.Tickers, b.UseCache, b.Settings, b.KnownShares)
		if err != nil {
			return nil, err
		}
		b.shares = sh
		for t, v := range sh {
			b.KnownShares[t] = v
		}
	}
	logx.Infof("Universo: %s (cap. aprox. >= US$ %g B)", Label(minCapBillions), minCapBillions)
	return &Universe{b.shares, minCapBillions}, nil
}

// ExplicitMinCap es el umbral pedido por línea de comandos (--min-cap-billions, o --large-cap = el de [universe]
// min_cap_billions); ok es false si no se pidió ningún filtro.
func ExplicitMinCap(cfg config.Config, largeCap bool, minCapBillions *float64) (float64, bool) {
	if minCapBillions != nil {
		return *minCapBillions, true
	}
	if largeCap {
		return cfg.Universe.MinCapBillions, true
	}
	return 0, false
}

// Label es el nombre del universo (0 = índice completo).
func Label(minCapB float64) string {
	switch {
	case minCapB == 0:
		return FullIndexLabel
	case minCapB >= MegaCapB:
		return "mega-cap"
	case minCapB >= LargeCapB:
		return "large-cap + mega-cap"
	}
	return fmt.Sprintf("cap. ≥ US$ %g B", minCapB)
}

// Suffix es el sufijo para nombres de archivo.
func Suffix(minCapB float64) string {
	switch {
	case minCapB == 0:
		return ""
	case minCapB >= MegaCapB:
		return "_megacap"
	case minCapB >= LargeCapB:
		return "_largecap"
	}
	return fmt.Sprintf("_cap%gB", minCapB)
}

// Title es el nombre del universo para un título (vacío para el índice completo).
func Title(minCapB float64) string {
	if minCapB == 0 {
		return ""
	}
	return " " + Label(minCapB)
}
