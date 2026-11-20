// Obtiene y arma los datos crudos de una empresa desde Yahoo (series de tiempo, monedas,
// calificaciones) para el evaluador.
//
// Fecha: 2026-11-19

package evaluator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/loc1263/gohoo"

	"topwx/internal/yahoo"
)

type Trend struct {
	Years       []string
	Revenue     []float64
	NetIncome   []float64
	EPS         []float64
	GrossMargin []float64
	FCF         []float64
	OpMargin    []float64
	Debt        []float64
}

type Raw struct {
	Ticker, Name, Sector, Industry, Currency, FinCurrency, Rating string

	Price, MarketCap, Beta, W52Low, W52High float64
	PE, FwdPE, EPS, Dividend                float64
	RevGrowth, EpsGrowth, ROE               float64
	Target                                  float64
	Revenue, NetIncome, OCF, FCF, Capex     float64
	Debt, Cash                              float64
	CurrentRatio, DebtToEquity              float64
	EBITDA, EBIT, TaxRate, InvestedCapital  float64
	GrossMargin, PB, PEG, Payout            float64
	QRevenue, QOpIncome, QInterest, QEquity float64
	AsOf                                    string
	Trend                                   Trend
}

func (r Raw) sameCurrency() bool {
	return r.Currency == "" || r.FinCurrency == "" || r.Currency == r.FinCurrency
}

const financialSector = "Financial Services"

func (r Raw) isBank() bool { return r.Sector == financialSector && isNaN(r.CurrentRatio) }

func (r Raw) withoutCashAndDebt() Raw {
	r.OCF, r.FCF, r.Capex, r.Debt, r.Cash, r.DebtToEquity, r.QInterest = nan, nan, nan, nan, nan, nan, nan
	r.EBITDA, r.EBIT, r.InvestedCapital = nan, nan, nan
	blank := func(a []float64) []float64 {
		out := make([]float64, len(a))
		for i := range out {
			out[i] = nan
		}
		return out
	}
	r.Trend.FCF, r.Trend.Debt = blank(r.Trend.FCF), blank(r.Trend.Debt)
	return r
}

var timeseriesTypes = []string{
	"annualTotalRevenue", "annualNetIncome", "annualFreeCashFlow", "annualOperatingIncome", "annualTotalDebt",
	"quarterlyTotalRevenue", "quarterlyOperatingIncome", "quarterlyInterestExpense", "quarterlyStockholdersEquity",
	"trailingOperatingCashFlow", "trailingFreeCashFlow", "trailingCapitalExpenditure", "trailingEBITDA", "trailingEBIT",
	"quarterlyInvestedCapital", "quarterlyTaxRateForCalcs", "annualDilutedEPS", "annualGrossProfit",
}

const TrendYears = 5

var ratings = map[string]string{"strong_buy": "Strong Buy", "buy": "Buy", "hold": "Hold", "underperform": "Sell", "sell": "Sell", "strong_sell": "Strong Sell"}

func num(m map[string]any, k string) float64 {
	if v, ok := gohoo.AsFloat(m[k]); ok && !isNaN(v) {
		return v
	}
	return nan
}

func scale(v, by float64) float64 {
	if isNaN(v) {
		return nan
	}
	return v * by
}

func Fetch(ctx context.Context, y *yahoo.Client, ticker string) (Raw, error) {
	info, err := y.Info(ctx, ticker, gohoo.InfoPercentYield())
	if err != nil {
		return Raw{}, err
	}
	ts, _ := y.FundamentalsTimeseriesContext(ctx, ticker, timeseriesTypes)
	r := BuildRaw(ticker, info, ts)
	if isNaN(r.Price) && isNaN(r.MarketCap) {
		return Raw{}, fmt.Errorf("sin datos de precios: %w", gohoo.ErrNotFound)
	}
	return r, nil
}

