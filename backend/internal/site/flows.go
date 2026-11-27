// Gráficos de flujos de la ficha de empresa: de dónde viene y a dónde va cada dólar de ingresos (estado de resultados) y de caja
// operativa, un par de diagramas por ejercicio anual.
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

package site

import (
	"html/template"
	"math"

	"topwx/internal/evaluator"
)

type flowsView struct {
	Currency string
	Years    []flowYear
}

type flowYear struct {
	Label, Date          string
	Income, Cash         template.HTML
	IncomeNote, CashNote string
}

func newFlows(c evaluator.Company, fin string) *flowsView {
	if len(c.Statements) == 0 {
		return nil
	}
	fv := &flowsView{Currency: fin}
	moneyFn := func(v float64) string { return money(&v, "") }
	for _, s := range c.Statements {
		y := flowYear{Label: s.Label, Date: s.Date}
		svg, note := incomeFlow(s, moneyFn, c.Name+" · "+s.Label+": ingresos")
		y.Income, y.IncomeNote = template.HTML(svg), note
		svg, note = cashFlow(s, moneyFn, c.Name+" · "+s.Label+": caja")
		y.Cash, y.CashNote = template.HTML(svg), note
		fv.Years = append(fv.Years, y)
	}
	return fv
}

func val(p *float64) (float64, bool) {
	if p == nil || math.IsNaN(*p) {
		return 0, false
	}
	return *p, true
}

const (
	cBlue, cOrange, cTeal, cAmber, cPink, cGreen = "var(--s1)", "var(--s2)", "var(--s3)", "var(--s4)", "var(--s5)", "var(--s6)"
	cRed, cGrey                                  = "var(--neg)", "var(--muted)"
)

// incomeFlow arma el diagrama del estado de resultados: ingresos, costo de ventas, utilidad bruta, gastos operativos, impuestos y
// utilidad neta. Devuelve el SVG o, si el ejercicio no se puede dibujar, el motivo.
func incomeFlow(s evaluator.Statement, money func(float64) string, title string) (string, string) {
	rev, ok := val(s.Revenue)
	gp, okGP := val(s.GrossProfit)
	op, okOp := val(s.OperatingIncome)
	pretax, okPre := val(s.PretaxIncome)
	ni, okNI := val(s.NetIncome)
	switch {
	case ok && rev > 0 && !okGP:
		return "", "Sin costo de ventas: no aplica a bancos, aseguradoras y otras empresas que no lo informan."
	case !ok || rev <= 0 || !okOp || !okPre || !okNI:
		return "", "Faltan datos del estado de resultados de este ejercicio."
	case gp <= 0 || gp > rev || op <= 0 || op > gp || pretax <= 0 || ni <= 0:
		return "", "Ese ejercicio terminó con pérdida (operativa, antes de impuestos o neta): el diagrama solo muestra utilidades."
	}
	opex := gp - op
	other := pretax - op
	tax := pretax - ni
	sga, _ := val(s.SGA)
	rnd, _ := val(s.RnD)
	if sum := sga + rnd; sum > opex && sum > 0 {
		sga, rnd = sga*opex/sum, rnd*opex/sum
	}
	rest := opex - sga - rnd
	if rest < rev*0.01 && rest > 0 {
		if sga >= rnd {
			sga += rest
		} else {
			rnd += rest
		}
		rest = 0
	}

	g := newSankey(title, rev, money)
	g.node("rev", "Ingresos", cBlue, 0, rev)
	if other > 0 {
		g.node("other", "Otros ingresos", cBlue, 0, other)
	}
	g.node("gp", "Utilidad bruta", cGreen, 1, gp)
	g.node("opex", "Gastos operativos", cOrange, 2, opex)
	g.node("op", "Utilidad operativa", cGreen, 2, op)
	g.node("pre", "Antes de impuestos", cGreen, 3, pretax)
	if tax < 0 {
		g.node("taxben", "Beneficio fiscal", cTeal, 3, -tax)
	}
	g.node("cogs", "Costo de ventas", cRed, 4, rev-gp)
	g.node("sga", "SG&A", cAmber, 4, sga)
	g.node("rnd", "I+D", cPink, 4, rnd)
	g.node("rest", "Otros gastos", cGrey, 4, rest)
	if other < 0 {
		g.node("oexp", "Otros egresos", cGrey, 4, -other)
	}
	if tax > 0 {
		g.node("tax", "Impuestos", cRed, 4, tax)
	}
	g.node("ni", "Utilidad neta", cTeal, 4, ni)

	g.link("rev", "gp", gp)
	g.link("rev", "cogs", rev-gp)
	g.link("gp", "opex", opex)
	g.link("gp", "op", op)
	g.link("opex", "sga", sga)
	g.link("opex", "rnd", rnd)
	g.link("opex", "rest", rest)
	g.link("op", "pre", math.Min(op, pretax))
	g.link("other", "pre", math.Max(other, 0))
	g.link("op", "oexp", math.Max(-other, 0))
	g.link("pre", "tax", math.Max(tax, 0))
	g.link("pre", "ni", pretax-math.Max(tax, 0))
	g.link("taxben", "ni", math.Max(-tax, 0))
	return g.svg(), ""
}

