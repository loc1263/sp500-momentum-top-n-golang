// Formato del evaluador de empresas: montos, métricas, distintivos y minigráfico.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"topwx/internal/evaluator"
)

var verdictLabel = map[evaluator.Level]string{evaluator.Good: "Bien", evaluator.Warn: "Atención", evaluator.Bad: "Alerta", evaluator.NA: "Sin datos"}

var levelOrder = map[evaluator.Level]int{evaluator.Good: 0, evaluator.Warn: 1, evaluator.Bad: 2, evaluator.NA: 3}

func verdictText(l evaluator.Level) string {
	if t, ok := verdictLabel[l]; ok {
		return t
	}
	return string(l)
}

func money(p *float64, currency string) string {
	if p == nil {
		return nd
	}
	v := *p
	a := math.Abs(v)
	var body string
	switch {
	case a >= 1e9:
		body = esNum(a/1e9, boolInt(a < 1e11)) + " B"
	case a >= 1e6:
		body = esNum(a/1e6, boolInt(a < 1e8)) + " M"
	default:
		body = esNum(a, 0)
	}
	if v < 0 {
		body = "-" + body
	}
	if currency != "" {
		body += " " + currency
	}
	return body
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func price(p *float64, currency string) string {
	if p == nil {
		return nd
	}
	s := esNum(*p, 2)
	if currency != "" {
		s += " " + currency
	}
	return s
}

func formatMetric(m evaluator.Metric, def evaluator.MetricDef, finCurrency string) string {
	if m.Text != "" {
		return m.Text
	}
	if m.Value == nil {
		return nd
	}
	v := *m.Value
	switch def.Unit {
	case "pct":
		if m.ID == "target_upside" {
			sign := ""
			if v > 0 {
				sign = "+"
			} else if v < 0 {
				sign = "-"
			}
			return sign + esNum(math.Abs(v), 1) + " %"
		}
		return esNum(v, 1) + " %"
	case "x":
		return esNum(v, 2) + "×"
	case "years":
		return esNum(v, 1) + " años"
	case "money":
		if m.Label == "Caja neta" {
			v = math.Abs(v)
		}
		return money(&v, finCurrency)
	}
	return esNum(v, 2)
}

// pillText es el texto del distintivo: el propio de la métrica, «Informativo», «Sin datos» o la etiqueta de la sección.
func pillText(level evaluator.Level, own string, labels [3]string) string {
	switch {
	case own != "":
		return own
	case level == evaluator.Info:
		return "Informativo"
	case level == evaluator.NA:
		return "Sin datos"
	}
	idx := map[evaluator.Level]int{evaluator.Good: 0, evaluator.Warn: 1, evaluator.Bad: 2}
	if i, ok := idx[level]; ok && labels[i] != "" {
		return labels[i]
	}
	return verdictText(level)
}

// sparkSVG dibuja la serie corta como minigráfico SVG: línea, puntos con título y línea de cero.
func sparkSVG(values []*float64, years []string, label string) string {
	const w, h, px, top, bottom = 300.0, 72.0, 12.0, 10.0, 62.0
	type pt struct {
		x, v float64
		i    int
	}
	var pts []pt
	for i, v := range values {
		if v != nil {
			pts = append(pts, pt{x: px + float64(i)*(w-2*px)/math.Max(float64(len(values)-1), 1), v: *v, i: i})
		}
	}
	if len(pts) < 2 {
		return ""
	}
	lo, hi := pts[0].v, pts[0].v
	for _, p := range pts {
		lo, hi = math.Min(lo, p.v), math.Max(hi, p.v)
	}
	lowV, highV := lo, hi
	if hi == lo {
		lowV, highV = lo-1, hi+1
	}
	y := func(v float64) float64 { return bottom - (v-lowV)/(highV-lowV)*(bottom-top) }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %g %g" role="img" aria-label="Evolución de %s: %s">`, w, h, esc(label), esc(strings.Join(years, ", ")))
	for _, g := range []float64{top, (top + bottom) / 2, bottom} {
		fmt.Fprintf(&b, `<line class="grid" x1="0" x2="%g" y1="%g" y2="%g"/>`, w, g, g)
	}
	if lo < 0 && hi > 0 {
		fmt.Fprintf(&b, `<line class="zero" x1="0" x2="%g" y1="%.1f" y2="%.1f"/>`, w, y(0), y(0))
	}
	coords := make([]string, len(pts))
	for i, p := range pts {
		coords[i] = strconv.FormatFloat(p.x, 'f', 1, 64) + "," + strconv.FormatFloat(y(p.v), 'f', 1, 64)
	}
	fmt.Fprintf(&b, `<polyline points="%s"/>`, strings.Join(coords, " "))
	for k, p := range pts {
		r := 3
		if k == len(pts)-1 {
			r = 5
		}
		yr := ""
		if p.i < len(years) {
			yr = years[p.i]
		}
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%d"><title>%s: %s</title></circle>`, p.x, y(p.v), r, esc(yr), esc(esNum(p.v, 1)))
	}
	b.WriteString(`</svg><div class="years">`)
	for _, yr := range years {
		b.WriteString("<span>" + esc(yr) + "</span>")
	}
	b.WriteString("</div>")
	return b.String()
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
