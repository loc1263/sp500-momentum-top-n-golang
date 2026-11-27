// Descarga y modela los fundamentales de las empresas y los agrupa en métricas de salud.
//
// Fecha: 2026-11-19

package quant

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/loc1263/gohoo"

	"topwx/internal/company"
	"topwx/internal/errs"
	"topwx/internal/logx"
	"topwx/internal/stats"
	"topwx/internal/yahoo"
)

var Fields = []string{"shortName", "sector", "marketCap", "returnOnEquity", "returnOnAssets", "profitMargins", "operatingMargins",
	"debtToEquity", "currentRatio", "quickRatio", "freeCashflow", "operatingCashflow", "totalRevenue",
	"totalDebt", "totalCash", "ebitda", "netIncomeToCommon"}

const (
	FcfYears        = 4
	FinancialSector = "Financial Services"
)

var HealthGroups = []string{"rentabilidad", "solvencia", "liquidez", "caja"}

type metric struct {
	name     string
	higherIs bool
}

var groupMetrics = map[string][]metric{
	"rentabilidad": {{"roe", true}, {"roa", true}, {"operating_margin", true}, {"profit_margin", true}},
	"solvencia":    {{"debt_to_equity", false}, {"net_debt_ebitda", false}, {"cash_to_debt", true}},
	"liquidez":     {{"current_ratio", true}, {"quick_ratio", true}},
	"caja": {{"fcf_margin", true}, {"ocf_margin", true}, {"fcf_yield", true}, {"cash_conversion", true},
		{"ocf_to_debt", true}, {"fcf_positive_years", true}},
}

var notForFinancials = []string{"debt_to_equity", "net_debt_ebitda", "cash_to_debt", "current_ratio", "quick_ratio", "fcf_margin",
	"ocf_margin", "fcf_yield", "cash_conversion", "ocf_to_debt", "fcf_positive_years"}

type Data struct {
	ShortName  string
	Sector     string
	Num        map[string]*float64
	FcfHistory []*float64
}

// dataFrom saca de los datos crudos de una empresa los campos que usa el score de salud. Falla si Yahoo no trae ningún dato de precios.
func dataFrom(s company.Snapshot) (Data, error) {
	d := Data{Num: map[string]*float64{}}
	d.ShortName = gohoo.AsString(s.Info["shortName"])
	d.Sector = gohoo.AsString(s.Info["sector"])
	hasData := false
	for _, k := range Fields[2:] {
		if v, ok := gohoo.AsFloat(s.Info[k]); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
			vv := v
			d.Num[k] = &vv
			if k != "marketCap" {
				hasData = true
			}
		}
	}
	if !hasData {
		return Data{}, fmt.Errorf("sin datos de precios: %w", gohoo.ErrNotFound)
	}
	for _, p := range gohoo.TimeseriesPoints(s.TS, "annualFreeCashFlow") {
		if len(d.FcfHistory) == FcfYears {
			break
		}
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			v := p.Value
			d.FcfHistory = append(d.FcfHistory, &v)
		} else {
			d.FcfHistory = append(d.FcfHistory, nil)
		}
	}
	return d, nil
}

// EnsureFundamentals arma los datos de salud de cada ticker a partir del caché compartido de datos crudos (company).
func EnsureFundamentals(ctx context.Context, d *sql.DB, y *yahoo.Client, tickers []string, useCache bool, ttlDays, workers int) (map[string]Data, error) {
	snaps, err := company.Ensure(ctx, d, y, tickers, useCache, ttlDays, workers)
	if err != nil {
		return nil, err
	}
	data := map[string]Data{}
	var missing []string
	for _, t := range tickers {
		s, ok := snaps[t]
		if !ok {
			continue
		}
		if data[t], err = dataFrom(s); err != nil {
			logx.Debugf("Sin datos financieros para %s: %v", t, err)
			delete(data, t)
			missing = append(missing, t)
		}
	}
	if n := len(tickers) - len(data); n > 0 {
		logx.Warnf(errs.DatNoFundamentals.Msg("Sin datos financieros para %d tickers (quedan fuera del score final)"), n)
		sort.Strings(missing)
		logx.Debugf("Sin datos financieros útiles: %v", missing)
	}
	return data, nil
}

type Row struct {
	Name, Sector string
	MarketCap    float64
	V            map[string]float64
}

func num(d Data, k string) float64 {
	if v := d.Num[k]; v != nil {
		return *v
	}
	return nan
}

