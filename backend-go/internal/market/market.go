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

// Data son los datos de mercado que necesitan el backtest y el modelo por score: composición del índice y precios al día.
type Data struct {
	Constituents *Constituents
	Prices       *model.Prices // fecha x ticker, todos los que pasaron por el índice en el histórico
	HistoryStart string
}

// Refresh deja en SQLite la composición del S&P 500 y los precios de todos sus miembros del histórico (descargando lo que
// falte) y los carga.
func (dl *Downloader) Refresh(ctx context.Context, d *sql.DB, cfg config.Config, useCache bool, today time.Time) (*Data, error) {
	return dl.refresh(ctx, d, cfg, useCache, today, 0)
}

// RefreshRecent es Refresh para quien solo mira el final de la serie (por ejemplo, el último año): descarga y actualiza
// igual, pero carga en memoria solo los últimos days días corridos y minRows ruedas como mínimo (si la ventana no las tiene,
// carga todo). Leer el histórico de 12 años desde SQLite es lo más lento de un comando que no lo usa.
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

// refresh hace el trabajo de Refresh; con loadDays > 0 solo carga esa cantidad de días corridos hasta hoy.
func (dl *Downloader) refresh(ctx context.Context, d *sql.DB, cfg config.Config, useCache bool, today time.Time, loadDays int) (*Data, error) {
	historyStart := cfg.Data.HistoryStart(today)
	EnsureConstituents(d, cfg.Data, useCache)
	constituents, err := NewConstituents(d, cfg.Tickers.Aliases)
	if err != nil {
		return nil, err
	}
	tickers := constituents.AllBetween(historyStart, dateutil.Day(today))
	logx.Infof("Tickers únicos que pasaron por el S&P 500 en el histórico: %d", len(tickers))
	priceEnd := dateutil.Day(today.AddDate(0, 0, 1)) // Yahoo: el fin es exclusivo
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
