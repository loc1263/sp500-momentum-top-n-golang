// Página del Visor: tabla de acciones y de ETFs ya formateada por Go; visor.js solo filtra, ordena y pagina las filas con los valores
// crudos que van incrustados en la página.
//
// Fecha: 2026-11-19

package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"topwx/internal/visor"
)

// visorPage es el contenido propio del Visor (la cáscara va aparte, en shell).
type visorPage struct {
	Sections []sectionView
	Data     template.JS
}

type sectionView struct {
	ID    string
	Label string
	Cols  []colView
	Rows  []rowView
}

type colView struct {
	Name  string
	Label string
	Text  bool
	Kind  string // «select» (vs_spy), «text» o «range»
}

type rowView struct {
	Index  int
	Ticker string
	Cells  []cellView
}

type jsSection struct {
	ID     string   `json:"id"`
	Cols   []string `json:"cols"`
	Rows   [][]any  `json:"rows"`
	Search []int    `json:"search"`
	Text   []bool   `json:"text"`
}

type jsData struct {
	Date     string      `json:"date"`
	Sections []jsSection `json:"sections"`
}

func newVisorPage(p visor.Payload) (visorPage, error) {
	var pg visorPage
	data := jsData{Date: p.Date}
	for _, s := range p.Sections {
		sv := sectionView{ID: s.ID, Label: s.Label}
		js := jsSection{ID: s.ID, Cols: s.Cols, Rows: s.Rows, Search: s.Search}
		for _, c := range s.Cols {
			kind := "range"
			switch {
			case c == "vs_spy":
				kind = "select"
			case isText(c):
				kind = "text"
			}
			sv.Cols = append(sv.Cols, colView{Name: c, Label: label(c), Text: isText(c), Kind: kind})
			js.Text = append(js.Text, isText(c))
		}
		for i, r := range s.Rows {
			if len(r) != len(s.Cols) {
				return visorPage{}, fmt.Errorf("sección %s: la fila %d tiene %d celdas y hay %d columnas", s.ID, i, len(r), len(s.Cols))
			}
			rv := rowView{Index: i}
			for j, col := range s.Cols {
				text := format(s, col, r[j])
				cell := cellView{Class: strings.TrimSpace(cellClass(s, col, r[j]) + " " + alignClass(col)), Text: text}
				switch col {
				case "ticker":
					rv.Ticker = text
					cell.Href = tickerURL(s.ID, text)
				case "nombre":
					cell.Title = text
				}
				rv.Cells = append(rv.Cells, cell)
			}
			sv.Rows = append(sv.Rows, rv)
		}
		pg.Sections = append(pg.Sections, sv)
		data.Sections = append(data.Sections, js)
	}
	b, err := json.Marshal(data) // json.Marshal escapa <, > y &, así que va seguro dentro de <script>
	if err != nil {
		return visorPage{}, err
	}
	pg.Data = template.JS(b)
	return pg, nil
}

func alignClass(col string) string {
	switch {
	case col == "nombre":
		return "name"
	case !isText(col):
		return "r"
	}
	return ""
}
