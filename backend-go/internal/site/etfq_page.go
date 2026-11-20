// Página «ETFs por gestora»: ranking de ETFs por score final (momentum + calidad), resumen por gestora y filtros.
// Las filas salen ya armadas; table.js solo filtra, ordena y pagina.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"

	"topwx/internal/etfq"
)

// tcol describe un encabezado de tabla ordenable (lo lee table.js).
type tcol struct {
	Label    string
	Num      bool
	Dir      string // sentido del primer clic: «asc» o «desc»
	Initial  string // «asc» / «desc» si es el orden con que vienen las filas
	AriaSort string
	Views    string // vistas en que se muestra la columna (table.js, data-views)
	Help     string
	Arrow    string
}

type etfqPage struct {
	Overview     overview
	Intro        string
	Issuers      []etfq.Issuer
	IssuerNames  []string
	Categories   []string
	Cols         []tcol
	Rows         []trow
	Total        int
	NotRanked    int
	ColSpan      int
	RiskFreeText string
}

type overview struct {
	AsOf, Listed, IssuerCount, Scored, Excluded, Eligibility, RiskFree string
}

// trow es una fila de tabla: sus atributos data-* y sus celdas.
type trow struct {
	Class  string
	Href   string // si no está vacío, un clic en la fila navega ahí
	Search string
	Attrs  map[string]string // se emiten como data-f-<clave>
	Cells  []cellView
}

func newEtfqPage(p etfq.Payload) etfqPage {
	s := p.Settings
	scored, excluded, listed := 0, 0, 0
	for _, r := range p.Rows {
		if r.Rank != nil {
			scored++
		}
		if len(r.Excluded) > 0 {
			excluded++
		}
	}
	for _, i := range p.Issuers {
		listed += i.Funds
	}
	elig := "Sin tope de TER"
	if s.MaxTER > 0 {
		elig = "TER ≤ " + esNum(s.MaxTER, 2) + "%"
	}
	if s.MinAumMillions > 0 {
		elig += ", AUM ≥ US$ " + esNum(s.MinAumMillions, 0) + " M"
	} else {
		elig += ", Sin mínimo de AUM"
	}
	ov := overview{
		AsOf: p.AsOf, Listed: strconv.Itoa(listed), IssuerCount: strconv.Itoa(len(p.Issuers)), Scored: strconv.Itoa(scored),
		Excluded: strconv.Itoa(excluded), Eligibility: elig,
	}
	if p.RiskFree != nil {
		ov.RiskFree = *p.RiskFree
	}
	pg := etfqPage{Overview: ov, Issuers: p.Issuers, Total: len(p.Rows), NotRanked: p.NotRanked}
	pg.Intro = fmt.Sprintf("Score final = %s momentum + %s calidad. «Momentum» es la posición (0-100) del score de retorno ajustado por riesgo entre los ETFs elegibles. "+
		"«Calidad» promedia la posición del TER (menos es mejor, peso %s) y del AUM (más es mejor, peso %s) dentro de su misma categoría, sea cual sea la gestora. "+
		"Los ETFs que no pasan el filtro de TER o AUM quedan fuera del ranking final; los que no informan TER ni AUM no puntúan.",
		share(1-s.QualityWeight, 0), share(s.QualityWeight, 0), esNum(s.TERWeight, 0), esNum(s.AUMWeight, 0))
	for _, i := range p.Issuers {
		pg.IssuerNames = append(pg.IssuerNames, i.Name)
	}
	seen := map[string]bool{}
	for _, r := range p.Rows {
		if !seen[r.Category] {
			seen[r.Category] = true
			pg.Categories = append(pg.Categories, r.Category)
		}
	}
	sort.Slice(pg.Categories, func(i, j int) bool { return collateEs(pg.Categories[i], pg.Categories[j]) < 0 })

	pg.Cols = []tcol{
		{Label: "#", Dir: "asc", Initial: "asc"}, {Label: "Ticker", Dir: "asc"}, {Label: "Nombre", Dir: "asc"}, {Label: "Gestora", Dir: "asc"},
		{Label: "Categoría", Dir: "asc"}, {Label: "Final", Num: true, Dir: "desc"}, {Label: "Momentum", Num: true, Dir: "desc"},
		{Label: "Calidad", Num: true, Dir: "desc"}, {Label: "TER", Num: true, Dir: "asc"}, {Label: "AUM", Num: true, Dir: "desc"},
	}
	for _, h := range p.Horizons {
		pg.Cols = append(pg.Cols, tcol{Label: fmt.Sprintf("Ret. %dm", h), Num: true, Dir: "desc"})
	}
	for i := range pg.Cols {
		pg.Cols[i].AriaSort = "none"
		if pg.Cols[i].Initial != "" {
			pg.Cols[i].AriaSort = "ascending"
			pg.Cols[i].Arrow = " ▲"
		}
	}
	pg.ColSpan = len(pg.Cols)

	for _, r := range p.Rows {
		pg.Rows = append(pg.Rows, etfqRow(r, p, s))
	}
	return pg
}

