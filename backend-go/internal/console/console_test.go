package console

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"top3/internal/model"
)

func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	Out = &buf
	t.Cleanup(func() { Out = os.Stdout })
	return &buf
}

func TestTableAlignsColumnsAndRightAlignsFromTheGivenOne(t *testing.T) {
	buf := capture(t)
	Table([]string{"Ticker", "Retorno"}, [][]string{{"AAPL", "+12.3%"}, {"MSFT", "-4.0%"}, {"BRK-B", "+100.0%"}}, 1)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	want := []string{
		"Ticker | Retorno",
		"-------+--------",
		"AAPL   |  +12.3%",
		"MSFT   |   -4.0%",
		"BRK-B  | +100.0%",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("línea %d: %q, quiero %q", i, lines[i], w)
		}
	}
}

func TestTableShrinksWideColumnsToTheMaximumWidth(t *testing.T) {
	buf := capture(t)
	long := strings.Repeat("x", 200)
	Table([]string{"A", "B"}, [][]string{{"1", long}}, 1)
	for _, l := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if utf8.RuneCountInString(l) > 130 {
			t.Fatalf("línea de %d caracteres", utf8.RuneCountInString(l))
		}
	}
	if !strings.Contains(buf.String(), "…") {
		t.Fatal("el texto recortado debe terminar en «…»")
	}
}

func TestBannerAndLineWrapLongText(t *testing.T) {
	buf := capture(t)
	Banner("TÍTULO", 10)
	Line(strings.Repeat("palabra ", 40))
	out := buf.String()
	if !strings.Contains(out, "==========\nTÍTULO\n==========") {
		t.Fatalf("marco: %q", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if utf8.RuneCountInString(l) > 130 {
			t.Fatalf("línea sin partir: %d caracteres", utf8.RuneCountInString(l))
		}
	}
}

func TestPaintCellColorsGainsAndLossesOnly(t *testing.T) {
	if got := paintCell("+1.0%", " +1.0%"); !strings.Contains(got, "\033[32m") {
		t.Errorf("ganancia sin verde: %q", got)
	}
	if got := paintCell("-1.0%", " -1.0%"); !strings.Contains(got, "\033[31m") {
		t.Errorf("pérdida sin rojo: %q", got)
	}
	for _, plain := range []string{"+0.0%", "n/d", "AAPL", "12.5%"} {
		if got := paintCell(plain, plain); got != plain {
			t.Errorf("%q no debía pintarse: %q", plain, got)
		}
	}
}

func TestEtfDropsTheNameColumn(t *testing.T) {
	buf := capture(t)
	headers := []string{"#", "Ticker", "Nombre", "Categoría", "Score"}
	rows := [][]string{{"1", "VTI", "Total Stock Market", "Amplio", "1.50"}}
	Etf("2024-08-30", []int{3, 12}, 5, 1, headers, rows, []string{"nota", ""}, false, "VUSB")
	out := buf.String()
	if strings.Contains(out, "Total Stock Market") || !strings.Contains(out, "Amplio") || !strings.Contains(out, "exceso sobre VUSB") ||
		!strings.Contains(out, "3, 12 meses") || !strings.Contains(out, "\nnota\n") {
		t.Fatalf("salida:\n%s", out)
	}
}

func TestBacktestPrintsTheBaseCalendar(t *testing.T) {
	buf := capture(t)
	term := model.Term{Months: 4, Calendars: []model.Calendar{{Offset: 0, Result: model.Result{
		History:      []model.Rebalance{{Date: "2024-01-01", Picks: []model.Pick{{Ticker: "AAA", Return: 0.5}}}},
		FinalCapital: 1210, TotalReturnPct: 21, CAGRPct: 30}}}}
	Backtest(Run{Years: 5, InitialCapital: 1000, TopN: 1}, term, 0)
	out := buf.String()
	for _, want := range []string{"RESULTADOS BACKTEST - Top 1 (Point-in-Time)", "Plazo:               4 meses", "Capital final:       US$ 1,210.00",
		"CAGR mediana:        +30.0%   (rango +30.0% a +30.0%, 1 calendarios)", "Historial de selecciones"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en:\n%s", want, out)
		}
	}
}
