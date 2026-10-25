package quant

import (
	"errors"
	"math"
)

// qp es un programa cuadrático convexo:  min ½ xᵀHx + cᵀx  sujeto a  Ax = b,  Gx ≤ h.
type qp struct {
	H [][]float64 // n x n, simétrica semidefinida positiva
	c []float64
	A [][]float64 // p x n (puede ser vacía)
	b []float64
	G [][]float64 // m x n
	h []float64
}

var errNoConverge = errors.New("el optimizador no convergió")

// solveLinear resuelve M x = rhs por eliminación gaussiana con pivoteo parcial (los sistemas son de decenas de filas).
func solveLinear(m [][]float64, rhs []float64) ([]float64, error) {
	n := len(rhs)
	a := make([][]float64, n)
	for i := range a {
		a[i] = append(append([]float64(nil), m[i]...), rhs[i])
	}
	for col := 0; col < n; col++ {
		piv := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[piv][col]) {
				piv = r
			}
		}
		if math.Abs(a[piv][col]) < 1e-300 {
			return nil, errors.New("sistema singular")
		}
		a[col], a[piv] = a[piv], a[col]
		for r := col + 1; r < n; r++ {
			f := a[r][col] / a[col][col]
			if f == 0 {
				continue
			}
			for k := col; k <= n; k++ {
				a[r][k] -= f * a[col][k]
			}
		}
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := a[i][n]
		for k := i + 1; k < n; k++ {
			s -= a[i][k] * x[k]
		}
		x[i] = s / a[i][i]
	}
	return x, nil
}

func matVec(m [][]float64, x []float64) []float64 {
	out := make([]float64, len(m))
	for i, row := range m {
		s := 0.0
		for j, v := range row {
			s += v * x[j]
		}
		out[i] = s
	}
	return out
}

func matTVec(m [][]float64, y []float64, n int) []float64 {
	out := make([]float64, n)
	for i, row := range m {
		for j, v := range row {
			out[j] += v * y[i]
		}
	}
	return out
}

func infNorm(x []float64) float64 {
	m := 0.0
	for _, v := range x {
		m = math.Max(m, math.Abs(v))
	}
	return m
}

func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// maxStep es el mayor paso α ∈ (0, 1] con v + α dv ≥ 0.
func maxStep(v, dv []float64) float64 {
	alpha := 1.0
	for i := range v {
		if dv[i] < 0 {
			alpha = math.Min(alpha, -v[i]/dv[i])
		}
	}
	return alpha
}

// solve resuelve el programa con un método de puntos interiores primal-dual (predictor-corrector de Mehrotra) que parte de x0
// (no hace falta que sea factible). Devuelve el óptimo con tolerancia ≈ 1e-10 o un error si no converge (problema infactible).
func (q *qp) solve(x0 []float64) ([]float64, error) {
	n, p, m := len(q.c), len(q.A), len(q.G)
	x := append([]float64(nil), x0...)
	y := make([]float64, p)
	s, z := make([]float64, m), make([]float64, m)
	gx := matVec(q.G, x)
	for i := range s {
		s[i] = math.Max(q.h[i]-gx[i], 1)
		z[i] = 1
	}
	scale := 1 + infNorm(q.c)
	for iter := 0; iter < 200; iter++ {
		hx := matVec(q.H, x)
		rd := make([]float64, n)
		aty, gtz := matTVec(q.A, y, n), matTVec(q.G, z, n)
		for i := range rd {
			rd[i] = hx[i] + q.c[i] + aty[i] + gtz[i]
		}
		re := matVec(q.A, x)
		for i := range re {
			re[i] -= q.b[i]
		}
		gx = matVec(q.G, x)
		ri := make([]float64, m)
		for i := range ri {
			ri[i] = gx[i] + s[i] - q.h[i]
		}
		mu := dot(s, z) / float64(m)
		if infNorm(rd) < 1e-11*scale && infNorm(re) < 1e-11 && infNorm(ri) < 1e-11 && mu < 1e-13 {
			return x, nil
		}

		// matriz de Newton reducida: [H + GᵀDG, Aᵀ; A, 0]
		size := n + p
		K := make([][]float64, size)
		for i := range K {
			K[i] = make([]float64, size)
		}
		for i := 0; i < n; i++ {
			copy(K[i][:n], q.H[i])
			K[i][i] += 1e-13
		}
		for r := 0; r < m; r++ {
			d := z[r] / s[r]
			for i := 0; i < n; i++ {
				if q.G[r][i] == 0 {
					continue
				}
				for j := 0; j < n; j++ {
					K[i][j] += d * q.G[r][i] * q.G[r][j]
				}
			}
		}
		for k := 0; k < p; k++ {
			for j := 0; j < n; j++ {
				K[n+k][j] = q.A[k][j]
				K[j][n+k] = q.A[k][j]
			}
		}
		newton := func(rc []float64) (dx, dy, ds, dz []float64, err error) {
			rhs := make([]float64, size)
			t := make([]float64, m)
			for r := range t {
				t[r] = rc[r]/s[r] - z[r]/s[r]*ri[r]
			}
			gt := matTVec(q.G, t, n)
			for i := 0; i < n; i++ {
				rhs[i] = -rd[i] + gt[i]
			}
			for k := 0; k < p; k++ {
				rhs[n+k] = -re[k]
			}
			sol, err := solveLinear(K, rhs)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dx, dy = sol[:n], sol[n:]
			gdx := matVec(q.G, dx)
			ds, dz = make([]float64, m), make([]float64, m)
			for r := 0; r < m; r++ {
				ds[r] = -ri[r] - gdx[r]
				dz[r] = -(rc[r] + z[r]*ds[r]) / s[r]
			}
			return dx, dy, ds, dz, nil
		}

		rc := make([]float64, m)
		for r := range rc {
			rc[r] = s[r] * z[r]
		}
		_, _, dsA, dzA, err := newton(rc)
		if err != nil {
			return nil, err
		}
		alpha := math.Min(maxStep(s, dsA), maxStep(z, dzA))
		muAff := 0.0
		for r := 0; r < m; r++ {
			muAff += (s[r] + alpha*dsA[r]) * (z[r] + alpha*dzA[r])
		}
		muAff /= float64(m)
		sigma := math.Pow(muAff/mu, 3)
		for r := range rc {
			rc[r] = s[r]*z[r] + dsA[r]*dzA[r] - sigma*mu
		}
		dx, dy, ds, dz, err := newton(rc)
		if err != nil {
			return nil, err
		}
		tau := math.Max(0.99, 1-mu)
		alpha = math.Min(1, tau*math.Min(maxStep(s, ds), maxStep(z, dz)))
		for i := range x {
			x[i] += alpha * dx[i]
		}
		for i := range y {
			y[i] += alpha * dy[i]
		}
		for r := range s {
			s[r] += alpha * ds[r]
			z[r] += alpha * dz[r]
		}
		for _, v := range x {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, errNoConverge
			}
		}
	}
	return nil, errNoConverge
}
