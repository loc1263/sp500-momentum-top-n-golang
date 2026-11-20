// Calcula el score de calidad de los ETFs con percentiles por categoría y lo mezcla con el momentum.
//
// Fecha: 2026-11-19

package etfq

import (
	"sort"
)

const (
	ExcludedTER = "ter"
	ExcludedAUM = "aum"
)

type QualityInput struct {
	Ticker   string
	Category string
	Score    float64
	TER      *float64
	AUM      *float64
}

type QualityParams struct {
	MaxTER    float64
	MinAUM    float64
	Weight    float64
	TERWeight float64
	AUMWeight float64
}

type QualityRow struct {
	Ticker   string
	Rank     int
	TERPct   *float64
	AUMPct   *float64
	Quality  *float64
	Momentum *float64
	Final    *float64
	Excluded []string
}

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

	scores := make([]float64, len(eligible))
	for k, i := range eligible {
		scores[k] = in[i].Score
	}
	for k, pct := range midRankPercentiles(scores, true) {
		rows[eligible[k]].Momentum = new(pct)
	}

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
			continue
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
