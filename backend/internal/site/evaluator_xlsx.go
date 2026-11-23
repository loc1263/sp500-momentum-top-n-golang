// Excel del listado del evaluador: una fila por empresa con sus datos generales, el veredicto de cada categoría y el valor de cada métrica.
// Cada valor lleva el color de su nivel (verde, amarillo o rojo); los números van como números, con su formato, para filtrarlos y ordenarlos.
//
// Fecha: 2026-11-20

package site

import (
	"topwx/internal/evaluator"
	"topwx/internal/xlsx"
)

const xlsxFile = "evaluador.xlsx"

// xlsxFormats es el formato de número de cada unidad de métrica. Los porcentajes ya vienen en puntos (18,2 = 18,2 %), por eso el «%» es texto.
var xlsxFormats = map[string]string{"pct": `0.0"%"`, "x": `0.00"×"`, "years": `0.0" años"`, "money": "#,##0", "num": "0.00"}

var xlsxHeaderUnits = map[string]string{"pct": "%", "x": "×", "years": "años", "money": "moneda de reporte"}

func levelTone(l evaluator.Level) xlsx.Tone {
	switch l {
	case evaluator.Good:
		return xlsx.Good
	case evaluator.Warn:
		return xlsx.Warn
	case evaluator.Bad:
		return xlsx.Bad
	}
	return xlsx.None
}

type metricColumn struct{ section, id string }

// Columnas cuyo encabezado va en verde claro, para encontrarlas rápido: el veredicto de dos categorías y cuatro métricas.
var (
	markedVerdicts = map[string]bool{"prof": true, "grow": true}
	markedMetrics  = map[metricColumn]bool{{"prof", "net_margin"}: true, {"cash", "fcf_margin"}: true, {"val", "sma200"}: true, {"val", "ret_3m"}: true}
)

// metricColumns lista las métricas por categoría en el orden de las secciones y, dentro de ellas, en el de primera aparición.
// La comparación con competidores no tiene métricas propias: repite las de otras categorías.
func metricColumns(p evaluator.Payload) []metricColumn {
	var cols []metricColumn
	for _, s := range p.Sections {
		seen := map[string]bool{}
		for _, c := range p.Companies {
			for _, k := range c.Categories {
				if k.ID != s.ID {
					continue
				}
				for _, m := range k.Metrics {
					if !seen[m.ID] {
						seen[m.ID] = true
						cols = append(cols, metricColumn{s.ID, m.ID})
					}
				}
			}
		}
	}
	return cols
}

func numOrBlank(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func yesNoCell(b, green bool) xlsx.Cell {
	if !b {
		return xlsx.Cell{Value: "No"}
	}
	c := xlsx.Cell{Value: "Sí"}
	if green {
		c.Tone = xlsx.Good
	}
	return c
}

func headerCell(s string, marked bool) xlsx.Cell {
	if marked {
		return xlsx.Cell{Value: s, Tone: xlsx.HeadGreen}
	}
	return xlsx.Cell{Value: s, Tone: xlsx.Head}
}

func verdictCell(l evaluator.Level, text string) xlsx.Cell {
	return xlsx.Cell{Value: text, Tone: levelTone(l)}
}

// evaluatorXLSX arma el libro con todas las empresas del payload.
func evaluatorXLSX(p evaluator.Payload) ([]byte, error) {
	title := map[string]string{}
	labels := map[string][3]string{}
	for _, s := range p.Sections {
		title[s.ID], labels[s.ID] = s.Title, s.Labels
	}
	cols := metricColumns(p)

	head := []string{"Ticker", "Empresa", "Mercado", "Sector", "Industria", "Moneda", "Moneda de reporte", "Precio", "Capitalización",
		"Datos al", "Banco", "Veredicto", "Sólida", "Barata"}
	widths := []float64{9, 32, 10, 22, 30, 9, 11, 11, 18, 12, 8, 12, 9, 9}
	marked := make([]bool, len(head))
	for _, s := range p.Sections {
		head = append(head, "Veredicto · "+s.Title)
		widths = append(widths, 13)
		marked = append(marked, markedVerdicts[s.ID])
	}
	for _, c := range cols {
		def := p.Metrics[c.id]
		name := def.Name
		if name == "" {
			name = c.id
		}
		if u := xlsxHeaderUnits[def.Unit]; u != "" {
			name += " (" + u + ")"
		}
		head = append(head, title[c.section]+" · "+name)
		widths = append(widths, 16)
		marked = append(marked, markedMetrics[c])
	}
	header := make([]xlsx.Cell, len(head))
	for i, h := range head {
		header[i] = headerCell(h, marked[i])
	}
	rows := [][]xlsx.Cell{header}

	for _, c := range p.Companies {
		solid, cheap := evaluator.Screen(c)
		row := []xlsx.Cell{{Value: c.Ticker}, {Value: c.Name}, {Value: c.Universe}, {Value: c.Sector}, {Value: c.Industry}, {Value: c.Currency},
			{Value: c.FinCurrency}, {Value: numOrBlank(c.Price), Format: "0.00"}, {Value: numOrBlank(c.MarketCap), Format: "#,##0"},
			{Value: c.AsOf}, yesNoCell(c.Bank, false), verdictCell(c.Verdict, verdictText(c.Verdict)), yesNoCell(solid, true), yesNoCell(cheap, true)}
		verdicts := map[string]evaluator.Level{}
		metrics := map[metricColumn]evaluator.Metric{}
		for _, k := range c.Categories {
			verdicts[k.ID] = k.Verdict
			for _, m := range k.Metrics {
				metrics[metricColumn{k.ID, m.ID}] = m
			}
		}
		for _, s := range p.Sections {
			v, ok := verdicts[s.ID]
			if !ok {
				v = evaluator.NA
			}
			row = append(row, verdictCell(v, pillText(v, "", labels[s.ID])))
		}
		for _, col := range cols {
			m, ok := metrics[col]
			if !ok {
				row = append(row, xlsx.Cell{})
				continue
			}
			cell := xlsx.Cell{Value: numOrBlank(m.Value), Format: xlsxFormats[p.Metrics[col.id].Unit], Tone: levelTone(m.Level)}
			if m.Text != "" {
				cell.Value, cell.Format = m.Text, ""
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}
	return xlsx.Write(xlsx.Sheet{Name: "Empresas", Widths: widths, Rows: rows, FreezeCols: 2})
}
