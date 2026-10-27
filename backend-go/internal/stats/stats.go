// Package stats tiene las estadísticas que usan el ranking de ETFs y el modelo por score: media y desvío
// muestral, rankings porcentuales con empates promediados y manejo explícito de NaN.
package stats

import (
	"math"
	"sort"
)

// Mean es el promedio de xs; NaN si hay algún NaN o está vacío (como Series.mean(skipna=False)).
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// MeanSkipNaN es el promedio de los valores que no son NaN; NaN si no hay ninguno.
func MeanSkipNaN(xs []float64) float64 {
	s, n := 0.0, 0
	for _, x := range xs {
		if !math.IsNaN(x) {
			s += x
			n++
		}
	}
	if n == 0 {
		return math.NaN()
	}
	return s / float64(n)
}

// Std es el desvío estándar muestral (n-1) de xs; NaN con menos de dos valores.
func Std(xs []float64) float64 {
	n := len(xs)
	if n < 2 {
		return math.NaN()
	}
	m := Mean(xs)
	s := 0.0
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(n-1))
}

// PctChange son los retornos simples x[i]/x[i-1] - 1 (uno menos que la entrada).
func PctChange(xs []float64) []float64 {
	if len(xs) < 2 {
		return nil
	}
	out := make([]float64, len(xs)-1)
	for i := 1; i < len(xs); i++ {
		out[i-1] = xs[i]/xs[i-1] - 1
	}
	return out
}

// RankPct es Series.rank(pct=True, ascending=asc): el puesto de cada valor (empates promediados) dividido por la cantidad de
// valores que no son NaN. Los NaN quedan en NaN. Maneja infinitos como valores comunes.
func RankPct(vals []float64, ascending bool) []float64 {
	idx := make([]int, 0, len(vals))
	for i, v := range vals {
		if !math.IsNaN(v) {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if ascending {
			return vals[idx[a]] < vals[idx[b]]
		}
		return vals[idx[a]] > vals[idx[b]]
	})
	out := make([]float64, len(vals))
	for i := range out {
		out[i] = math.NaN()
	}
	n := float64(len(idx))
	for i := 0; i < len(idx); {
		j := i
		for j < len(idx) && vals[idx[j]] == vals[idx[i]] {
			j++
		}
		avg := float64(i+1+j) / 2 // puestos i+1 .. j promediados
		for k := i; k < j; k++ {
			out[idx[k]] = avg / n
		}
		i = j
	}
	return out
}

// Sum es la suma de xs.
func Sum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

// Clip limita x al rango [lo, hi].
func Clip(x, lo, hi float64) float64 { return math.Min(math.Max(x, lo), hi) }