// cashFlow arma el diagrama de caja: de la utilidad neta y los ajustes sin caja al flujo operativo, de ahí a CapEx y flujo libre, y
// del flujo libre a recompras, dividendos, pago de deuda y caja retenida.
func cashFlow(s evaluator.Statement, money func(float64) string, title string) (string, string) {
	ocf, okO := val(s.OCF)
	ni, okN := val(s.NetIncome)
	if !okO || !okN || ocf <= 0 {
		return "", "Faltan datos del flujo de caja de este ejercicio o el flujo operativo fue negativo."
	}
	sbc, _ := val(s.StockComp)
	da, _ := val(s.DandA)
	capex, _ := val(s.Capex)
	buy, _ := val(s.Buybacks)
	div, _ := val(s.Dividends)
	debt, _ := val(s.DebtPayment)
	fcf := ocf - capex
	if fcf <= 0 {
		return "", "El flujo de caja libre de ese ejercicio fue negativo: el diagrama solo muestra flujos positivos."
	}

	type part struct {
		id, label, color string
		v                float64
	}
	parts := []part{
		{"sbc", "Pago en acciones", cPink, sbc}, {"ni", "Utilidad neta", cTeal, ni},
		{"da", "Depreciación", cBlue, da}, {"wc", "Capital de trabajo", cGrey, ocf - ni - sbc - da},
	}
	g := newSankey(title, ocf, money)
	var pos, neg []part
	var posSum, negSum float64
	for _, p := range parts {
		switch {
		case p.v > sankeyEps:
			pos, posSum = append(pos, p), posSum+p.v
			g.node(p.id, p.label, p.color, 0, p.v)
		case p.v < -sankeyEps:
			neg, negSum = append(neg, p), negSum-p.v
		}
	}
	g.node("ocf", "Flujo operativo", cGreen, 1, ocf)
	for _, p := range neg {
		g.node("neg-"+p.id, p.label+" (uso)", cGrey, 1, -p.v)
	}
	g.node("fcf", "Flujo libre", cGreen, 2, fcf)
	retained := fcf - buy - div - debt
	if retained < 0 {
		g.node("reserve", "Caja acumulada", cGrey, 2, -retained)
	}
	g.node("capex", "CapEx", cOrange, 3, capex)
	g.node("buy", "Recompras", cPink, 3, buy)
	g.node("div", "Dividendos", cAmber, 3, div)
	g.node("debt", "Pago de deuda", cRed, 3, debt)
	if retained > 0 {
		g.node("ret", "Caja retenida", cTeal, 3, retained)
	}

	for _, p := range pos {
		g.link(p.id, "ocf", p.v*ocf/posSum)
		for _, n := range neg {
			g.link(p.id, "neg-"+n.id, p.v*(-n.v)/posSum)
		}
	}
	g.link("ocf", "capex", capex)
	g.link("ocf", "fcf", fcf)
	payouts := buy + div + debt
	if retained >= 0 {
		g.link("fcf", "buy", buy)
		g.link("fcf", "div", div)
		g.link("fcf", "debt", debt)
		g.link("fcf", "ret", retained)
	} else {
		for _, o := range []struct {
			id string
			v  float64
		}{{"buy", buy}, {"div", div}, {"debt", debt}} {
			g.link("fcf", o.id, o.v*fcf/payouts)
			g.link("reserve", o.id, o.v*(-retained)/payouts)
		}
	}
	return g.svg(), ""
}