func etfqRow(r etfq.Row, p etfq.Payload, s etfq.Settings) trow {
	scored := "0"
	if r.Rank != nil {
		scored = "1"
	}
	row := trow{
		Search: strings.ToLower(r.Ticker + " " + r.Name),
		Attrs:  map[string]string{"issuer": r.Issuer, "category": r.Category, "scored": scored},
	}
	if r.Rank == nil {
		row.Class = "out"
	}
	rank := cellView{Text: "–"}
	if r.Rank != nil {
		rank = cellView{Text: strconv.Itoa(*r.Rank), V: strconv.Itoa(*r.Rank)}
	}
	final := cellView{Class: "r muted why", Text: etfqReason(r, s)}
	if r.Final != nil {
		final = cellView{Class: "r", Text: esNum(*r.Final, 1), V: fnum(*r.Final), Strong: true}
	}
	terClass, aumClass := "r", "r"
	if contains(r.Excluded, "ter") {
		terClass += " neg"
	}
	if contains(r.Excluded, "aum") {
		aumClass += " neg"
	}
	ter := cellView{Class: terClass, Text: nd, V: vOf(r.TER)}
	if r.TER != nil {
		ter.Text = esNum(*r.TER, 2) + "%"
	}
	row.Cells = []cellView{
		rank,
		{Text: r.Ticker, Href: tickerURL("etf", r.Ticker), Ext: true},
		{Text: r.Name},
		{Class: "muted", Text: r.Issuer},
		{Class: "muted", Text: r.Category, Title: fmt.Sprintf("%d ETFs elegibles en esta categoría", r.GroupSize)},
		final,
		{Class: "r", Text: dec(r.Momentum, 1), V: vOf(r.Momentum)},
		{Class: "r", Text: dec(r.Quality, 1), V: vOf(r.Quality)},
		ter,
		{Class: aumClass, Text: aum(r.AUM), V: vOf(r.AUM)},
	}
	for _, h := range p.Horizons {
		var v *float64
		if x, ok := r.Returns[strconv.Itoa(h)]; ok {
			v = &x
		}
		row.Cells = append(row.Cells, cellView{Class: strings.TrimSpace("r " + signClass(v)), Text: pctFrac(v, 1), V: vOf(v)})
	}
	return row
}

func etfqReason(r etfq.Row, s etfq.Settings) string {
	if len(r.Excluded) == 0 {
		return "sin TER ni AUM"
	}
	parts := make([]string, len(r.Excluded))
	for i, e := range r.Excluded {
		if e == "ter" {
			parts[i] = "TER > " + esNum(s.MaxTER, 2) + "%"
		} else {
			parts[i] = "AUM < " + esNum(s.MinAumMillions, 0) + " M"
		}
	}
	return strings.Join(parts, " y ")
}

// fnum es el valor crudo para ordenar (data-v): decimal con punto, sin formato.
func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func vOf(p *float64) string {
	if p == nil {
		return ""
	}
	return fnum(*p)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// FAttrs emite los atributos data-f-<clave> de la fila (ordenados) para que table.js los filtre.
func (r trow) FAttrs() template.HTMLAttr {
	keys := make([]string, 0, len(r.Attrs))
	for k := range r.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, ` data-f-%s="%s"`, k, esc(r.Attrs[k]))
	}
	return template.HTMLAttr(b.String())
}
