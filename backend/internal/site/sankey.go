// Diagramas de flujo (Sankey) en SVG: distribución de columnas, cintas y etiquetas. Los usan los gráficos de ingresos y de caja
// de la ficha de empresa.
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

package site

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	sankeyW, sankeyH     = 800.0, 400.0
	sankeyLeft, sankeyRt = 140.0, 140.0
	sankeyNodeW          = 14.0
	sankeyGap            = 10.0
	sankeyMinSlot        = 30.0 // alto mínimo que se reserva a cada nodo para que quepan sus dos líneas de etiqueta
	sankeyEps            = 1e-9
)

type sNode struct {
	id, label, color string
	col              int
	disp             float64 // valor que se rotula
	val              float64 // alto del nodo: el mayor entre lo que entra y lo que sale
	x, y, h          float64
	in, out          []*sLink
}

type sLink struct {
	from, to *sNode
	v        float64
	y0, y1   float64
}

type sankey struct {
	nodes []*sNode
	links []*sLink
	byID  map[string]*sNode
	base  float64 // valor que representa el 100% en las etiquetas
	money func(float64) string
	title string
}

func newSankey(title string, base float64, money func(float64) string) *sankey {
	return &sankey{byID: map[string]*sNode{}, base: base, money: money, title: title}
}

func (s *sankey) node(id, label, color string, col int, disp float64) {
	n := &sNode{id: id, label: label, color: color, col: col, disp: disp}
	s.nodes = append(s.nodes, n)
	s.byID[id] = n
}

// link une dos nodos; los flujos nulos o negativos se ignoran.
func (s *sankey) link(from, to string, v float64) {
	f, t := s.byID[from], s.byID[to]
	if f == nil || t == nil || v <= sankeyEps {
		return
	}
	l := &sLink{from: f, to: t, v: v}
	s.links = append(s.links, l)
	f.out = append(f.out, l)
	t.in = append(t.in, l)
}

func sum(ls []*sLink) float64 {
	t := 0.0
	for _, l := range ls {
		t += l.v
	}
	return t
}

func (s *sankey) pct(v float64) string {
	if s.base <= 0 {
		return ""
	}
	p := v / s.base * 100
	if p < 0.5 {
		return "<1%"
	}
	return esNum(p, 0) + "%"
}

func (s *sankey) svg() string {
	var nodes []*sNode
	cols := map[int][]*sNode{}
	maxCol := 0
	for _, n := range s.nodes {
		n.val = math.Max(sum(n.in), sum(n.out))
		if n.val <= sankeyEps {
			continue
		}
		nodes = append(nodes, n)
		cols[n.col] = append(cols[n.col], n)
		maxCol = max(maxCol, n.col)
	}
	if len(nodes) == 0 {
		return ""
	}
	colHeight := func(k float64, ns []*sNode) float64 {
		h := sankeyGap * float64(len(ns)-1)
		for _, n := range ns {
			h += math.Max(n.val*k, sankeyMinSlot)
		}
		return h
	}
	lo, hi := 0.0, 1.0
	for _, n := range nodes {
		if n.val > sankeyEps {
			hi = math.Max(hi, sankeyH/n.val)
		}
	}
	for range 60 {
		k := (lo + hi) / 2
		fits := true
		for _, ns := range cols {
			if colHeight(k, ns) > sankeyH {
				fits = false
			}
		}
		if fits {
			lo = k
		} else {
			hi = k
		}
	}
	k := lo
	step := (sankeyW - sankeyLeft - sankeyRt - sankeyNodeW) / float64(max(maxCol, 1))
	for c, ns := range cols {
		y := (sankeyH - colHeight(k, ns)) / 2
		for _, n := range ns {
			slot := math.Max(n.val*k, sankeyMinSlot)
			n.h = math.Max(n.val*k, 1.5)
			n.x = sankeyLeft + float64(c)*step
			n.y = y + (slot-n.h)/2
			y += slot + sankeyGap
		}
	}
	for _, n := range nodes {
		sort.SliceStable(n.out, func(i, j int) bool { return n.out[i].to.y < n.out[j].to.y })
		sort.SliceStable(n.in, func(i, j int) bool { return n.in[i].from.y < n.in[j].from.y })
		o, in := n.y, n.y
		for _, l := range n.out {
			l.y0 = o
			o += l.v * k
		}
		for _, l := range n.in {
			l.y1 = in
			in += l.v * k
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %g %g" role="img" aria-label="%s">`, sankeyW, sankeyH, esc(s.title))
	for _, l := range s.links {
		if l.from.val <= sankeyEps || l.to.val <= sankeyEps {
			continue
		}
		color := l.to.color
		if l.from.col == 0 {
			color = l.from.color
		}
		x0, x1, t := l.from.x+sankeyNodeW, l.to.x, l.v*k
		xm := (x0 + x1) / 2
		fmt.Fprintf(&b, `<path class="flow" style="fill:%s" d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f L%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f Z"/>`,
			color, x0, l.y0, xm, l.y0, xm, l.y1, x1, l.y1, x1, l.y1+t, xm, l.y1+t, xm, l.y0+t, x0, l.y0+t)
	}
	for _, n := range nodes {
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%g" height="%.1f" style="fill:%s"><title>%s: %s</title></rect>`,
			n.x, n.y, sankeyNodeW, n.h, n.color, esc(n.label), esc(s.money(n.disp)))
	}
	for _, n := range nodes {
		x, anchor, class := n.x+sankeyNodeW+6, "start", "lbl"
		switch {
		case n.col == 0:
			x, anchor = n.x-6, "end"
		case n.col == maxCol:
		default:
			class = "lbl mid"
		}
		yc := n.y + n.h/2
		fmt.Fprintf(&b, `<text class="%s" x="%.1f" y="%.1f" text-anchor="%s"><tspan class="n">%s</tspan><tspan class="v" x="%.1f" dy="14">%s · %s</tspan></text>`,
			class, x, yc-2, anchor, esc(n.label), x, esc(s.money(n.disp)), s.pct(n.disp))
	}
	b.WriteString(`</svg>`)
	return b.String()
}
