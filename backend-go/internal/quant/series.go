// Package quant es el modelo por score: score de momentum, de salud financiera y técnico de las acciones del S&P 500, su
// promedio como score final y Markowitz sobre las mejores. No hay backtest: es una foto de hoy.
package quant

import "math"

var nan = math.NaN()

func isNaN(x float64) bool { return x != x }

// Las funciones de este archivo calculan, sobre una serie de precios, diferencias, desfases, medias exponenciales y
// ventanas móviles: los NaN se propagan igual y las ventanas exigen todas sus observaciones (min_periods = ventana).

// diff es x[i] - x[i-1]; el primero es NaN.
func diff(x []float64) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		if i == 0 {
			out[i] = nan
		} else {
			out[i] = x[i] - x[i-1]
		}
	}
	return out
}

// shift corre la serie n posiciones hacia adelante (los primeros n quedan en NaN).
func shift(x []float64, n int) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		if i-n >= 0 {
			out[i] = x[i-n]
		} else {
			out[i] = nan
		}
	}
	return out
}

// ewm es la media exponencial (alpha, sin ajuste, minp observaciones mínimas) con este manejo de los NaN: un NaN no
// actualiza el promedio pero sí hace perder peso al anterior (ignore_na=False).
func ewm(x []float64, alpha float64, minp int) []float64 {
	out := make([]float64, len(x))
	if len(x) == 0 {
		return out
	}
	oldWtFactor, newWt := 1-alpha, alpha
	weighted := x[0]
	nobs := 0
	if !isNaN(weighted) {
		nobs = 1
	}
	out[0] = nan
	if nobs >= minp {
		out[0] = weighted
	}
	oldWt := 1.0
	for i := 1; i < len(x); i++ {
		cur := x[i]
		isObs := !isNaN(cur)
		if isObs {
			nobs++
		}
		if !isNaN(weighted) {
			oldWt *= oldWtFactor
			if isObs {
				if weighted != cur { // evita errores numéricos en series constantes
					weighted = (oldWt*weighted + newWt*cur) / (oldWt + newWt)
				}
				oldWt = 1
			}
		} else if isObs {
			weighted = cur
		}
		if nobs >= minp {
			out[i] = weighted
		} else {
			out[i] = nan
		}
	}
	return out
}

// emaSpan es ewm(span=span, adjust=False, min_periods=span).mean().
func emaSpan(x []float64, span int) []float64 { return ewm(x, 2/float64(span+1), span) }

// wilder es la media de Wilder (la que usan RSI, ADX y MFI): ewm con alpha = 1/n.
func wilder(x []float64, n int) []float64 { return ewm(x, 1/float64(n), n) }

// rolling aplica f a cada ventana de n valores; si la ventana tiene algún NaN (o es incompleta) el resultado es NaN.
func rolling(x []float64, n int, f func(w []float64) float64) []float64 {
	out := make([]float64, len(x))
	for i := range x {
		out[i] = nan
		if i+1 < n {
			continue
		}
		w := x[i+1-n : i+1]
		ok := true
		for _, v := range w {
			if isNaN(v) {
				ok = false
				break
			}
		}
		if ok {
			out[i] = f(w)
		}
	}
	return out
}

func rollingMean(x []float64, n int) []float64 {
	return rolling(x, n, func(w []float64) float64 {
		s := 0.0
		for _, v := range w {
			s += v
		}
		return s / float64(len(w))
	})
}

func rollingSum(x []float64, n int) []float64 {
	return rolling(x, n, func(w []float64) float64 {
		s := 0.0
		for _, v := range w {
			s += v
		}
		return s
	})
}

func rollingStd(x []float64, n int) []float64 {
	return rolling(x, n, func(w []float64) float64 {
		m := 0.0
		for _, v := range w {
			m += v
		}
		m /= float64(len(w))
		s := 0.0
		for _, v := range w {
			s += (v - m) * (v - m)
		}
		return math.Sqrt(s / float64(len(w)-1))
	})
}

func rollingMin(x []float64, n int) []float64 {
	return rolling(x, n, func(w []float64) float64 {
		m := w[0]
		for _, v := range w {
			m = math.Min(m, v)
		}
		return m
	})
}

func rollingMax(x []float64, n int) []float64 {
	return rolling(x, n, func(w []float64) float64 {
		m := w[0]
		for _, v := range w {
			m = math.Max(m, v)
		}
		return m
	})
}

// zip2 aplica f elemento a elemento a dos series del mismo largo.
func zip2(a, b []float64, f func(a, b float64) float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = f(a[i], b[i])
	}
	return out
}

func mapf(a []float64, f func(float64) float64) []float64 {
	out := make([]float64, len(a))
	for i, v := range a {
		out[i] = f(v)
	}
	return out
}

// clipLower y clipUpper son Series.clip con un solo extremo; el NaN queda en NaN.
func clipLower(a []float64, lo float64) []float64 {
	return mapf(a, func(v float64) float64 {
		if v < lo {
			return lo
		}
		return v
	})
}

func clipBoth(v, lo, hi float64) float64 {
	if isNaN(v) {
		return v
	}
	return math.Min(math.Max(v, lo), hi)
}

// whereCond conserva a[i] donde cond[i] es verdadero y pone other en el resto (Series.where(cond, other)).
func whereCond(a []float64, cond []bool, other float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		if cond[i] {
			out[i] = a[i]
		} else {
			out[i] = other
		}
	}
	return out
}

// cumsum es Series.cumsum(): los NaN siguen siendo NaN pero no cortan la suma.
func cumsum(x []float64) []float64 {
	out := make([]float64, len(x))
	s := 0.0
	for i, v := range x {
		if isNaN(v) {
			out[i] = nan
			continue
		}
		s += v
		out[i] = s
	}
	return out
}
