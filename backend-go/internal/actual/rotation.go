// Package actual calcula el Top N actual del S&P 500: las acciones que más subieron en los últimos N meses (el criterio
// del backtest, aplicado a hoy), con sus suplentes y la rotación contra la corrida anterior.
package actual

import (
	"fmt"
	"strings"
	"time"

	"top3/internal/dateutil"
)

// Changes son los tickers que entran, salen y se mantienen (en el orden del ranking actual / anterior).
type Changes struct {
	Entered, Exited, Kept []string
}

// DiffPicks compara la selección anterior con la actual.
func DiffPicks(previous, current []string) Changes {
	in := func(list []string, t string) bool {
		for _, x := range list {
			if x == t {
				return true
			}
		}
		return false
	}
	c := Changes{Entered: []string{}, Exited: []string{}, Kept: []string{}}
	for _, t := range current {
		if in(previous, t) {
			c.Kept = append(c.Kept, t)
		} else {
			c.Entered = append(c.Entered, t)
		}
	}
	for _, t := range previous {
		if !in(current, t) {
			c.Exited = append(c.Exited, t)
		}
	}
	return c
}

// NextRebalance es el primer inicio de mes de la grilla anchor + k x months que no es anterior a today.
func NextRebalance(anchor string, months int, today time.Time) (time.Time, error) {
	a, err := dateutil.Parse(anchor)
	if err != nil {
		return time.Time{}, err
	}
	d := dateutil.Date(a.Year(), a.Month(), 1)
	for d.Before(today) {
		d = dateutil.AddMonths(d, months)
	}
	return d, nil
}

func orDash(list []string) string {
	if len(list) == 0 {
		return "-"
	}
	return strings.Join(list, ", ")
}

// Describe son las líneas de texto para la consola: qué entró y salió y cuándo toca rebalancear.
func Describe(previousAsOf string, changes Changes, anchor string, months int, today time.Time) []string {
	var lines []string
	switch {
	case previousAsOf == "":
		lines = append(lines, "Cambios: primera corrida guardada, sin selección anterior para comparar.")
	case len(changes.Entered) == 0 && len(changes.Exited) == 0:
		lines = append(lines, fmt.Sprintf("Cambios vs. %s: ninguno, la selección se mantiene.", previousAsOf))
	default:
		lines = append(lines, fmt.Sprintf("Cambios vs. %s: entran %s; salen %s; se mantienen %s.", previousAsOf,
			orDash(changes.Entered), orDash(changes.Exited), orDash(changes.Kept)))
	}
	if anchor != "" {
		if next, err := NextRebalance(anchor, months, today); err == nil {
			lines = append(lines, fmt.Sprintf("Próximo rebalanceo: %s (cada %d meses desde la primera corrida guardada, %s).",
				dateutil.Day(next), months, anchor))
		}
	}
	return lines
}
