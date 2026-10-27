package etfq

import (
	"encoding/json"
	"sort"
	"strconv"

	"top3/internal/config"
	"top3/internal/etf"
)

// Settings son los parámetros con los que se calculó la calidad (los que muestra la web).
type Settings struct {
	MaxTER         float64 `json:"max_ter"`
	MinAumMillions float64 `json:"min_aum_millions"`
	QualityWeight  float64 `json:"quality_weight"`
	TERWeight      float64 `json:"ter_weight"`
	AUMWeight      float64 `json:"aum_weight"`
}

// Issuer es el resumen de una gestora: ETFs listados, rankeados por momentum y con score final.
type Issuer struct {
	Name   string `json:"name"`
	Funds  int    `json:"funds"`
	Ranked int    `json:"ranked"`
	Scored int    `json:"scored"`
}

// Row es un ETF rankeado por momentum con su calidad. Las posiciones (ter_pct, aum_pct, quality, momentum, final) van de 0 a
// 100; rank es null si el ETF quedó fuera por el filtro de elegibilidad o no tiene TER ni AUM para puntuar.
type Row struct {
	Rank          *int               `json:"rank"`
	Ticker        string             `json:"ticker"`
	Name          string             `json:"name"`
	Issuer        string             `json:"issuer"`
	Category      string             `json:"category"`
	GroupSize     int                `json:"group_size"`
	MomentumRank  int                `json:"momentum_rank"`
	MomentumScore float64            `json:"momentum_score"`
	Returns       map[string]float64 `json:"returns"`
	TER           *float64           `json:"ter"`
	AUM           *float64           `json:"aum"`
	TERPct        *float64           `json:"ter_pct"`
	AUMPct        *float64           `json:"aum_pct"`
	Quality       *float64           `json:"quality"`
	Momentum      *float64           `json:"momentum"`
	Final         *float64           `json:"final"`
	Excluded      []string           `json:"excluded"`
}

// Payload es lo que se guarda en SQLite y lee el frontend.
type Payload struct {
	AsOf        string   `json:"as_of"`
	GeneratedAt string   `json:"generated_at"`
	Horizons    []int    `json:"horizons"`
	RiskFree    *string  `json:"risk_free"`
	Settings    Settings `json:"settings"`
	Issuers     []Issuer `json:"issuers"`
	NotRanked   int      `json:"not_ranked"`
	Rows        []Row    `json:"rows"`
}

// JSON serializa el resultado.
func (p Payload) JSON() (json.RawMessage, error) { return json.Marshal(p) }

// Params son los datos de la corrida que no salen de los ETFs.
type Params struct {
	Config    config.Etfq
	Horizons  []int
	NotRanked int // ETFs sin historia suficiente, con huecos o de tipo efectivo
}

// Assemble junta el ranking de momentum con TER, AUM y categoría de cada ETF y calcula la calidad. listed son los ETFs listados
// por gestora. Las filas quedan con score final primero (de mayor a menor) y el resto por su puesto de momentum.
func Assemble(funds []Fund, scores etf.Scores, categories map[string]string, listed map[string]int, p Params, asOf, generatedAt string) Payload {
	byTicker := map[string]Fund{}
	for _, f := range funds {
		byTicker[f.Ticker] = f
	}
	c := p.Config
	tw, aw := 1.0, 1.0
	if len(c.QualityWeights) == 2 {
		tw, aw = c.QualityWeights[0], c.QualityWeights[1]
	}
	categoryOf := func(ticker string) string {
		if cat := categories[ticker]; cat != "" {
			return cat
		}
		return NoCategory
	}
	in := make([]QualityInput, len(scores.Rows))
	for i, s := range scores.Rows {
		f := byTicker[s.Ticker]
		in[i] = QualityInput{Ticker: s.Ticker, Category: categoryOf(s.Ticker), Score: s.Score, TER: f.TER, AUM: f.AUM}
	}
	scored := ScoreQuality(in, QualityParams{MaxTER: c.MaxTER, MinAUM: c.MinAumMillions * 1e6, Weight: c.QualityWeight, TERWeight: tw, AUMWeight: aw})

	groupSize := map[string]int{}
	for i, q := range scored {
		if len(q.Excluded) == 0 {
			groupSize[in[i].Category]++
		}
	}
	out := Payload{AsOf: asOf, GeneratedAt: generatedAt, Horizons: p.Horizons, NotRanked: p.NotRanked,
		Settings: Settings{MaxTER: c.MaxTER, MinAumMillions: c.MinAumMillions, QualityWeight: c.QualityWeight, TERWeight: tw, AUMWeight: aw},
		Rows:     make([]Row, len(scores.Rows))}
	if scores.RiskFree != "" {
		out.RiskFree = &scores.RiskFree
	}
	rankedBy, scoredBy := map[string]int{}, map[string]int{}
	for i, s := range scores.Rows {
		q, f := scored[i], byTicker[s.Ticker]
		var rank *int
		if q.Rank > 0 {
			rank = &q.Rank
			scoredBy[f.Issuer]++
		}
		rankedBy[f.Issuer]++
		returns := map[string]float64{}
		for h, v := range s.Returns {
			returns[strconv.Itoa(h)] = v
		}
		out.Rows[i] = Row{Rank: rank, Ticker: s.Ticker, Name: f.Name, Issuer: f.Issuer, Category: in[i].Category,
			GroupSize: groupSize[in[i].Category], MomentumRank: s.Rank, MomentumScore: s.Score, Returns: returns,
			TER: f.TER, AUM: f.AUM, TERPct: q.TERPct, AUMPct: q.AUMPct, Quality: q.Quality, Momentum: q.Momentum, Final: q.Final,
			Excluded: append([]string{}, q.Excluded...)}
	}
	sort.SliceStable(out.Rows, func(a, b int) bool {
		x, y := out.Rows[a], out.Rows[b]
		switch {
		case x.Rank != nil && y.Rank != nil:
			return *x.Rank < *y.Rank
		case x.Rank != nil || y.Rank != nil:
			return x.Rank != nil
		}
		return x.MomentumRank < y.MomentumRank
	})
	// Gestoras en el orden de [etfq] families (listed solo trae las que respondieron)
	for _, fam := range c.Families {
		name := fam.Label
		if name == "" {
			name = fam.Name
		}
		if listed[name] > 0 {
			out.Issuers = append(out.Issuers, Issuer{Name: name, Funds: listed[name], Ranked: rankedBy[name], Scored: scoredBy[name]})
		}
	}
	return out
}
