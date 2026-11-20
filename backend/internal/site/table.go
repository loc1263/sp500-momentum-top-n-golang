// Piezas comunes de las tablas que arma Go y que table.js hace interactivas: encabezados, filas, celdas, opciones de selector y los
// valores crudos (data-v) con que se ordena.
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"

	"topwx/internal/visor"
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

// newCols completa el estado inicial de los encabezados: el que tiene Initial queda marcado como el orden actual.
func newCols(cols []tcol) []tcol {
	for i := range cols {
		cols[i].AriaSort = "none"
		switch cols[i].Initial {
		case "asc":
			cols[i].AriaSort, cols[i].Arrow = "ascending", " ▲"
		case "desc":
			cols[i].AriaSort, cols[i].Arrow = "descending", " ▼"
		}
	}
	return cols
}

// trow es una fila de tabla: sus atributos data-* y sus celdas.
type trow struct {
	Class  string
	Href   string // si no está vacío, un clic en la fila navega ahí
	Search string
	Attrs  map[string]string // se emiten como data-f-<clave>
	Cells  []cellView
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

// cellView es una celda de tabla. V es el valor crudo para ordenar (data-v); Ext marca un enlace externo; Strong, negrita;
// Dot, un punto de color (nivel del evaluador); PillLevel, un distintivo; HTML, contenido ya escapado.
type cellView struct {
	Class     string
	Text      string
	Title     string
	Href      string
	V         string
	Ext       bool
	Strong    bool
	Dot       string
	PillLevel string
	Views     string
	HTML      template.HTML
}

// option es una opción de <select>.
type option struct {
	Value, Label string
	Selected     bool
}

// fnum es el valor crudo para ordenar (data-v): decimal con punto, sin formato.
func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func vOf(p *float64) string {
	if p == nil {
		return ""
	}
	return fnum(*p)
}

// numClass es la clase de una celda numérica alineada a la derecha, con el color del signo: «r», «r pos» o «r neg».
func numClass(p *float64) string {
	if c := signClass(p); c != "" {
		return "r " + c
	}
	return "r"
}

func numClassF(v float64) string { return numClass(&v) }

// tickerURL es el enlace de un ticker en stockanalysis.com (acciones o ETFs).
func tickerURL(sectionID, ticker string) string {
	kind := "stocks"
	if sectionID == visor.ETFs {
		kind = "etf"
	}
	return "https://stockanalysis.com/" + kind + "/" + strings.ReplaceAll(strings.ToLower(ticker), "-", ".") + "/"
}