func HealthIndicators(tickers []string, data map[string]Data) []Row {
	rows := make([]Row, 0, len(tickers))
	for _, t := range tickers {
		d, ok := data[t]
		if !ok {
			continue
		}
		revenue := num(d, "totalRevenue")
		if !(revenue > 0) {
			revenue = nan
		}
		debt, cash, ebitda := num(d, "totalDebt"), num(d, "totalCash"), num(d, "ebitda")
		marketCap, income, ocf, fcf := num(d, "marketCap"), num(d, "netIncomeToCommon"), num(d, "operatingCashflow"), num(d, "freeCashflow")
		var history []float64
		for _, v := range d.FcfHistory {
			if v != nil {
				history = append(history, *v)
			}
		}
		netDebt := nan
		if !isNaN(debt) && !isNaN(cash) {
			netDebt = debt - cash
		}
		var netDebtEbitda float64
		switch {
		case isNaN(netDebt) || isNaN(ebitda):
			netDebtEbitda = nan
		case netDebt <= 0:
			netDebtEbitda = 0
		case ebitda > 0:
			netDebtEbitda = netDebt / ebitda
		default:
			netDebtEbitda = math.Inf(1)
		}
		d2e := num(d, "debtToEquity")
		if !isNaN(d2e) && !(d2e >= 0) {
			d2e = math.Inf(1)
		}
		cashToDebt := nan
		switch {
		case debt > 0 && !isNaN(cash):
			cashToDebt = cash / debt
		case debt == 0:
			cashToDebt = math.Inf(1)
		}
		fcfYield, cashConversion, ocfToDebt, positive := nan, nan, nan, nan
		if marketCap > 0 {
			fcfYield = fcf / marketCap
		}
		if income > 0 {
			cashConversion = fcf / income
			if !isNaN(cashConversion) {
				cashConversion = math.Min(math.Max(cashConversion, -1), 2)
			}
		}
		switch {
		case debt > 0:
			ocfToDebt = ocf / debt
		case debt == 0 && !isNaN(ocf):
			ocfToDebt = math.Inf(1)
		}
		if len(history) >= 3 {
			n := 0
			for _, v := range history {
				if v > 0 {
					n++
				}
			}
			positive = float64(n) / float64(len(history))
		}
		name := d.ShortName
		if name == "" {
			name = t
		}
		sector := d.Sector
		if sector == "" {
			sector = "n/d"
		}
		r := Row{Name: name, Sector: sector, MarketCap: marketCap, V: map[string]float64{
			"roe": num(d, "returnOnEquity"), "roa": num(d, "returnOnAssets"), "operating_margin": num(d, "operatingMargins"),
			"profit_margin": num(d, "profitMargins"), "debt_to_equity": d2e, "net_debt_ebitda": netDebtEbitda, "cash_to_debt": cashToDebt,
			"current_ratio": num(d, "currentRatio"), "quick_ratio": num(d, "quickRatio"),
			"fcf_margin": fcf / revenue, "ocf_margin": ocf / revenue, "fcf_yield": fcfYield,
			"cash_conversion": cashConversion, "ocf_to_debt": ocfToDebt, "fcf_positive_years": positive,
		}}
		if d.Sector == FinancialSector && isNaN(r.V["current_ratio"]) {
			for _, k := range notForFinancials {
				r.V[k] = nan
			}
		}
		rows = append(rows, r)
	}
	return rows
}

type Health struct {
	Groups  [4]float64
	Measure int
	Health  float64
}

func HealthScore(rows []Row, weights map[string]float64) []Health {
	n := len(rows)
	groups := make([][]float64, len(HealthGroups))
	for g, name := range HealthGroups {
		ranks := make([][]float64, 0, len(groupMetrics[name]))
		for _, m := range groupMetrics[name] {
			col := make([]float64, n)
			for i, r := range rows {
				col[i] = r.V[m.name]
			}
			ranks = append(ranks, stats.RankPct(col, m.higherIs))
		}
		groups[g] = make([]float64, n)
		for i := 0; i < n; i++ {
			vals := make([]float64, len(ranks))
			for k := range ranks {
				vals[k] = ranks[k][i]
			}
			groups[g][i] = stats.MeanSkipNaN(vals)
		}
	}
	w := make([]float64, len(HealthGroups))
	for g, name := range HealthGroups {
		w[g] = 1
		if v, ok := weights[name]; ok {
			w[g] = v
		}
	}
	out := make([]Health, n)
	average := make([]float64, n)
	for i := 0; i < n; i++ {
		num, den, count := 0.0, 0.0, 0
		for g := range HealthGroups {
			v := groups[g][i]
			out[i].Groups[g] = v * 100
			if !isNaN(v) {
				num += v * w[g]
				den += w[g]
				count++
			}
		}
		out[i].Measure = count
		average[i] = nan
		if den != 0 {
			average[i] = num / den
		}
	}
	for i, r := range stats.RankPct(average, true) {
		out[i].Health = r * 100
	}
	return out
}
