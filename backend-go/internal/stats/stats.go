// Package stats tiene las estadísticas que usan el ranking de ETFs y el modelo por score: media y desvío
// muestral, rankings porcentuales con empates promediados y manejo explícito de NaN.
//
// Fecha: 2026-11-19
package stats

import (
	"math"
	"sort"
)

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
		avg := float64(i+1+j) / 2
		for k := i; k < j; k++ {
			out[idx[k]] = avg / n
		}
		i = j
	}
	return out
}

func Sum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

func Clip(x, lo, hi float64) float64 { return math.Min(math.Max(x, lo), hi) }
