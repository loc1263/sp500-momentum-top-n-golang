// Gráfico de líneas SVG de series de capital en el tiempo: ejes, líneas y leyenda salen de Go; chart.js
// agrega el cursor y el cuadro de valores al pasar el mouse, a partir de los datos incrustados en data-chart.
//
// Fecha: 2026-11-19

package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	chartW, chartH                               = 800.0, 280.0
	chartTop, chartRight, chartBottom, chartLeft = 12.0, 16.0, 28.0, 64.0
)

type chartPoint struct {
	Date  string
	Value float64
}

// chartSeries es una línea del gráfico. Width y Opacity en cero toman los valores por defecto (2 y 1); con Opacity < 1 la serie
// es de fondo: no aparece en la leyenda ni en el cuadro de valores.
type chartSeries struct {
	Name, Color    string
	Width, Opacity float64
	Points         []chartPoint
}

func (s chartSeries) faint() bool { return s.Opacity != 0 && s.Opacity < 1 }

func days(date string) (float64, bool) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, false
	}
	return float64(t.Unix()) / 86400, true
}

type chartData struct {
	ML, IW, MT, IH float64
	X0, X1, Y0, Y1 float64
	Series         []chartDataSeries
}

type chartDataSeries struct {
	N string   `json:"n"`
	C string   `json:"c"`
	P [][2]any `json:"p"`
}

func chartHTML(series []chartSeries, label string) template.HTML {
	var all []chartPoint
	for _, s := range series {
		all = append(all, s.Points...)
	}
	if len(all) == 0 {
		return ""
	}
	xMin, xMax := math.Inf(1), math.Inf(-1)
	yMin, yMax := math.Inf(1), math.Inf(-1)
	for _, p := range all {
		if d, ok := days(p.Date); ok {
			xMin, xMax = math.Min(xMin, d), math.Max(xMax, d)
		}
		yMin, yMax = math.Min(yMin, p.Value), math.Max(yMax, p.Value)
	}
	if math.IsInf(xMin, 0) {
		return ""
	}
	innerW, innerH := chartW-chartLeft-chartRight, chartH-chartTop-chartBottom
	spanX, spanY := xMax-xMin, yMax-yMin
	if spanX == 0 {
		spanX = 1
	}
	if spanY == 0 {
		spanY = 1
	}
	x := func(d float64) float64 { return chartLeft + (d-xMin)/spanX*innerW }
	y := func(v float64) float64 { return chartTop + innerH - (v-yMin)/spanY*innerH }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %g %g" role="img" aria-label="%s">`, chartW, chartH, esc(label))
	for i := 0; i < 5; i++ {
		v := yMin + (yMax-yMin)*float64(i)/4
		fmt.Fprintf(&b, `<line class="gline" x1="%g" x2="%g" y1="%.1f" y2="%.1f"/><text class="tick" x="%g" y="%.1f" text-anchor="end">%s</text>`,
			chartLeft, chartW-chartRight, y(v), y(v), chartLeft-8, y(v)+4, esNum(v, 0))
	}
	var years []string
	for yr := time.Unix(int64(xMin*86400), 0).UTC().Year() + 1; yr <= time.Unix(int64(xMax*86400), 0).UTC().Year(); yr++ {
		years = append(years, strconv.Itoa(yr))
	}
	step := int(math.Max(1, math.Ceil(float64(len(years))/math.Max(1, math.Floor(innerW/40)))))
	for i, yr := range years {
		if i%step != 0 {
			continue
		}
		d, _ := days(yr + "-01-01")
		fmt.Fprintf(&b, `<line class="gline" x1="%.1f" x2="%.1f" y1="%g" y2="%g"/><text class="tick" x="%.1f" y="%g" text-anchor="middle">%s</text>`,
			x(d), x(d), chartTop, chartTop+innerH, x(d), chartH-8, yr)
	}
	data := chartData{ML: chartLeft, IW: innerW, MT: chartTop, IH: innerH, X0: xMin, X1: xMax, Y0: yMin, Y1: yMax}
	for _, s := range series {
		width, opacity := s.Width, s.Opacity
		if width == 0 {
			width = 2
		}
		if opacity == 0 {
			opacity = 1
		}
		var d strings.Builder
		pts := make([][2]any, 0, len(s.Points))
		for i, p := range s.Points {
			dd, ok := days(p.Date)
			if !ok {
				continue
			}
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%.1f,%.1f ", cmd, x(dd), y(p.Value))
			pts = append(pts, [2]any{p.Date, math.Round(p.Value*100) / 100})
		}
		fmt.Fprintf(&b, `<path d="%s" fill="none" style="stroke:%s" stroke-width="%g" stroke-opacity="%g" stroke-linejoin="round"/>`, strings.TrimSpace(d.String()), esc(s.Color), width, opacity)
		if !s.faint() {
			data.Series = append(data.Series, chartDataSeries{N: s.Name, C: s.Color, P: pts})
		}
	}
	b.WriteString(`</svg><div class="tip" hidden></div><div class="legend">`)
	for _, s := range data.Series {
		fmt.Fprintf(&b, `<span><span class="dot" style="background:%s"></span>%s</span>`, esc(s.C), esc(s.N))
	}
	b.WriteString("</div>")
	raw, err := json.Marshal(map[string]any{"ml": data.ML, "iw": data.IW, "mt": data.MT, "ih": data.IH, "x0": data.X0, "x1": data.X1, "y0": data.Y0, "y1": data.Y1, "s": data.Series})
	if err != nil {
		return ""
	}
	return template.HTML(`<div class="chart" data-chart="` + esc(string(raw)) + `">` + b.String() + `</div>`)
}
