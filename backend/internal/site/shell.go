// Cáscara común del sitio: pestañas agrupadas, botones Home / About / tema, título, subtítulo y pie de página.
// Todas las páginas se generan desde Go; el Lab tiene su propia cáscara (newLabShell).
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"strings"

	"topwx/internal/report"
)

const (
	siteName = "Momentum y Scores"
)

// tab es una pestaña del sitio.
type tab struct {
	ID, Group, Label, Title, Sub, Note, File string
}

var tabs = []tab{
	{
		ID: "actual", Group: "Momentum", Label: "Top actual", File: "index.html",
		Sub: "Momentum: cada N meses se compran, con ponderación igual, las acciones que más subieron en los últimos N meses.",
	},
	{
		ID: "etfq", Group: "ETFs", Label: "ETFs por gestora", Title: "ETFs por gestora", File: "etfq.html",
		Sub:  "Los ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum ajustado por riesgo y calidad (costo y tamaño) dentro de cada categoría.",
		Note: "ETFs por gestora: TER, AUM y categoría de Yahoo Finance; foto de hoy, sin backtest.",
	},
	{
		ID: "quant", Group: "Acciones por score", Label: "EEUU", Title: "Modelo por score", File: "quant.html",
		Sub:  "Scores de momentum, salud financiera y técnico de las acciones del S&P 500, su promedio y Markowitz sobre las mejores.",
		Note: "Modelo por score: foto de hoy, sin backtest; los datos financieros son los últimos publicados (Yahoo Finance).",
	},
	{
		ID: "evaluador", Group: "Análisis fundamental", Label: "Evaluador de empresas", Title: "Evaluador de empresas", File: "evaluador.html",
		Sub:  "Análisis fundamental de cada acción del S&P 500: rentabilidad, crecimiento, caja, deuda, valoración, tendencia y comparación con competidores. Es independiente de los scores.",
		Note: "Evaluador: foto de hoy con los últimos estados financieros publicados (Yahoo Finance); los umbrales son reglas generales y dependen del sector.",
	},
	{
		ID: "visor", Group: "Visor", Label: "Acciones y ETFs", Title: "Visor de acciones y ETFs", File: "visor.html",
		Sub:  "Rendimientos de 1 a 12 meses y de 1 a 5 años, Sharpe, beta, caída máxima y comparación contra SPY de las acciones de EE. UU. con capitalización mayor a 100 B y de los ETFs de mayor AUM.",
		Note: "Visor: foto de hoy, sin backtest; precios ajustados por dividendos y splits de Yahoo Finance, y SPY como referencia.",
	},
}

// aboutTab no está en los grupos: va en la esquina junto a Home.
var aboutTab = tab{ID: "about", Label: "About", Title: "About", File: "about.html", Sub: "Qué se muestra en esta aplicación y cómo leerlo."}

// Meta son los datos de la corrida que muestran el subtítulo y el pie (salen de report.json; todos son opcionales).
type Meta struct {
	Generated      string
	RunDate        string
	TopN, Months   int
	InitialCapital float64
	Commission     *float64
}

func metaOf(r *report.Report) Meta {
	if r == nil {
		return Meta{}
	}
	m := Meta{
		Generated: strings.Replace(r.GeneratedAt, "T", " ", 1), TopN: r.Settings.TopN, Months: r.Settings.Months,
		InitialCapital: r.Settings.InitialCapital, Commission: &r.Settings.Commission,
	}
	if r.RunDate != nil {
		m.RunDate = *r.RunDate
	}
	return m
}

type navLink struct {
	Label, Href string
	Active      bool
}

type navGroup struct {
	Label string
	On    bool
	Links []navLink
}

type shell struct {
	SiteName, DocTitle, Title, Sub, Footer string
	Root                                   string // prefijo relativo a la raíz del sitio («» o «../»)
	Groups                                 []navGroup
	Home, About                            navLink
	Body                                   any
}

func (t tab) href() string { return t.File }

func (t tab) title(m Meta) string {
	switch {
	case t.Title != "":
		return t.Title
	case m.TopN > 0:
		return fmt.Sprintf("Top %d del S&P 500", m.TopN)
	}
	return "Top N del S&P 500"
}

// newShell arma la cáscara de la pestaña current. root es el prefijo para llegar a la raíz desde la página («../» en subcarpetas).
func newShell(current string, m Meta, generatedFallback, root string, body any) shell {
	cur := aboutTab
	var groups []navGroup
	for _, t := range tabs {
		if t.ID == current {
			cur = t
		}
		if len(groups) == 0 || groups[len(groups)-1].Label != t.Group {
			groups = append(groups, navGroup{Label: t.Group})
		}
		g := &groups[len(groups)-1]
		g.Links = append(g.Links, navLink{Label: t.Label, Href: t.href(), Active: t.ID == current})
		g.On = g.On || t.ID == current
	}
	sub := cur.Sub
	if m.RunDate != "" && cur.ID != aboutTab.ID {
		sub += " Corrida del " + m.RunDate + "."
	}
	generated := m.Generated
	if generated == "" {
		generated = generatedFallback
	}
	title := cur.title(m)
	return shell{
		SiteName: siteName, DocTitle: siteName + " · " + title, Title: title, Sub: sub, Footer: footer(generated, cur.Note, m), Root: root,
		Groups: groups, Home: navLink{Label: "Home", Href: tabs[0].href(), Active: current == tabs[0].ID},
		About: navLink{Label: aboutTab.Label, Href: aboutTab.href(), Active: current == aboutTab.ID}, Body: body,
	}
}

func footer(generated, note string, m Meta) string {
	commission := "comisión"
	if m.Commission != nil {
		commission = "comisión de " + share(*m.Commission, 2)
	}
	parts := []string{"Generado el " + generated + "."}
	if note != "" {
		parts = append(parts, note)
	}
	parts = append(parts, "Backtest con "+commission+" por lado, sin impuestos, ponderación igual entre los tickers seleccionados.",
		"Excluye tickers sin datos en Yahoo (sesgo de supervivencia). Herramienta de análisis, no una recomendación de inversión.")
	return strings.Join(parts, " ")
}

// newLabShell arma la cáscara de una página del Lab: pestañas propias (por vista), un solo botón «Volver a la app» y sin About.
// Las pestañas apuntan a las otras vistas (lab-<vista>.html); lab.js les agrega el hash para conservar el universo y el período.
func newLabShell(view string, m Meta, generatedFallback string, body any) shell {
	group := navGroup{On: true}
	title := ""
	for _, v := range labViews {
		if v.ID == view {
			title = "Lab: " + v.Title
		}
		group.Links = append(group.Links, navLink{Label: v.Label, Href: labPage(v.ID), Active: v.ID == view})
	}
	sub := "Laboratorio de backtest. Momentum: cada N meses se compran, con ponderación igual, las acciones que más subieron en los últimos N meses."
	if m.RunDate != "" {
		sub += " Corrida del " + m.RunDate + "."
	}
	generated := m.Generated
	if generated == "" {
		generated = generatedFallback
	}
	return shell{
		SiteName: siteName, DocTitle: siteName + " · " + title, Title: title, Sub: sub, Footer: footer(generated, "", m), Root: "",
		Groups: []navGroup{group}, Home: navLink{Label: "Volver a la app", Href: tabs[0].href()}, Body: body,
	}
}
