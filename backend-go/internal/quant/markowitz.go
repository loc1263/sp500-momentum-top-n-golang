package quant

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"top3/internal/model"
	"top3/internal/stats"
)

// Markowitz sobre unas pocas acciones: tres carteras de la frontera eficiente (sin posiciones cortas, con un máximo por acción
// y por sector).
//
//   - Máximo retorno: el mayor retorno esperado con volatilidad anual de hasta vol_target.
//   - Máximo Sharpe: la mejor relación entre el exceso de retorno sobre la tasa libre de riesgo y la volatilidad.
//   - Mínima varianza: la de menor volatilidad posible.
//
// Entradas: el retorno esperado de cada acción es su retorno anual de los últimos 252 días (limitado a ±100%) encogido hacia
// un retorno de largo plazo (prior = tasa libre de riesgo + prima de mercado), con mu_shrink como peso del dato propio: el
// retorno pasado de una sola acción es muy ruidoso y el de las líderes de momentum, desmesurado. La covarianza es la de
// Ledoit-Wolf sobre esos mismos 252 días. Los pesos salen sobre todo del riesgo y, en menor medida, del retorno pasado.
//
// Cada cartera es la solución exacta de un programa cuadrático convexo (puntos interiores): el mínimo de varianza directamente,
// el máximo Sharpe con el cambio de variable de Charnes-Cooper y el máximo retorno acotado por riesgo con el multiplicador de
// la restricción de volatilidad hallado por bisección.

// ledoitWolf es la covarianza de Ledoit-Wolf de las filas de x (observaciones x variables), centrada en la media.
func ledoitWolf(x [][]float64) [][]float64 {
	nSamples, nFeatures := len(x), len(x[0])
	mean := make([]float64, nFeatures)
	for _, row := range x {
		for j, v := range row {
			mean[j] += v / float64(nSamples)
		}
	}
	xc := make([][]float64, nSamples)
	for i, row := range x {
		xc[i] = make([]float64, nFeatures)
		for j, v := range row {
			xc[i][j] = v - mean[j]
		}
	}
	emp := make([][]float64, nFeatures)
	for i := range emp {
		emp[i] = make([]float64, nFeatures)
	}
	for _, row := range xc {
		for i := 0; i < nFeatures; i++ {
			for j := 0; j < nFeatures; j++ {
				emp[i][j] += row[i] * row[j] / float64(nSamples)
			}
		}
	}
	shrinkage := 0.0
	if nFeatures > 1 {
		empTrace := make([]float64, nFeatures) // suma de X² por columna / n
		for _, row := range xc {
			for j, v := range row {
				empTrace[j] += v * v / float64(nSamples)
			}
		}
		mu := stats.Sum(empTrace) / float64(nFeatures)
		beta, delta := 0.0, 0.0 // suma de <X²ᵀ, X²> y suma de los cuadrados de <Xᵀ, X>
		for i := 0; i < nFeatures; i++ {
			for j := 0; j < nFeatures; j++ {
				var b, d float64
				for _, row := range xc {
					b += row[i] * row[i] * row[j] * row[j]
					d += row[i] * row[j]
				}
				beta += b
				delta += d * d
			}
		}
		delta /= float64(nSamples) * float64(nSamples)
		b := 1 / (float64(nFeatures) * float64(nSamples)) * (beta/float64(nSamples) - delta)
		d := delta - 2*mu*stats.Sum(empTrace) + float64(nFeatures)*mu*mu
		d /= float64(nFeatures)
		b = math.Min(b, d)
		if b != 0 {
			shrinkage = b / d
		}
	}
	trace := 0.0
	for i := range emp {
		trace += emp[i][i]
	}
	mu := trace / float64(nFeatures)
	out := make([][]float64, nFeatures)
	for i := range out {
		out[i] = make([]float64, nFeatures)
		for j := range out[i] {
			out[i][j] = (1 - shrinkage) * emp[i][j]
		}
		out[i][i] += shrinkage * mu
	}
	return out
}

// Inputs son (retorno esperado anual, covarianza anual) de las columnas de prices (últimas 252 ruedas, sin huecos).
func Inputs(prices *model.Prices, muShrink, prior float64) (mu []float64, sigma [][]float64) {
	w := prices.Tail(DayYear + 1)
	n := prices.Cols()
	rows := w.Rows() - 1
	returns := make([][]float64, rows)
	for i := range returns {
		returns[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			returns[i][j] = w.At(i+1, j)/w.At(i, j) - 1
		}
	}
	mu = make([]float64, n)
	for j := 0; j < n; j++ {
		col := make([]float64, rows)
		for i := range col {
			col[i] = returns[i][j]
		}
		mean := stats.MeanSkipNaN(col) * DayYear
		mu[j] = muShrink*stats.Clip(mean, -1, 1) + (1-muShrink)*prior
	}
	sigma = ledoitWolf(returns)
	for i := range sigma {
		for j := range sigma[i] {
			sigma[i][j] *= DayYear
		}
	}
	return mu, sigma
}

