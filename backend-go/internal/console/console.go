// Package console es la salida por terminal: tablas de texto y resúmenes de cada tipo de corrida.
package console

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"top3/internal/dateutil"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/numfmt"
	"top3/internal/tables"
	"top3/internal/termfmt"
)

// Out es donde se imprime (la salida estándar; los tests la reemplazan).
var Out io.Writer = os.Stdout

var signedPercent = regexp.MustCompile(`^[+-]\d[\d,]*(\.\d+)?%$`)

func useColor() bool { return Out == io.Writer(os.Stdout) && logx.ColorEnabled(os.Stdout) }

// Println imprime una línea sin partirla.
func Println(args ...any) { fmt.Fprintln(Out, args...) }

// Line imprime text partiendo las líneas más largas que termfmt.MaxWidth, para que no se desborden.
func Line(text string) {
	for _, l := range termfmt.WrapDefault(text) {
		fmt.Fprintln(Out, l)
	}
}

// Linef es Line con formato.
func Linef(format string, args ...any) { Line(fmt.Sprintf(format, args...)) }

// Banner imprime un título entre marcos de `=` de `width` caracteres.
func Banner(text string, width int) {
	frame := strings.Repeat("=", width)
	if useColor() {
		frame = termfmt.Color(frame, termfmt.Green)
	}
	fmt.Fprintln(Out, "\n"+frame)
	if useColor() && len([]rune(text)) <= termfmt.MaxWidth {
		text = termfmt.Color(text, termfmt.Bold)
	}
	Line(text)
	fmt.Fprintln(Out, frame)
}

// paintCell pinta las ganancias en verde y las pérdidas en rojo (celdas con forma de porcentaje con signo); el 0 queda
// sin color.
func paintCell(cell, padded string) string {
	c := strings.TrimSpace(cell)
	if signedPercent.MatchString(c) {
		v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSuffix(c, "%"), ",", ""), 64)
		if err == nil && v != 0 {
			if strings.HasPrefix(c, "+") {
				return termfmt.Color(padded, termfmt.Green)
			}
			return termfmt.Color(padded, termfmt.Red)
		}
	}
	return padded
}

// Table imprime una tabla de texto: las columnas desde rightFrom van alineadas a la derecha. Si no entra en el ancho
// máximo se recortan (con «…») las columnas más anchas. Encabezado en negrita; ganancias en verde y pérdidas en rojo.
func Table(headers []string, rows [][]string, rightFrom int) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len([]rune(h))
	}
	for _, r := range rows {
		for i := 0; i < len(headers) && i < len(r); i++ {
			widths[i] = max(widths[i], len([]rune(r[i])))
		}
	}
	widths = termfmt.FitWidths(widths, 3, termfmt.MaxWidth, 8)
	colored := useColor()

	line := func(cells []string, paint bool) string {
		parts := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			cell = termfmt.Truncate(cell, widths[i])
			padded := termfmt.PadRight(cell, widths[i])
			if i >= rightFrom {
				padded = termfmt.PadLeft(cell, widths[i])
			}
			if paint && colored {
				padded = paintCell(cell, padded)
			}
			parts[i] = padded
		}
		return strings.Join(parts, " | ")
	}
	head := line(headers, false)
	if colored {
		head = termfmt.Color(head, termfmt.Bold)
	}
	fmt.Fprintln(Out, head)
	seps := make([]string, len(widths))
	for i, w := range widths {
		seps[i] = strings.Repeat("-", w)
	}
	fmt.Fprintln(Out, strings.Join(seps, "-+-"))
	for _, r := range rows {
		fmt.Fprintln(Out, line(r, true))
	}
}

// PrintTable imprime una tabla de tables.Table.
func PrintTable(t tables.Table, rightFrom int) { Table(t.Headers, t.Rows, rightFrom) }

// History imprime el historial de selecciones.
func History(history []model.Rebalance, equity []model.EquityPoint, initialCapital float64) {
	t := tables.HistoryTable(history, equity, initialCapital)
	PrintTable(t, len(t.Headers)-2)
}

// Run son los datos de una corrida del backtest que la consola muestra.
type Run struct {
	Start, End     time.Time
	Years          float64
	InitialCapital float64
	TopN           int
}

// Backtest imprime el resultado del backtest normal de un plazo.
func Backtest(r Run, term model.Term, minCapB float64) {
	base := term.Base()
	n := term.Months
	median, low, high := term.CagrStats()
	Banner(fmt.Sprintf("RESULTADOS BACKTEST - Top %d%s (Point-in-Time)", r.TopN, market.Title(minCapB)), 65)
	Linef("Período:             %s a %s (%s años)", dateutil.Day(r.Start), dateutil.Day(r.End), numfmt.G(r.Years))
	Linef("Plazo:               %d meses (ranking %d meses, rebalanceo cada %d meses)", n, n, n)
	Linef("Capital inicial:     US$ %s", numfmt.Comma(r.InitialCapital, 2))
	Linef("Capital final:       US$ %s   (calendario base)", numfmt.Comma(base.FinalCapital, 2))
	Linef("Retorno total:       %.1f%%", base.TotalReturnPct)
	Linef("CAGR aproximado:     %.1f%%", base.CAGRPct)
	Linef("CAGR mediana:        %s   (rango %s a %s, %d calendarios)", numfmt.Signed(median, 1)+"%",
		numfmt.Signed(low, 1)+"%", numfmt.Signed(high, 1)+"%", len(term.Calendars))
	Line(strings.Repeat("=", 65))

	Line("\nHistorial de selecciones (calendario base):")
	History(base.History, base.Equity, r.InitialCapital)

	Linef("\nCalendarios de arranque posibles (%d):", len(term.Calendars))
	PrintTable(tables.CalendarTable(term.Calendars), 1)

	Line("\nCurva de capital:")
	for _, e := range base.Equity {
		fmt.Fprintf(Out, "%s  %s\n", dateutil.Day(e.Date), numfmt.Comma(e.Capital, 2))
	}
}