func BuildRaw(ticker string, info, ts map[string]any) Raw {
	r := Raw{
		Ticker: ticker, Name: gohoo.AsString(info["longName"]), Sector: gohoo.AsString(info["sector"]), Industry: gohoo.AsString(info["industry"]),
		Currency: gohoo.AsString(info["currency"]), FinCurrency: gohoo.AsString(info["financialCurrency"]),
		Rating:    ratings[strings.ToLower(gohoo.AsString(info["recommendationKey"]))],
		Price:     num(info, "regularMarketPrice"),
		MarketCap: num(info, "marketCap"), Beta: num(info, "beta"), W52Low: num(info, "fiftyTwoWeekLow"), W52High: num(info, "fiftyTwoWeekHigh"),
		PE: num(info, "trailingPE"), FwdPE: num(info, "forwardPE"), EPS: num(info, "trailingEps"), Dividend: num(info, "dividendYield"),
		RevGrowth: scale(num(info, "revenueGrowth"), 100), EpsGrowth: scale(num(info, "earningsGrowth"), 100), ROE: scale(num(info, "returnOnEquity"), 100),
		Target:  num(info, "targetMeanPrice"),
		Revenue: num(info, "totalRevenue"), NetIncome: num(info, "netIncomeToCommon"), OCF: num(info, "operatingCashflow"), FCF: num(info, "freeCashflow"),
		Debt: num(info, "totalDebt"), Cash: num(info, "totalCash"), CurrentRatio: num(info, "currentRatio"), DebtToEquity: num(info, "debtToEquity"),
		EBITDA: num(info, "ebitda"), EBIT: nan, TaxRate: nan, InvestedCapital: nan,
		GrossMargin: scale(num(info, "grossMargins"), 100), PB: num(info, "priceToBook"), PEG: num(info, "pegRatio"),
		Payout: scale(num(info, "payoutRatio"), 100),
		Capex:  nan, QRevenue: nan, QOpIncome: nan, QInterest: nan, QEquity: nan,
	}
	if r.GrossMargin == 0 { // Yahoo informa 0 en bancos y aseguradoras, que no tienen costo de ventas
		r.GrossMargin = nan
	}
	if r.Name == "" {
		r.Name = gohoo.AsString(info["shortName"])
	}
	if r.Name == "" {
		r.Name = ticker
	}
	if isNaN(r.Price) {
		r.Price = num(info, "currentPrice")
	}
	if ts == nil {
		return r
	}
	qrev := gohoo.TimeseriesPoints(ts, "quarterlyTotalRevenue")
	if len(qrev) > 0 {
		r.AsOf, r.QRevenue = qrev[0].Date, qrev[0].Value
		r.QOpIncome = pointOn(ts, "quarterlyOperatingIncome", r.AsOf)
		r.QInterest = math.Abs(pointOn(ts, "quarterlyInterestExpense", r.AsOf))
		r.QEquity = pointOn(ts, "quarterlyStockholdersEquity", r.AsOf)
	}
	// info.freeCashflow de Yahoo no cuadra con los estados financieros (MSFT: 3 mil millones contra 60 reales);
	// las series de 12 meses sí, así que mandan y info queda de respaldo.
	if v, ok := latest(ts, "trailingOperatingCashFlow"); ok {
		r.OCF = v
	}
	if v, ok := latest(ts, "trailingFreeCashFlow"); ok {
		r.FCF = v
	}
	if v, ok := latest(ts, "trailingCapitalExpenditure"); ok {
		r.Capex = math.Abs(v)
	}
	for typ, dst := range map[string]*float64{"trailingEBITDA": &r.EBITDA, "trailingEBIT": &r.EBIT,
		"quarterlyInvestedCapital": &r.InvestedCapital, "quarterlyTaxRateForCalcs": &r.TaxRate} {
		if v, ok := latest(ts, typ); ok {
			*dst = v
		}
	}
	r.Trend = buildTrend(ts)
	return r
}

func latest(ts map[string]any, typ string) (float64, bool) {
	pts := gohoo.TimeseriesPoints(ts, typ)
	if len(pts) == 0 || isNaN(pts[0].Value) {
		return nan, false
	}
	return pts[0].Value, true
}

func pointOn(ts map[string]any, typ, date string) float64 {
	if p, ok := gohoo.PointOn(gohoo.TimeseriesPoints(ts, typ), date); ok {
		return p.Value
	}
	return nan
}

func buildTrend(ts map[string]any) Trend {
	base := gohoo.TimeseriesPoints(ts, "annualTotalRevenue")
	if len(base) > TrendYears {
		base = base[:TrendYears]
	}
	var t Trend
	for i := len(base) - 1; i >= 0; i-- {
		d := base[i].Date
		op := pointOn(ts, "annualOperatingIncome", d)
		margin := nan
		if !isNaN(op) && base[i].Value != 0 {
			margin = op / base[i].Value * 100
		}
		t.Years = append(t.Years, fiscalYear(d))
		t.Revenue = append(t.Revenue, base[i].Value)
		t.NetIncome = append(t.NetIncome, pointOn(ts, "annualNetIncome", d))
		t.EPS = append(t.EPS, pointOn(ts, "annualDilutedEPS", d))
		gross := nan
		if gp := pointOn(ts, "annualGrossProfit", d); !isNaN(gp) && base[i].Value != 0 {
			gross = gp / base[i].Value * 100
		}
		t.GrossMargin = append(t.GrossMargin, gross)
		t.FCF = append(t.FCF, pointOn(ts, "annualFreeCashFlow", d))
		t.OpMargin = append(t.OpMargin, margin)
		t.Debt = append(t.Debt, pointOn(ts, "annualTotalDebt", d))
	}
	return t
}

func fiscalYear(date string) string {
	if len(date) >= 4 {
		return "FY" + date[:4]
	}
	return date
}

func formatFloat(v float64, dec int) string { return strconv.FormatFloat(v, 'f', dec, 64) }
