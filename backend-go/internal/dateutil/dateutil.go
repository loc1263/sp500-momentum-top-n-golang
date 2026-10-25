// Package dateutil tiene la aritmética de fechas calendario que usa el resto (fechas sin hora, en UTC).
package dateutil

import "time"

const Layout = "2006-01-02"

// Day devuelve la fecha como "YYYY-MM-DD".
func Day(t time.Time) string { return t.Format(Layout) }

// Parse lee "YYYY-MM-DD" (o un timestamp que empiece así) como medianoche UTC.
func Parse(s string) (time.Time, error) {
	if len(s) > len(Layout) {
		s = s[:len(Layout)]
	}
	return time.Parse(Layout, s)
}

// MustParse es Parse que entra en pánico; para constantes y tests.
func MustParse(s string) time.Time {
	t, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

// Date es la medianoche UTC del día y/m/d.
func Date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// Today es hoy (fecha local) como medianoche UTC.
func Today() time.Time {
	n := time.Now()
	return Date(n.Year(), n.Month(), n.Day())
}

// Midnight lleva t a la medianoche UTC de su fecha calendario.
func Midnight(t time.Time) time.Time { return Date(t.Year(), t.Month(), t.Day()) }

// AddMonths suma n meses sin pasarse de fin de mes (31 de enero + 1 mes = 28/29 de febrero), como pd.DateOffset(months=n).
func AddMonths(t time.Time, n int) time.Time {
	total := t.Year()*12 + int(t.Month()) - 1 + n
	y, m := total/12, total%12
	if m < 0 {
		m += 12
		y--
	}
	last := daysIn(y, time.Month(m+1))
	d := min(t.Day(), last)
	return time.Date(y, time.Month(m+1), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

// AddYears suma n años con el mismo recorte de fin de mes (29 de febrero -> 28).
func AddYears(t time.Time, n int) time.Time { return AddMonths(t, 12*n) }

// AddDays suma n días.
func AddDays(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) }

func daysIn(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

// DaysBetween son los días enteros de a hasta b.
func DaysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }

// MonthStarts son las fechas de pd.date_range(start, end, freq="{step}MS"): el primer inicio de mes que no es anterior
// a `start`, y desde ahí uno cada `step` meses mientras no pasen de `end`.
func MonthStarts(start, end time.Time, step int) []time.Time {
	first := Date(start.Year(), start.Month(), 1)
	if first.Before(start) {
		first = AddMonths(first, 1)
	}
	var out []time.Time
	for d, i := first, 0; !d.After(end); i++ {
		out = append(out, d)
		d = AddMonths(first, step*(i+1))
	}
	return out
}

// Days es la cantidad de días enteros (365,25 = un año).
func Years(a, b time.Time) float64 { return float64(DaysBetween(a, b)) / 365.25 }
