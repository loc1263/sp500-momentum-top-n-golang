// Pruebas de los gráficos de flujos: estado de resultados y caja de una empresa, con las cifras de Okta (FY2026).
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

package site

import (
	"regexp"
	"strings"
	"testing"

	"topwx/internal/evaluator"
)

func f(v float64) *float64 { return &v }

func okta() evaluator.Statement {
	return evaluator.Statement{
		Label: "FY2026", Date: "2026-01-31", Revenue: f(2919e6), GrossProfit: f(2258e6), SGA: f(1466e6), RnD: f(639e6),
		OperatingIncome: f(149e6), PretaxIncome: f(255e6), NetIncome: f(235e6),
		StockComp: f(544e6), DandA: f(96e6), OCF: f(884e6), Capex: f(21e6), Buybacks: f(73e6), DebtPayment: f(510e6),
	}
}

func fm(v float64) string { return money(&v, "") }

func TestIncomeFlow(t *testing.T) {
	svg, note := incomeFlow(okta(), fm, "Okta")
	if note != "" {
		t.Fatalf("nota inesperada: %s", note)
	}
	wantAll(t, "income", svg, "<svg", "Ingresos", "2,9 B", "Costo de ventas", "661 M", "SG&amp;A", "I+D", "639 M", "Utilidad neta", "235 M", "Otros ingresos", "Impuestos", "20,0 M")
}

func TestIncomeFlowCasosSinDiagrama(t *testing.T) {
	banco := okta()
	banco.GrossProfit = nil
	perdida := okta()
	perdida.OperatingIncome = f(-10e6)
	sinDatos := evaluator.Statement{Label: "FY2026"}
	for name, s := range map[string]evaluator.Statement{"banco": banco, "pérdida": perdida, "sin datos": sinDatos} {
		if svg, note := incomeFlow(s, fm, "x"); svg != "" || note == "" {
			t.Errorf("%s: esperaba nota sin SVG, obtuve svg=%d note=%q", name, len(svg), note)
		}
	}
}

func TestIncomeFlowBeneficioFiscalYEgresos(t *testing.T) {
	s := okta()
	s.NetIncome = f(300e6) // más que antes de impuestos: beneficio fiscal
	if svg, _ := incomeFlow(s, fm, "x"); !strings.Contains(svg, "Beneficio fiscal") {
		t.Error("falta el beneficio fiscal")
	}
	s = okta()
	s.PretaxIncome = f(100e6) // menos que la operativa: egresos no operativos
	if svg, _ := incomeFlow(s, fm, "x"); !strings.Contains(svg, "Otros egresos") {
		t.Error("faltan los egresos no operativos")
	}
}

func TestCashFlow(t *testing.T) {
	svg, note := cashFlow(okta(), fm, "Okta")
	if note != "" {
		t.Fatalf("nota inesperada: %s", note)
	}
	wantAll(t, "cash", svg, "Pago en acciones", "544 M", "Utilidad neta", "Flujo operativo", "884 M", "Flujo libre", "863 M", "CapEx", "Recompras", "73,0 M", "Pago de deuda", "510 M", "Caja retenida", "280 M")
}

func TestCashFlowPagosMayoresAlFlujoLibre(t *testing.T) {
	s := okta()
	s.Buybacks = f(900e6)
	svg, _ := cashFlow(s, fm, "x")
	wantAll(t, "reserva", svg, "Caja acumulada")
	if strings.Contains(svg, "Caja retenida") {
		t.Error("no debería haber caja retenida")
	}
}

func TestCashFlowCapitalDeTrabajoNegativo(t *testing.T) {
	s := okta()
	s.OCF = f(700e6) // el resto de las fuentes suma más que el flujo operativo
	svg, _ := cashFlow(s, fm, "x")
	wantAll(t, "uso", svg, "Capital de trabajo (uso)")
}

func TestCashFlowSinDiagrama(t *testing.T) {
	s := okta()
	s.Capex = f(2e9)
	for name, st := range map[string]evaluator.Statement{"fcf negativo": s, "sin ocf": {Label: "x", NetIncome: f(1)}} {
		if svg, note := cashFlow(st, fm, "x"); svg != "" || note == "" {
			t.Errorf("%s: esperaba nota sin SVG", name)
		}
	}
}

// Las cintas de cada nodo suman lo que el nodo rotula: nada se crea ni se pierde.
func TestSankeyConservaLosFlujos(t *testing.T) {
	g := newSankey("t", 100, fm)
	g.node("a", "A", cBlue, 0, 100)
	g.node("b", "B", cGreen, 1, 60)
	g.node("c", "C", cRed, 1, 40)
	g.link("a", "b", 60)
	g.link("a", "c", 40)
	g.link("a", "zz", 5) // destino inexistente
	g.link("a", "b", -3) // flujo negativo
	if got := sum(g.byID["a"].out); got != 100 {
		t.Errorf("salidas de A = %v, esperaba 100", got)
	}
	svg := g.svg()
	if n := len(regexp.MustCompile(`<path class="flow"`).FindAllString(svg, -1)); n != 2 {
		t.Errorf("cintas = %d, esperaba 2", n)
	}
	if newSankey("t", 1, fm).svg() != "" {
		t.Error("un diagrama vacío no debe dibujar nada")
	}
}

func TestNewFlows(t *testing.T) {
	if newFlows(evaluator.Company{}, "USD") != nil {
		t.Error("sin estados financieros no hay gráficos")
	}
	fv := newFlows(evaluator.Company{Name: "Okta", Statements: []evaluator.Statement{okta()}}, "USD")
	if fv == nil || len(fv.Years) != 1 || fv.Years[0].Income == "" || fv.Years[0].Cash == "" || fv.Currency != "USD" {
		t.Errorf("flows = %+v", fv)
	}
}
