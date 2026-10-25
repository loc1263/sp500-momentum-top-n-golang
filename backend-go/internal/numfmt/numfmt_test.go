package numfmt

import (
	"math"
	"testing"
)

func TestComma(t *testing.T) {
	cases := []struct {
		x    float64
		d    int
		want string
	}{
		{1234567.891, 2, "1,234,567.89"},
		{999, 0, "999"},
		{1000, 0, "1,000"},
		{-12345.5, 1, "-12,345.5"},
		{0.004, 2, "0.00"},
		{-0.004, 2, "-0.00"}, // redondeo a cero con signo
		{15000, 2, "15,000.00"},
	}
	for _, c := range cases {
		if got := Comma(c.x, c.d); got != c.want {
			t.Errorf("Comma(%v, %d) = %q, quiero %q", c.x, c.d, got, c.want)
		}
	}
	if Comma(math.NaN(), 2) != "nan" {
		t.Error("NaN")
	}
}

func TestSignedAndPct(t *testing.T) {
	if Signed(1.25, 1) != "+1.2" && Signed(1.25, 1) != "+1.3" {
		t.Errorf("Signed = %s", Signed(1.25, 1))
	}
	if Pct(-2.04) != "-2.0%" || Pct(0) != "+0.0%" || Pct(12.34) != "+12.3%" {
		t.Errorf("Pct: %s %s %s", Pct(-2.04), Pct(0), Pct(12.34))
	}
	if Frac(0.1234, true) != "+12.3%" || Frac(0.1234, false) != "12.3%" || Frac(math.NaN(), true) != "n/d" {
		t.Error("Frac")
	}
	if G(4) != "4" || G(0.5) != "0.5" || G(200) != "200" {
		t.Error("G")
	}
}