// FeasibleSectorCap es sectorMax, o el menor tope mayor que él con el que se puede invertir todo: con pocos sectores (o
// sectores con pocas acciones y wMax) el tope pedido puede no dejar colocar el 100%.
func FeasibleSectorCap(sectors []string, wMax, sectorMax float64) float64 {
	count := map[string]int{}
	for _, s := range sectors {
		count[s]++
	}
	capacity := func(c float64) float64 {
		total := 0.0
		for _, size := range count {
			total += math.Min(c, float64(size)*wMax)
		}
		return total
	}
	c := sectorMax
	for capacity(c) < 1-1e-9 && c < 1 {
		c = math.Min(c+0.01, 1.0)
	}
	return c
}

// Weight es el peso de una acción en una cartera.
type Weight struct {
	Ticker string  `json:"ticker"`
	Weight float64 `json:"weight"`
}

// SectorWeight es el peso de un sector en una cartera.
type SectorWeight struct {
	Sector string  `json:"sector"`
	Weight float64 `json:"weight"`
}

// Portfolio es una cartera de la frontera eficiente.
type Portfolio struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	ExpReturn float64        `json:"exp_return"`
	Vol       float64        `json:"vol"`
	Sharpe    float64        `json:"sharpe"`
	Weights   []Weight       `json:"weights"`
	Sectors   []SectorWeight `json:"sectors"`
}

// Frontier son las carteras y los límites con los que se armaron.
type Frontier struct {
	Portfolios    []Portfolio `json:"portfolios"`
	MinVol        float64     `json:"min_vol"`
	VolTarget     float64     `json:"vol_target"`
	WMax          float64     `json:"w_max"`
	SectorMax     float64     `json:"sector_max"`
	SectorRelaxed bool        `json:"sector_relaxed"`
}

func quadForm(cov [][]float64, w []float64) float64 { return dot(w, matVec(cov, w)) }

func volOf(cov [][]float64, w []float64) float64 { return math.Sqrt(math.Max(quadForm(cov, w), 0)) }

// limits arma las restricciones lineales de las carteras: pesos entre 0 y wMax y, si hay tope, suma por sector <= sectorMax.
// Con homog = true son las del problema escalado de Charnes-Cooper (y_i <= wMax·Σy, y_sector <= sectorMax·Σy).
func limits(n int, wMax, sectorMax float64, groups [][]int, homog bool) (G [][]float64, h []float64) {
	row := func() []float64 { return make([]float64, n) }
	for i := 0; i < n; i++ {
		lo := row()
		lo[i] = -1
		G, h = append(G, lo), append(h, 0)
		hi := row()
		hi[i] = 1
		if homog {
			for j := range hi {
				hi[j] -= wMax
			}
			G, h = append(G, hi), append(h, 0)
		} else {
			G, h = append(G, hi), append(h, wMax)
		}
	}
	if sectorMax < 1 {
		for _, idx := range groups {
			r := row()
			for _, i := range idx {
				r[i] = 1
			}
			if homog {
				for j := range r {
					r[j] -= sectorMax
				}
				G, h = append(G, r), append(h, 0)
			} else {
				G, h = append(G, r), append(h, sectorMax)
			}
		}
	}
	return G, h
}

func uniform(n int) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = 1 / float64(n)
	}
	return x
}

func ones(n int) [][]float64 {
	r := make([]float64, n)
	for i := range r {
		r[i] = 1
	}
	return [][]float64{r}
}

