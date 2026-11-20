// Página «Top actual» (Home): por cada universo, las acciones elegidas, los suplentes, los cambios contra la corrida anterior y el
// seguimiento de selecciones pasadas. Un selector muestra un universo a la vez (table.js, data-switch).
//
// Fecha: 2026-11-19

package site

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"topwx/internal/report"
)

type actualPage struct {
	Options  []option
	Universe []actualView
}

type option struct {
	Value, Label string
	Selected     bool
}

type actualView struct {
	Key           string
	Hidden        bool
	AsOf          string
	Months        int
	PickCount     int
	Weight        string
	NextRebalance string
	Anchor        string
	From          string
	Changes       *changesView
	FirstRun      bool
	PreviousAsOf  string
	Picks         []trow
	Alternates    []trow
	Tracking      []trow
}

type changesView struct {
	Previous              string
	None                  bool
	Entered, Exited, Kept string
}

func capKey(a report.Actual) string { return capKeyOf(a.MinCapBillions) }

func capKeyOf(min *float64) string {
	if min == nil {
		return "0"
	}
	return fnum(*min)
}

func newActualPage(list []report.Actual) actualPage {
	best := 0
	for i, a := range list {
		if capValue(a) > capValue(list[best]) {
			best = i
		}
	}
	var pg actualPage
	for i, a := range list {
		pg.Options = append(pg.Options, option{Value: capKey(a), Label: a.Label, Selected: i == best})
		pg.Universe = append(pg.Universe, newActualView(a, i != best))
	}
	return pg
}

func capValue(a report.Actual) float64 {
	if a.MinCapBillions == nil {
		return 0
	}
	return *a.MinCapBillions
}

func newActualView(a report.Actual, hidden bool) actualView {
	v := actualView{Key: capKey(a), Hidden: hidden, AsOf: a.AsOf, Months: a.Months, PickCount: len(a.Picks), NextRebalance: a.NextRebalance}
	weight := 0.0
	if len(a.Picks) > 0 {
		weight = 1 / float64(len(a.Picks))
	}
	v.Weight = share(weight, 1)
	if a.Anchor != nil {
		v.Anchor = *a.Anchor
	}
	if d, err := time.Parse("2006-01-02", a.AsOf); err == nil {
		v.From = d.AddDate(0, -a.Months, 0).Format("2006-01-02")
	}
	switch {
	case a.Changes != nil:
		c := a.Changes
		prev := ""
		if a.PreviousAsOf != nil {
			prev = *a.PreviousAsOf
		}
		v.Changes = &changesView{Previous: prev, None: len(c.Entered) == 0 && len(c.Exited) == 0, Entered: list(c.Entered), Exited: list(c.Exited), Kept: list(c.Kept)}
	case a.PreviousAsOf == nil:
		v.FirstRun = true
	}
	for _, p := range a.Picks {
		v.Picks = append(v.Picks, pickRow(p, true, weight))
	}
	for _, p := range a.Alternates {
		v.Alternates = append(v.Alternates, pickRow(p, false, 0))
	}
	for _, t := range a.Tracking {
		status := "en curso"
		if t.Closed {
			status = "cerrada"
		}
		var picks []string
		for _, p := range t.Picks {
			picks = append(picks, fmt.Sprintf(`%s <span class="%s">%s</span>`, esc(p.Ticker), signClass(&p.Return), pctFrac(&p.Return, 1)))
		}
		v.Tracking = append(v.Tracking, trow{Cells: []cellView{
			{Text: t.AsOf}, {Text: t.Until}, {Class: "r", Text: strconv.Itoa(t.Days)}, {Text: status},
			{HTML: template.HTML(strings.Join(picks, ", "))}, {Class: strings.TrimSpace("r " + signClass(&t.Return)), Text: pctFrac(&t.Return, 1)},
		}})
	}
	return v
}

func pickRow(p report.ActualPick, withWeight bool, weight float64) trow {
	ret := p.Return
	cells := []cellView{
		{Text: strconv.Itoa(p.Rank)},
		{Text: p.Ticker, Href: tickerURL("acciones", p.Ticker), Ext: true},
		{Class: strings.TrimSpace("r " + signClass(&ret)), Text: pctFrac(&ret, 1)},
		{Class: "r", Text: billions(p.MarketCap)},
		{Class: "r", Text: usd(&p.Price)},
	}
	if withWeight {
		cells = append(cells, cellView{Class: "r", Text: share(weight, 1)})
	}
	return trow{Cells: cells}
}

func list(tickers []string) string {
	if len(tickers) == 0 {
		return "-"
	}
	return strings.Join(tickers, ", ")
}
