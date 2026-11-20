// Define los universos de acciones (por capitalización mínima) con sus etiquetas y títulos.
//
// Fecha: 2026-11-19

package market

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"topwx/internal/config"
	"topwx/internal/logx"
)

const (
	LargeCapB = 10
	MegaCapB  = 200

	FullIndexLabel = "índice completo"
)

type Universe struct {
	Shares         map[string]float64
	MinCapBillions float64
}

func (u *Universe) MinCapUSD() float64 { return u.MinCapBillions * 1e9 }

func (u *Universe) Select(tickers []string, prices []float64) []string {
	var out []string
	for i, t := range tickers {
		sh, ok := u.Shares[t]
		if !ok || math.IsNaN(prices[i]) {
			continue
		}
		if prices[i]*sh >= u.MinCapUSD() {
			out = append(out, t)
		}
	}
	return out
}

type UniverseBuilder struct {
	D           *sql.DB
	DL          *Downloader
	Tickers     []string
	UseCache    bool
	Settings    config.Data
	KnownShares map[string]float64
	shares      map[string]float64
}

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

func ExplicitMinCap(cfg config.Config, largeCap bool, minCapBillions *float64) (float64, bool) {
	if minCapBillions != nil {
		return *minCapBillions, true
	}
	if largeCap {
		return cfg.Universe.MinCapBillions, true
	}
	return 0, false
}

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

func Title(minCapB float64) string {
	if minCapB == 0 {
		return ""
	}
	return " " + Label(minCapB)
}
