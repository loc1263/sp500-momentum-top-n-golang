// Pruebas del sitio estático: formato y color de celdas, y el HTML y los archivos que genera Build.
//
// Fecha: 2026-11-19

package site

import (
	"encoding/json"
	"testing"

	"top3/internal/visor"
)

func sample() visor.Payload {
	return visor.Payload{
		Date: "2026-11-18", GeneratedAt: "2026-11-18T20:29:17", Ref: "SPY",
		Sections: []visor.Section{
			{
				ID: visor.Stocks, Label: "Acciones",
				Cols: []string{"ticker", "nombre", "market_cap_B", "ret_1m", "sharpe", "vs_spy"},
				Rows: [][]any{
					{"BRK-B", "Berkshire <Hathaway> & Co", 5121.87, 3.89, 1.2, "SUPERA"},
					{"XYZ", "Xyz", 12345.4, -2.04, nil, "NO SUPERA"},
				},
				Fmt:    map[string]string{"market_cap_B": "int", "ret_1m": "pct", "sharpe": "dec"},
				Sign:   []string{"ret_1m"},
				Thr:    map[string][2]any{"sharpe": {">", 1.0}},
				Search: []int{0, 1},
			},
			{ID: visor.ETFs, Label: "ETF", Cols: []string{"ticker"}, Rows: [][]any{{"VOO"}}, Search: []int{0}},
		},
	}
}

func TestLabel(t *testing.T) {
	for col, want := range map[string]string{"ret_3m": "3m", "ret_5y": "5a", "sharpe": "Sharpe", "otro": "otro"} {
		if got := label(col); got != want {
			t.Errorf("label(%q) = %q, quiero %q", col, got, want)
		}
	}
}

func TestFormat(t *testing.T) {
	sec := sample().Sections[0]
	cases := []struct {
		col  string
		v    any
		want string
	}{
		{"ret_1m", -2.04, "-2.0%"},
		{"ret_1m", 3.89, "3.9%"},
		{"ret_1m", nil, ""},
		{"sharpe", 1.2, "1.20"},
		{"market_cap_B", 5121.87, "5122"},
		{"market_cap_B", 12345.4, "12.345"},
		{"market_cap_B", 1234567.0, "1.234.567"},
		{"nombre", "Apple", "Apple"},
	}
	for _, c := range cases {
		if got := format(sec, c.col, c.v); got != c.want {
			t.Errorf("format(%s, %v) = %q, quiero %q", c.col, c.v, got, c.want)
		}
	}
}

func TestCellClass(t *testing.T) {
	sec := sample().Sections[0]
	cases := []struct {
		col  string
		v    any
		want string
	}{
		{"ret_1m", 1.0, "pos"},
		{"ret_1m", -1.0, "neg"},
		{"ret_1m", 0.0, ""},
		{"sharpe", 1.2, "pos"},
		{"sharpe", 0.5, ""},
		{"sharpe", nil, ""},
		{"vs_spy", "SUPERA", "pos"},
		{"vs_spy", "NO SUPERA", "neg"},
		{"nombre", "x", ""},
	}
	for _, c := range cases {
		if got := cellClass(sec, c.col, c.v); got != c.want {
			t.Errorf("cellClass(%s, %v) = %q, quiero %q", c.col, c.v, got, c.want)
		}
	}
}

func writeVisor(t *testing.T, dir string, p visor.Payload) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return writeFile(t, dir, "visor.json", string(b))
}