// Sweep imprime la comparación de plazos.
func Sweep(r Run, terms []model.Term, minCapB float64) {
	if minCapB != 0 {
		Linef("\nUniverso: %s (cap. aprox. ≥ US$ %s B)", market.Label(minCapB), numfmt.G(minCapB))
	}
	for _, t := range terms {
		if t.Base() != nil {
			Banner(fmt.Sprintf("SELECCIONES - plazo %d meses (ranking %d meses, rebalanceo cada %d meses)", t.Months, t.Months, t.Months), 65)
			History(t.Base().History, t.Base().Equity, r.InitialCapital)
		}
	}
	Banner(fmt.Sprintf("COMPARACIÓN DE PLAZOS - %s a %s", dateutil.Day(r.Start), dateutil.Day(r.End)), 100)
	PrintTable(tables.SweepTable(terms), 1)
	Line("Capital final / Retorno total / CAGR = calendario base (arranque en el primer mes); " +
		"mediana y rango = los N calendarios posibles del plazo.")
	if best := tables.SweepBest(terms, r.InitialCapital); best != "" {
		Line("\n" + best)
	}
}

func universeSuffix(minCapB float64) string {
	if minCapB == 0 {
		return ""
	}
	return " - " + market.Label(minCapB)
}

// Models imprime el modelo N-0 frente al skip-month N-1.
func Models(r Run, terms, skipTerms []model.Term, minCapB float64) {
	Banner(fmt.Sprintf("MODELO N-0 vs. SKIP-MONTH N-1 - %s a %s%s", dateutil.Day(r.Start), dateutil.Day(r.End), universeSuffix(minCapB)), 85)
	PrintTable(tables.ModelsTable(terms, skipTerms), 1)
	Line("N-0 = ranking con los últimos N meses hasta hoy; N-1 = los N meses que terminan un mes antes. " +
		"CAGR y capital = mediana de los calendarios de arranque.")
}

// Offsets imprime la sensibilidad al arranque.
func Offsets(r Run, terms []model.Term, minCapB float64) {
	Banner(fmt.Sprintf("SENSIBILIDAD AL ARRANQUE - %s a %s%s", dateutil.Day(r.Start), dateutil.Day(r.End), universeSuffix(minCapB)), 85)
	PrintTable(tables.OffsetsTable(terms), 1)
	if best := tables.OffsetsBest(terms); best != "" {
		Line("\n" + best)
	}
	for _, t := range terms {
		if len(t.Calendars) > 0 {
			Linef("\nPlazo %d meses - un calendario por mes de arranque:", t.Months)
			PrintTable(tables.CalendarTable(t.Calendars), 1)
		}
	}
}

// Etf imprime el ranking de ETFs. Los encabezados y filas traen la columna «Nombre» en el puesto 2: no se muestra.
func Etf(asOf string, horizons []int, topN, ranked int, headers []string, rows [][]string, notes []string, historical bool, riskFree string) {
	Banner(fmt.Sprintf("ETFs DE VANGUARD - momentum ajustado por riesgo (%d rankeados)", ranked), 78)
	Line("Datos al cierre del " + asOf)
	if historical {
		Println("(fecha pedida con --as-of: lista actual, no es un backtest)")
	}
	excess := ""
	if riskFree != "" {
		excess = fmt.Sprintf(" (exceso sobre %s)", riskFree)
	}
	hs := make([]string, len(horizons))
	for i, h := range horizons {
		hs[i] = strconv.Itoa(h)
	}
	Linef("Score = promedio del retorno%s / volatilidad en %s meses. Top %d:\n", excess, strings.Join(hs, ", "), topN)
	drop := func(r []string) []string { return append(append([]string(nil), r[:2]...), r[3:]...) }
	shown := rows[:min(topN, len(rows))]
	cut := make([][]string, len(shown))
	for i, r := range shown {
		cut[i] = drop(r)
	}
	Table(drop(headers), cut, 3)
	Line("")
	for _, n := range notes {
		if n != "" {
			Line(n)
		}
	}
}

// Actual imprime el Top actual.
func Actual(topN, months int, minCapB float64, asOf string, windowStart string, windowRows int, headers []string,
	rows [][]string, weight float64, notes []string, alternates [][]string) {
	Banner(fmt.Sprintf("TOP %d S&P 500%s - últimos %d meses", topN, market.Title(minCapB), months), 70)
	Linef("Datos al cierre del %s (ventana %s a %s, %d ruedas)", asOf, windowStart, asOf, windowRows)
	if minCapB != 0 {
		Linef("Universo: %s (cap. aprox. >= US$ %s B)", market.Label(minCapB), numfmt.G(minCapB))
	}
	Line("")
	Table(headers, rows, 1)
	Linef("\n→ Ponderación igual: %.1f%% cada una. Se rebalancea cada %d meses.", weight, months)
	for _, n := range notes {
		Line(n)
	}
	if len(alternates) > 0 {
		Line("\nSuplentes (puestos siguientes, como reemplazo posible de alguna acción del top):")
		Table(headers, alternates, 1)
	}
}
