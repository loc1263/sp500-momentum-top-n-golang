package evaluator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/loc1263/gohoo"

	"top3/internal/yahoo"
)

// Trend son las series anuales de los últimos años fiscales, de la más vieja a la más nueva. Los años sin dato son NaN.
type Trend struct {
	Years     []string
	Revenue   []float64
	NetIncome []float64
	FCF       []float64
	OpMargin  []float64 // en %
	Debt      []float64
}

// Raw son los datos de Yahoo de una empresa. Los que faltan son NaN. Los porcentajes están en % (no en fracción) y los montos en la
// moneda de cada dato (Currency para el precio y la capitalización, FinCurrency para los estados financieros).
type Raw struct {
	Ticker, Name, Sector, Industry, Currency, FinCurrency, Rating string

	Price, MarketCap, Beta, W52Low, W52High  float64
	PE, FwdPE, EPS, Dividend                 float64
	RevGrowth, EpsGrowth, ROE                float64
	Target                                   float64
	Revenue, NetIncome, OCF, FCF, Debt, Cash float64 // últimos 12 meses (deuda y caja: último balance)
	CurrentRatio, DebtToEquity               float64
	QRevenue, QOpIncome, QInterest, QEquity  float64 // último trimestre informado
	AsOf                                     string
	Trend                                    Trend
}

// sameCurrency dice si el precio y los estados financieros están en la misma moneda (o si Yahoo no distingue).
func (r Raw) sameCurrency() bool {
	return r.Currency == "" || r.FinCurrency == "" || r.Currency == r.FinCurrency
}

// financialSector es el sector de Yahoo de bancos, aseguradoras y similares.
const financialSector = "Financial Services"

// isBank dice si es un banco o aseguradora: financiera que no informa liquidez corriente. Su deuda es materia prima del negocio y
// no tiene flujo de caja ni liquidez comparables con los de una empresa común. Visa y Mastercard, que sí la informan, no entran.
func (r Raw) isBank() bool { return r.Sector == financialSector && isNaN(r.CurrentRatio) }

// withoutCashAndDebt borra lo que no tiene sentido en un banco: caja, deuda, intereses y flujos, también en la tendencia.
func (r Raw) withoutCashAndDebt() Raw {
	r.OCF, r.FCF, r.Debt, r.Cash, r.DebtToEquity, r.QInterest = nan, nan, nan, nan, nan, nan
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

// timeseriesTypes son las series de fundamentals-timeseries que se piden (una sola request).
var timeseriesTypes = []string{
	"annualTotalRevenue", "annualNetIncome", "annualFreeCashFlow", "annualOperatingIncome", "annualTotalDebt",
	"quarterlyTotalRevenue", "quarterlyOperatingIncome", "quarterlyInterestExpense", "quarterlyStockholdersEquity",
}

// TrendYears son los años fiscales que se muestran en la tendencia.
const TrendYears = 5

// ratings traduce la recomendación de Yahoo al texto que se muestra.
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

// Fetch baja de Yahoo (quoteSummary y fundamentals-timeseries) los datos de una empresa. Si no hay precio ni capitalización, Yahoo
// no la conoce y devuelve un error ErrNotFound. Una falla de fundamentals-timeseries no es grave: la empresa se evalúa sin tendencia.
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

// BuildRaw arma los datos de una empresa con la respuesta de quoteSummary aplanada y la de fundamentals-timeseries (puede ser nil).
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
		QRevenue: nan, QOpIncome: nan, QInterest: nan, QEquity: nan,
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
	// Último trimestre informado: sus ingresos, resultado operativo, intereses y patrimonio deben ser del mismo cierre.
	qrev := gohoo.TimeseriesPoints(ts, "quarterlyTotalRevenue")
	if len(qrev) > 0 {
		r.AsOf, r.QRevenue = qrev[0].Date, qrev[0].Value
		r.QOpIncome = pointOn(ts, "quarterlyOperatingIncome", r.AsOf)
		r.QInterest = math.Abs(pointOn(ts, "quarterlyInterestExpense", r.AsOf))
		r.QEquity = pointOn(ts, "quarterlyStockholdersEquity", r.AsOf)
	}
	r.Trend = buildTrend(ts)
	return r
}

// pointOn es el valor de la serie en la fecha dada (NaN si no hay).
func pointOn(ts map[string]any, typ, date string) float64 {
	if p, ok := gohoo.PointOn(gohoo.TimeseriesPoints(ts, typ), date); ok {
		return p.Value
	}
	return nan
}

// buildTrend alinea las series anuales por fecha de cierre, tomando como base los ingresos; los últimos TrendYears años.
func buildTrend(ts map[string]any) Trend {
	base := gohoo.TimeseriesPoints(ts, "annualTotalRevenue") // la más reciente primero
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
		t.FCF = append(t.FCF, pointOn(ts, "annualFreeCashFlow", d))
		t.OpMargin = append(t.OpMargin, margin)
		t.Debt = append(t.Debt, pointOn(ts, "annualTotalDebt", d))
	}
	return t
}

// fiscalYear es la etiqueta del año fiscal que cierra en la fecha "YYYY-MM-DD": «FY2025».
func fiscalYear(date string) string {
	if len(date) >= 4 {
		return "FY" + date[:4]
	}
	return date
}

func formatFloat(v float64, dec int) string { return strconv.FormatFloat(v, 'f', dec, 64) }
