// Estado de resultados y flujo de caja anuales de una empresa, base de los gráficos de flujos (Sankey) de su ficha.
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

package evaluator

import (
	"math"

	"github.com/loc1263/gohoo"
)

// StatementYears es la cantidad máxima de ejercicios anuales que se conservan (Yahoo entrega cuatro).
const StatementYears = 4

// Statement reúne las partidas de un ejercicio anual, en la moneda de los estados financieros. Las salidas de caja
// (CapEx, recompras, dividendos, pago de deuda) van en positivo; un dato que Yahoo no informa queda en nil.
type Statement struct {
	Date  string `json:"date"`
	Label string `json:"label"`

	Revenue         *float64 `json:"revenue,omitempty"`
	CostOfRevenue   *float64 `json:"cost_of_revenue,omitempty"`
	GrossProfit     *float64 `json:"gross_profit,omitempty"`
	RnD             *float64 `json:"rnd,omitempty"`
	SGA             *float64 `json:"sga,omitempty"`
	OperatingIncome *float64 `json:"operating_income,omitempty"`
	PretaxIncome    *float64 `json:"pretax_income,omitempty"`
	NetIncome       *float64 `json:"net_income,omitempty"`

	StockComp   *float64 `json:"stock_comp,omitempty"`
	DandA       *float64 `json:"d_and_a,omitempty"`
	OCF         *float64 `json:"ocf,omitempty"`
	Capex       *float64 `json:"capex,omitempty"`
	Buybacks    *float64 `json:"buybacks,omitempty"`
	Dividends   *float64 `json:"dividends,omitempty"`
	DebtPayment *float64 `json:"debt_payment,omitempty"`
}

// buildStatements arma hasta StatementYears ejercicios, del más reciente al más antiguo, con las fechas de ingresos anuales.
func buildStatements(ts map[string]any) []Statement {
	base := gohoo.TimeseriesPoints(ts, "annualTotalRevenue")
	if len(base) > StatementYears {
		base = base[:StatementYears]
	}
	var out []Statement
	for _, b := range base {
		d := b.Date
		at := func(typ string) float64 { return pointOn(ts, typ, d) }
		out = append(out, Statement{
			Date: d, Label: fiscalYear(d),
			Revenue: ptr(b.Value), CostOfRevenue: ptr(abs(at("annualCostOfRevenue"))), GrossProfit: ptr(at("annualGrossProfit")),
			RnD: ptr(abs(at("annualResearchAndDevelopment"))), SGA: ptr(abs(at("annualSellingGeneralAndAdministration"))),
			OperatingIncome: ptr(firstValid(at("annualTotalOperatingIncomeAsReported"), at("annualOperatingIncome"))),
			PretaxIncome:    ptr(at("annualPretaxIncome")), NetIncome: ptr(at("annualNetIncome")),
			StockComp: ptr(at("annualStockBasedCompensation")), DandA: ptr(at("annualDepreciationAndAmortization")),
			OCF: ptr(at("annualOperatingCashFlow")), Capex: ptr(abs(at("annualCapitalExpenditure"))),
			Buybacks: ptr(outflow(at("annualRepurchaseOfCapitalStock"))), Dividends: ptr(outflow(at("annualCashDividendsPaid"))),
			DebtPayment: ptr(outflow(at("annualNetIssuancePaymentsOfDebt"))),
		})
	}
	return out
}

func abs(v float64) float64 {
	if isNaN(v) {
		return nan
	}
	return math.Abs(v)
}

// outflow devuelve la salida de caja (positiva) de una partida que Yahoo informa con signo; un valor positivo es un ingreso de caja.
func outflow(v float64) float64 {
	if isNaN(v) {
		return nan
	}
	return math.Max(-v, 0)
}

func firstValid(vs ...float64) float64 {
	for _, v := range vs {
		if !isNaN(v) {
			return v
		}
	}
	return nan
}