// normalize deja los pesos no negativos y con suma 1.
func normalize(w []float64) []float64 {
	out := make([]float64, len(w))
	sum := 0.0
	for i, v := range w {
		out[i] = math.Max(v, 0)
		sum += out[i]
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

// Optimize arma las carteras (máximo retorno con vol <= volTarget, máximo Sharpe y mínima varianza) con sus pesos, retorno
// esperado, volatilidad y Sharpe (y su peso por sector), más MinVol, la menor volatilidad posible. Si volTarget es menor que
// MinVol, no hay cartera de máximo retorno y se omite. Con menos acciones que 1/wMax, o sectores que no alcanzan para colocar
// todo con sectorMax, el tope se sube lo mínimo necesario (SectorMax en el resultado). tickers y sectors van alineados con mu.
func Optimize(tickers []string, mu []float64, sigma [][]float64, volTarget, wMax, riskFree float64, sectors []string, sectorMax float64) (Frontier, error) {
	n := len(mu)
	wMax = math.Max(wMax, 1/float64(n)+1e-9)
	if sectors == nil {
		sectors = make([]string, n)
		for i := range sectors {
			sectors[i] = "todas"
		}
	}
	requested := sectorMax
	sectorMax = FeasibleSectorCap(sectors, wMax, sectorMax)
	bySector := map[string][]int{}
	var order []string
	for i, s := range sectors {
		if _, ok := bySector[s]; !ok {
			order = append(order, s)
		}
		bySector[s] = append(bySector[s], i)
	}
	groups := make([][]int, len(order))
	for k, s := range order {
		groups[k] = bySector[s]
	}

	G, h := limits(n, wMax, sectorMax, groups, false)
	eq, one := ones(n), []float64{1}
	start := uniform(n)

	// mínima varianza
	H := make([][]float64, n)
	for i := range H {
		H[i] = make([]float64, n)
		for j := range H[i] {
			H[i][j] = 2 * sigma[i][j]
		}
	}
	minVarW, err := (&qp{H: H, c: make([]float64, n), A: eq, b: one, G: G, h: h}).solve(start)
	if err != nil {
		return Frontier{}, fmt.Errorf("mínima varianza: %w", err)
	}
	minVar := normalize(minVarW)
	minVol := volOf(sigma, minVar)

	// máximo Sharpe: Charnes-Cooper. min yᵀΣy con aᵀy = 1 (a = mu - rf), y >= 0 y los topes escalados por Σy; w = y / Σy
	var maxSharpe []float64
	a := make([]float64, n)
	for i := range a {
		a[i] = mu[i] - riskFree
	}
	Gh, hh := limits(n, wMax, sectorMax, groups, true)
	if y, err := (&qp{H: H, c: make([]float64, n), A: [][]float64{a}, b: one, G: Gh, h: hh}).solve(start); err == nil {
		if sum := stats.Sum(y); sum > 0 {
			maxSharpe = normalize(y)
		}
	}

	// máximo retorno con volatilidad <= volTarget: max μᵀw - λ wᵀΣw, con λ el menor que cumple el tope
	var maxReturn []float64
	if minVol <= volTarget {
		atLambda := func(lambda float64) ([]float64, error) {
			HL := make([][]float64, n)
			for i := range HL {
				HL[i] = make([]float64, n)
				for j := range HL[i] {
					HL[i][j] = 2 * lambda * sigma[i][j]
				}
			}
			c := make([]float64, n)
			for i := range c {
				c[i] = -mu[i]
			}
			w, err := (&qp{H: HL, c: c, A: eq, b: one, G: G, h: h}).solve(minVar)
			if err != nil {
				return nil, err
			}
			return normalize(w), nil
		}
		lo, hi := 1e-9, 1.0
		w, err := atLambda(lo)
		if err == nil && volOf(sigma, w) <= volTarget {
			maxReturn = w
		} else if err == nil {
			var wHi []float64
			for hi < 1e12 {
				if wHi, err = atLambda(hi); err != nil || volOf(sigma, wHi) <= volTarget {
					break
				}
				lo, hi = hi, hi*4
			}
			if err == nil && wHi != nil && volOf(sigma, wHi) <= volTarget {
				for k := 0; k < 200 && hi/lo > 1+1e-13; k++ {
					mid := math.Sqrt(lo * hi)
					wm, err := atLambda(mid)
					if err != nil {
						break
					}
					if volOf(sigma, wm) <= volTarget {
						hi, wHi = mid, wm
					} else {
						lo = mid
					}
				}
				maxReturn = wHi
			}
		}
	}

	var portfolios []Portfolio
	for _, c := range []struct {
		key, label string
		w          []float64
	}{
		{"max_return", fmt.Sprintf("Máximo retorno (volatilidad hasta %.0f%%)", volTarget*100), maxReturn},
		{"max_sharpe", "Máximo Sharpe", maxSharpe},
		{"min_var", "Mínima varianza", minVar},
	} {
		if c.w == nil {
			continue
		}
		ret, risk := dot(mu, c.w), volOf(sigma, c.w)
		p := Portfolio{Key: c.key, Label: c.label, ExpReturn: ret, Vol: risk, Sharpe: (ret - riskFree) / risk}
		for i, w := range c.w {
			if w > 1e-4 {
				p.Weights = append(p.Weights, Weight{tickers[i], w})
			}
		}
		sort.SliceStable(p.Weights, func(a, b int) bool { return p.Weights[a].Weight > p.Weights[b].Weight })
		sum := map[string]float64{}
		for i, w := range c.w {
			sum[sectors[i]] += w
		}
		for _, s := range order {
			if sum[s] > 1e-4 {
				p.Sectors = append(p.Sectors, SectorWeight{s, sum[s]})
			}
		}
		sort.SliceStable(p.Sectors, func(a, b int) bool { return p.Sectors[a].Weight > p.Sectors[b].Weight })
		portfolios = append(portfolios, p)
	}
	if len(portfolios) == 0 {
		return Frontier{}, errors.New("no se pudo armar ninguna cartera")
	}
	return Frontier{Portfolios: portfolios, MinVol: minVol, VolTarget: volTarget, WMax: wMax, SectorMax: sectorMax,
		SectorRelaxed: sectorMax > requested+1e-9}, nil
}
