package etfq

import (
	"sort"
)

// Calidad de los ETFs: costos (TER) y liquidez (AUM).
//
// El ranking de momentum dice qué ETFs subieron más por unidad de riesgo; la calidad dice cuáles son baratos y grandes.
// Cada ETF recibe la posición (0-100) de su TER (menos es mejor) y de su AUM (más es mejor) entre los ETFs de su misma
// categoría (peer group): comparar el costo de un ETF de bonos con el de uno de acciones sectoriales no tiene sentido. Se usan
// posiciones y no Z-scores porque unos pocos ETFs gigantes (VOO, VTI) comprimirían a todos los demás. La calidad es el promedio
// ponderado de las dos posiciones y el score final mezcla la posición de momentum (entre todos los elegibles, como el ranking
// global) con la calidad: final = (1 - peso) x momentum + peso x calidad.
//
// Antes de puntuar se aplica un filtro de elegibilidad: un TER por encima del máximo o un AUM por debajo del mínimo dejan al
// ETF fuera del ranking final. Un dato que falta no excluye: ese factor simplemente no cuenta.

// Motivos de exclusión de QualityRow.Excluded.
const (
	ExcludedTER = "ter"
	ExcludedAUM = "aum"
)

// QualityInput es un ETF del ranking de momentum con sus datos de costo y tamaño (nil = sin dato). TER en % anual y AUM en US$.
type QualityInput struct {
	Ticker   string
	Category string
	Score    float64 // score de momentum
	TER      *float64
	AUM      *float64
}

// QualityParams son los parámetros del puntaje de calidad.
type QualityParams struct {
	MaxTER    float64 // % anual; 0 = sin tope
	MinAUM    float64 // US$; 0 = sin mínimo
	Weight    float64 // peso de la calidad en el score final (0-1)
	TERWeight float64 // peso del TER dentro de la calidad
	AUMWeight float64 // peso del AUM dentro de la calidad
}

// QualityRow es el resultado de un ETF. Las posiciones van de 0 a 100; nil si faltan los datos para calcularlas.
type QualityRow struct {
	Ticker   string
	Rank     int // posición por score final; 0 = sin score final
	TERPct   *float64
	AUMPct   *float64
	Quality  *float64
	Momentum *float64
	Final    *float64
	Excluded []string
}

// midRankPercentiles es la posición (0-100) de cada valor entre todos: los peores más la mitad de los iguales (él incluido),
// sobre el total. Con higherBetter false, el menor valor es el mejor.
func midRankPercentiles(values []float64, higherBetter bool) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		worse, same := 0.0, 0.0
		for _, w := range values {
			switch {
			case w == v:
				same++
			case (w < v) == higherBetter:
				worse++
			}
		}
		out[i] = (worse + same/2) / float64(len(values)) * 100
	}
	return out
}

// ScoreQuality calcula calidad y score final de los ETFs, en el mismo orden que in (el resultado también).
func ScoreQuality(in []QualityInput, p QualityParams) []QualityRow {
	rows := make([]QualityRow, len(in))
	var eligible []int
	for i, e := range in {
		rows[i].Ticker = e.Ticker
		if e.TER != nil && p.MaxTER > 0 && *e.TER > p.MaxTER {
			rows[i].Excluded = append(rows[i].Excluded, ExcludedTER)
		}
		if e.AUM != nil && p.MinAUM > 0 && *e.AUM < p.MinAUM {
			rows[i].Excluded = append(rows[i].Excluded, ExcludedAUM)
		}
		if len(rows[i].Excluded) == 0 {
			eligible = append(eligible, i)
		}
	}

	// Momentum: posición entre todos los elegibles
	scores := make([]float64, len(eligible))
	for k, i := range eligible {
		scores[k] = in[i].Score
	}
	for k, pct := range midRankPercentiles(scores, true) {
		rows[eligible[k]].Momentum = new(pct)
	}

	// TER y AUM: posición dentro de la categoría
	groups := map[string][]int{}
	for _, i := range eligible {
		groups[in[i].Category] = append(groups[in[i].Category], i)
	}
	for _, group := range groups {
		percentilesOf(group, rows, func(i int) *float64 { return in[i].TER }, false, func(r *QualityRow, v float64) { r.TERPct = &v })
		percentilesOf(group, rows, func(i int) *float64 { return in[i].AUM }, true, func(r *QualityRow, v float64) { r.AUMPct = &v })
	}

	tw, aw := p.TERWeight, p.AUMWeight
	if tw < 0 || aw < 0 || tw+aw == 0 {
		tw, aw = 1, 1
	}
	w := min(max(p.Weight, 0), 1)
	var ranked []int
	for _, i := range eligible {
		r := &rows[i]
		var sum, weights float64
		if r.TERPct != nil {
			sum, weights = sum+tw**r.TERPct, weights+tw
		}
		if r.AUMPct != nil {
			sum, weights = sum+aw**r.AUMPct, weights+aw
		}
		if weights == 0 {
			continue // sin TER ni AUM no hay calidad que mezclar
		}
		r.Quality = new(sum / weights)
		r.Final = new((1-w)**r.Momentum + w**r.Quality)
		ranked = append(ranked, i)
	}
	sort.SliceStable(ranked, func(a, b int) bool { return *rows[ranked[a]].Final > *rows[ranked[b]].Final })
	for pos, i := range ranked {
		rows[i].Rank = pos + 1
	}
	return rows
}

// percentilesOf pone en cada fila del grupo con dato la posición de su valor entre los del grupo.
func percentilesOf(group []int, rows []QualityRow, get func(int) *float64, higherBetter bool, set func(*QualityRow, float64)) {
	var with []int
	var values []float64
	for _, i := range group {
		if v := get(i); v != nil {
			with = append(with, i)
			values = append(values, *v)
		}
	}
	for k, pct := range midRankPercentiles(values, higherBetter) {
		set(&rows[with[k]], pct)
	}
}
