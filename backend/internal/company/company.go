// Package company baja de Yahoo, una sola vez por empresa, los datos fundamentales crudos (Info y series de tiempo de los estados
// financieros) y los guarda en SQLite. El modelo por score y el evaluador leen del mismo caché en lugar de pedir cada uno lo suyo.
//
// Fecha: 2026-11-27
package company

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/dateutil"
	"topwx/internal/db"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/repo"
	"topwx/internal/yahoo"
)

// Snapshot son los datos crudos de una empresa: info (con dividendYield en %) y la respuesta de las series de tiempo.
type Snapshot struct {
	Info, TS map[string]any
}

// Types son las series de tiempo que se piden; es la unión de lo que necesitan el modelo por score y el evaluador.
var Types = []string{
	"annualTotalRevenue", "annualNetIncome", "annualFreeCashFlow", "annualOperatingIncome", "annualTotalDebt",
	"quarterlyTotalRevenue", "quarterlyOperatingIncome", "quarterlyInterestExpense", "quarterlyStockholdersEquity",
	"trailingOperatingCashFlow", "trailingFreeCashFlow", "trailingCapitalExpenditure", "trailingEBITDA", "trailingEBIT",
	"quarterlyInvestedCapital", "quarterlyTaxRateForCalcs", "annualDilutedEPS", "annualGrossProfit",
	// Estado de resultados y flujo de caja anuales (gráficos de flujos de la ficha de empresa)
	"annualCostOfRevenue", "annualResearchAndDevelopment", "annualSellingGeneralAndAdministration", "annualPretaxIncome",
	"annualTaxProvision", "annualTotalOperatingIncomeAsReported", "annualStockBasedCompensation", "annualDepreciationAndAmortization",
	"annualOperatingCashFlow", "annualCapitalExpenditure", "annualRepurchaseOfCapitalStock", "annualCashDividendsPaid",
	"annualNetIssuancePaymentsOfDebt",
}

// version distingue este formato del caché de la tabla fundamentals (que antes guardaba solo los campos del modelo por score).
const version = 4

type stored struct {
	V    int            `json:"v"`
	Info map[string]any `json:"info"`
	TS   map[string]any `json:"ts"`
}

func parse(js string) (Snapshot, bool) {
	var s stored
	if json.Unmarshal([]byte(js), &s) != nil || s.V != version || s.Info == nil {
		return Snapshot{}, false
	}
	return Snapshot{Info: s.Info, TS: s.TS}, true
}

// fetch baja una empresa. Sin Info no hay snapshot; las series de tiempo son opcionales (si fallan, ts queda nil).
// cacheable es false cuando faltan las series: no conviene guardar ese resultado incompleto por varios días.
func fetch(ctx context.Context, y *yahoo.Client, ticker string) (s Snapshot, cacheable bool, err error) {
	info, err := y.Info(ctx, ticker, gohoo.InfoPercentYield())
	if err != nil {
		return Snapshot{}, false, err
	}
	ts, tsErr := y.FundamentalsTimeseriesContext(ctx, ticker, Types)
	return Snapshot{Info: info, TS: ts}, tsErr == nil, nil
}

// Ensure devuelve el snapshot de cada ticker: del caché si tiene menos de ttlDays y useCache, y si no lo baja y lo guarda.
// Los tickers que Yahoo no contesta quedan fuera del mapa.
func Ensure(ctx context.Context, d *sql.DB, y *yahoo.Client, tickers []string, useCache bool, ttlDays, workers int) (map[string]Snapshot, error) {
	cutoff := dateutil.Day(time.Now().AddDate(0, 0, -ttlDays))
	cached, err := repo.FreshFundamentals(d, cutoff)
	if err != nil {
		return nil, err
	}
	out := map[string]Snapshot{}
	var todo []string
	for _, t := range tickers {
		if js, ok := cached[t]; ok && useCache {
			if s, ok := parse(js); ok {
				out[t] = s
				continue
			}
		}
		todo = append(todo, t)
	}
	logx.Infof("Datos financieros: %d ya disponibles, %d a descargar", len(out), len(todo))
	type result struct {
		s         Snapshot
		cacheable bool
	}
	var missing []string
	save := map[string]string{}
	for k, r := range gohoo.ParallelMap(ctx, todo, workers, func(ctx context.Context, t string) (result, error) {
		s, ok, err := fetch(ctx, y, t)
		return result{s, ok}, err
	}) {
		t := todo[k]
		if r.Err != nil {
			logx.Debugf("Sin datos financieros para %s: %v", t, r.Err)
			missing = append(missing, t)
			continue
		}
		out[t] = r.Value.s
		if !r.Value.cacheable {
			continue
		}
		js, err := json.Marshal(stored{V: version, Info: r.Value.s.Info, TS: r.Value.s.TS})
		if err != nil {
			logx.Debugf("No se pudo guardar el caché de %s: %v", t, err)
			continue
		}
		save[t] = string(js)
	}
	if err := repo.SaveFundamentals(d, save, db.Today()); err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		logx.Warnf(errs.DatNoFundamentals.Msg("Sin datos financieros para %d tickers"), len(missing))
		logx.Debugf("Sin datos financieros: %v", missing)
	}
	return out, nil
}

// Sectors devuelve el sector de las empresas que están vigentes en el caché (sin pedir nada a Yahoo). Sirve a quien solo necesita
// ese dato, como el Visor.
func Sectors(d *sql.DB, ttlDays int) map[string]string {
	out := map[string]string{}
	cached, err := repo.FreshFundamentals(d, dateutil.Day(time.Now().AddDate(0, 0, -ttlDays)))
	if err != nil {
		logx.Debugf("Sin caché de sectores: %v", err)
		return out
	}
	for t, js := range cached {
		var s struct {
			V    int `json:"v"`
			Info struct {
				Sector string `json:"sector"`
			} `json:"info"`
		}
		if json.Unmarshal([]byte(js), &s) == nil && s.V == version && s.Info.Sector != "" {
			out[t] = s.Info.Sector
		}
	}
	return out
}
