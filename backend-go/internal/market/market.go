// Datos de mercado: actualiza constituyentes, precios y capitalización de forma completa o reciente.
//
// Fecha: 2026-11-19

package market

import (
	"context"
	"database/sql"
	"time"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/logx"
	"top3/internal/model"
)

type Data struct {
	Constituents *Constituents
	Prices       *model.Prices
	HistoryStart string
}

func (dl *Downloader) Refresh(ctx context.Context, d *sql.DB, cfg config.Config, useCache bool, today time.Time) (*Data, error) {
	return dl.refresh(ctx, d, cfg, useCache, today, 0)
}

func (dl *Downloader) RefreshRecent(ctx context.Context, d *sql.DB, cfg config.Config, useCache bool, today time.Time,
	days, minRows int) (*Data, error) {
	data, err := dl.refresh(ctx, d, cfg, useCache, today, days)
	if err != nil {
		return nil, err
	}
	if data.Prices.Rows() < minRows {
		logx.Debugf("La ventana de %d días tiene %d ruedas (< %d): se carga todo el histórico", days, data.Prices.Rows(), minRows)
		if data.Prices, err = LoadPrices(d); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (dl *Downloader) refresh(ctx context.Context, d *sql.DB, cfg config.Config, useCache bool, today time.Time, loadDays int) (*Data, error) {
	historyStart := cfg.Data.HistoryStart(today)
	EnsureConstituents(d, cfg.Data, useCache)
	constituents, err := NewConstituents(d, cfg.Tickers.Aliases)
	if err != nil {
		return nil, err
	}
	tickers := constituents.AllBetween(historyStart, dateutil.Day(today))
	logx.Infof("Tickers únicos que pasaron por el S&P 500 en el histórico: %d", len(tickers))
	priceEnd := dateutil.Day(today.AddDate(0, 0, 1))
	if err := dl.EnsurePrices(ctx, d, tickers, historyStart, priceEnd, useCache, cfg.Data); err != nil {
		return nil, err
	}
	var prices *model.Prices
	if loadDays > 0 {
		prices, err = LoadPricesSince(d, dateutil.Day(today.AddDate(0, 0, -loadDays)))
	} else {
		prices, err = LoadPrices(d)
	}
	if err != nil {
		return nil, err
	}
	return &Data{constituents, prices, historyStart}, nil
}
