// Package site genera el sitio estático sin Node ni bundler: Go arma el HTML (tablas ya formateadas) y copia un CSS y un
// JS plano que solo filtra, ordena y pagina las filas que ya están en la página. Hoy solo cubre el Visor.
//
// Fecha: 2026-11-19
package site

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"top/internal/visor"
)

//go:embed templates/*.html static/*
var assets embed.FS

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

// jsSection es lo único que necesita el JS para filtrar y ordenar: los valores crudos, no el formato.
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

func tickerURL(sectionID, ticker string) string {
	kind := "stocks"
	if sectionID == visor.ETFs {
		kind = "etf"
	}
	return "https://stockanalysis.com/" + kind + "/" + strings.ReplaceAll(strings.ToLower(ticker), "-", ".") + "/"
}

// Serve sirve dir por HTTP hasta que ctx se cancele (reemplaza a start.py para ver el sitio en local).
func Serve(ctx context.Context, dir, addr string) error {
	srv := &http.Server{Addr: addr, Handler: http.FileServer(http.Dir(dir)), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
