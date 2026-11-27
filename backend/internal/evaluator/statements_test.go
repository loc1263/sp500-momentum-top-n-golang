// Pruebas de la extracción del estado de resultados y del flujo de caja anuales.
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

package evaluator

import "testing"

func tsFrom(series map[string][][2]any) map[string]any {
	var result []any
	for typ, pts := range series {
		arr := make([]any, len(pts))
		for i, p := range pts {
			arr[i] = map[string]any{"asOfDate": p[0], "reportedValue": map[string]any{"raw": p[1]}}
		}
		result = append(result, map[string]any{"meta": map[string]any{"type": []any{typ}}, typ: arr})
	}
	return map[string]any{"timeseries": map[string]any{"result": result}}
}

func TestBuildStatements(t *testing.T) {
	ts := tsFrom(map[string][][2]any{
		"annualTotalRevenue":                   {{"2025-01-31", 2600.0}, {"2026-01-31", 2919.0}},
		"annualCostOfRevenue":                  {{"2026-01-31", 661.0}},
		"annualGrossProfit":                    {{"2026-01-31", 2258.0}},
		"annualOperatingIncome":                {{"2026-01-31", 153.0}},
		"annualTotalOperatingIncomeAsReported": {{"2026-01-31", 149.0}},
		"annualCapitalExpenditure":             {{"2026-01-31", -21.0}},
		"annualRepurchaseOfCapitalStock":       {{"2026-01-31", -73.0}},
		"annualNetIssuancePaymentsOfDebt":      {{"2026-01-31", 40.0}}, // endeudamiento neto: no es una salida de caja
	})
	got := buildStatements(ts)
	if len(got) != 2 || got[0].Label != "FY2026" || got[1].Label != "FY2025" {
		t.Fatalf("ejercicios: %+v", got)
	}
	s := got[0]
	check := func(name string, p *float64, want float64) {
		t.Helper()
		if p == nil || *p != want {
			t.Errorf("%s = %v, esperaba %v", name, p, want)
		}
	}
	check("revenue", s.Revenue, 2919)
	check("cost", s.CostOfRevenue, 661)
	check("operating (la informada manda)", s.OperatingIncome, 149)
	check("capex en positivo", s.Capex, 21)
	check("buybacks en positivo", s.Buybacks, 73)
	check("deuda neta positiva = sin pago", s.DebtPayment, 0)
	if s.Dividends != nil || s.RnD != nil {
		t.Error("lo que Yahoo no informa debe quedar en nil")
	}
	if got[1].CostOfRevenue != nil {
		t.Error("el ejercicio anterior no tiene costo de ventas")
	}
}

func TestBuildStatementsSinIngresos(t *testing.T) {
	if got := buildStatements(nil); len(got) != 0 {
		t.Errorf("sin series esperaba vacío, obtuve %+v", got)
	}
}

func TestBuildStatementsMaximoDeEjercicios(t *testing.T) {
	var pts [][2]any
	for _, y := range []string{"2020", "2021", "2022", "2023", "2024", "2025"} {
		pts = append(pts, [2]any{y + "-12-31", 1.0})
	}
	if got := buildStatements(tsFrom(map[string][][2]any{"annualTotalRevenue": pts})); len(got) != StatementYears {
		t.Errorf("ejercicios = %d, esperaba %d", len(got), StatementYears)
	}
}
