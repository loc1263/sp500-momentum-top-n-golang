// Package dateutil tiene la aritmética de fechas calendario que usa el resto (fechas sin hora, en UTC).
//
// Fecha: 2026-11-19
package dateutil

import "time"

const Layout = "2006-01-02"

func Day(t time.Time) string { return t.Format(Layout) }

func Parse(s string) (time.Time, error) {
	if len(s) > len(Layout) {
		s = s[:len(Layout)]
	}
	return time.Parse(Layout, s)
}

// MustParse es Parse para fechas fijas (pruebas); entra en pánico si s no es válida.
func MustParse(s string) time.Time {
	t, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

func Date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func Today() time.Time {
	n := time.Now()
	return Date(n.Year(), n.Month(), n.Day())
}

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

func AddYears(t time.Time, n int) time.Time { return AddMonths(t, 12*n) }

func daysIn(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

func DaysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }

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

func Years(a, b time.Time) float64 { return float64(DaysBetween(a, b)) / 365.25 }
