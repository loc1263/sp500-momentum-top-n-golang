package quant

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"top3/internal/config"
	"top3/internal/dateutil"
	"top3/internal/db"
	"top3/internal/errs"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/model"
	"top3/internal/repo"
	"top3/internal/stats"
	"top3/internal/yahoo"
)

// Num es un número que en JSON sale como null cuando es NaN o infinito (JSON no los admite).
type Num float64

// MarshalJSON escribe null para NaN e infinitos.
func (n Num) MarshalJSON() ([]byte, error) {
	if f := float64(n); math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(float64(n))
}

// UnmarshalJSON lee null como NaN.
func (n *Num) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = Num(math.NaN())
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*n = Num(f)
	return nil
}

// RankRow es una acción del ranking final con sus scores, indicadores y el peso de Markowitz (solo las del top).
type RankRow struct {
	Rank            int            `json:"rank"`
	Ticker          string         `json:"ticker"`
	Name            string         `json:"name"`
	Sector          string         `json:"sector"`
	MarketCap       Num            `json:"market_cap"`
	Score           Num            `json:"score"`
	Momentum        Num            `json:"momentum"`
	Health          Num            `json:"health"`
	Technical       Num            `json:"technical"`
	Groups          int            `json:"groups"`
	HealthGroups    map[string]Num `json:"health_groups"`
	TechnicalGroups map[string]Num `json:"technical_groups"`
	RSI             Num            `json:"rsi"`
	MacdHist        Num            `json:"macd_hist"`
	VsEma50         Num            `json:"vs_ema50"`
	VsEma200        Num            `json:"vs_ema200"`
	Mom121          Num            `json:"mom_12_1"`
	Mom61           Num            `json:"mom_6_1"`
	ROE             Num            `json:"roe"`
	OperatingMargin Num            `json:"operating_margin"`
	DebtToEquity    Num            `json:"debt_to_equity"`
	NetDebtEbitda   Num            `json:"net_debt_ebitda"`
	CurrentRatio    Num            `json:"current_ratio"`
	FcfMargin       Num            `json:"fcf_margin"`
	FcfYield        Num            `json:"fcf_yield"`
	CashConversion  Num            `json:"cash_conversion"`
	OcfToDebt       Num            `json:"ocf_to_debt"`
	FcfPositive     Num            `json:"fcf_positive_years"`
	Weight          Num            `json:"weight"`
}

// Settings son los parámetros con los que se calculó el modelo.
type Settings struct {
	TopN          int                `json:"top_n"`
	VolTarget     float64            `json:"vol_target"`
	WMax          float64            `json:"w_max"`
	SectorMax     float64            `json:"sector_max"`
	MuShrink      float64            `json:"mu_shrink"`
	RiskFree      float64            `json:"risk_free"`
	Prior         float64            `json:"prior"`
	Weights       []float64          `json:"weights"`
	HealthWeights map[string]float64 `json:"health_weights"`
}

// Excluded son las acciones que quedaron fuera de cada score por falta de datos.
type Excluded struct {
	NoMomentum     []string `json:"sin_momentum"`
	NoFundamentals []string `json:"sin_datos_financieros"`
	NoTechnical    []string `json:"sin_datos_tecnicos"`
}

// Coverage dice cuántas acciones tienen cada score y cuáles quedaron fuera.
type Coverage struct {
	Universe  int      `json:"universe"`
	Momentum  int      `json:"momentum"`
	Health    int      `json:"health"`
	Technical int      `json:"technical"`
	Ranked    int      `json:"ranked"`
	Excluded  Excluded `json:"excluded"`
}

// MarkowitzTicker es el retorno esperado y la volatilidad de una de las acciones del top.
type MarkowitzTicker struct {
	Ticker    string `json:"ticker"`
	ExpReturn Num    `json:"exp_return"`
	Vol       Num    `json:"vol"`
}

// MarkowitzOut es el resultado de Markowitz sobre el top.
type MarkowitzOut struct {
	Tickers []MarkowitzTicker `json:"tickers"`
	Frontier
	Main string `json:"main"`
}

// Payload es el resultado del modelo por score, tal como se guarda en SQLite y lo lee el frontend.
type Payload struct {
	AsOf        string       `json:"as_of"`
	GeneratedAt string       `json:"generated_at"`
	Settings    Settings     `json:"settings"`
	Coverage    Coverage     `json:"coverage"`
	Ranking     []RankRow    `json:"ranking"`
	Markowitz   MarkowitzOut `json:"markowitz"`
}

// Inputs son los datos con los que se calcula el modelo.
type ComputeInput struct {
	Prices  *model.Prices
	Tickers []string
	Data    map[string]Data
	OHLCV   repo.OHLCVFrames
}

type scored struct {
	ticker      string
	row         Row
	mom12, mom6 float64
	momentum    float64
	health      Health
	tech        TechnicalScore
	score       float64
}

// Compute arma el ranking por score y Markowitz sobre los mejores.
func Compute(in ComputeInput, cfg config.Config) (*Payload, error) {
	q := cfg.Quant
	// Sin la rueda de hoy si aún no cerró: de las acciones actuales no tiene ningún precio y anularía el momentum de todas
	prices := in.Prices.Reindex(in.Tickers).TrimEmptyTail()
	tickers := in.Tickers
	n := len(tickers)

	mom12, mom6 := MomentumReturns(prices)
	momentum := MomentumScore(mom12, mom6)
	idx := map[string]int{}
	for i, t := range tickers {
		idx[t] = i
	}

	var withData []string
	for _, t := range tickers {
		if _, ok := in.Data[t]; ok {
			withData = append(withData, t)
		}
	}
	rows := HealthIndicators(withData, in.Data)
	hw := map[string]float64{}
	for i, g := range HealthGroups {
		if i < len(q.HealthWeights) {
			hw[g] = q.HealthWeights[i]
		}
	}
	health := HealthScore(rows, hw)

	// técnico: sobre todas las acciones del índice (el ranking porcentual las incluye a todas)
	o := repo.OHLCVFrames{High: in.OHLCV.High, Low: in.OHLCV.Low, Close: in.OHLCV.Close, Volume: in.OHLCV.Volume}
	techs := make([]TechnicalScore, n)
	for i, t := range tickers {
		techs[i] = ScoreOne(ComputeIndicators(Bars{High: series(o.High, t), Low: series(o.Low, t), Close: series(o.Close, t), Volume: series(o.Volume, t)}))
	}
	TechnicalRank(techs)

	var ranked []scored
	for k, r := range rows {
		i := idx[withData[k]]
		s := scored{ticker: withData[k], row: r, mom12: mom12[i], mom6: mom6[i], momentum: momentum[i], health: health[k], tech: techs[i]}
		if isNaN(s.momentum) || isNaN(s.health.Health) || isNaN(s.tech.Technical) {
			continue
		}
		ranked = append(ranked, s)
	}
	if len(ranked) < q.TopN {
		return nil, errs.QntNoSignal.Errorf("Solo %d acciones tienen los tres scores.", len(ranked))
	}
	weights := q.Weights
	wsum := stats.Sum(weights)
	for i := range ranked {
		s := &ranked[i]
		s.score = (s.momentum*weights[0] + s.health.Health*weights[1] + s.tech.Technical*weights[2]) / wsum
	}
	sort.SliceStable(ranked, func(a, b int) bool {
		if ranked[a].score != ranked[b].score {
			return ranked[a].score > ranked[b].score
		}
		return ranked[a].momentum > ranked[b].momentum
	})
	nHealth, nTech, nMom := 0, 0, 0
	for _, v := range momentum {
		if !isNaN(v) {
			nMom++
		}
	}
	for _, h := range health {
		if !isNaN(h.Health) {
			nHealth++
		}
	}
	for _, t := range techs {
		if !isNaN(t.Technical) {
			nTech++
		}
	}
	logx.Infof("Acciones: %d en el índice, %d con momentum, %d con datos financieros, %d con datos técnicos, %d en el ranking final",
		n, nMom, nHealth, nTech, len(ranked))

	// Markowitz sobre las mejores
	topN := q.TopN
	top := make([]string, topN)
	sectors := make([]string, topN)
	for i := 0; i < topN; i++ {
		top[i], sectors[i] = ranked[i].ticker, ranked[i].row.Sector
	}
	prior := q.RiskFree + q.EquityPremium
	mu, sigma := Inputs(prices.Select(top), q.MuShrink, prior)
	frontier, err := Optimize(top, mu, sigma, q.VolTarget, q.WMax, q.RiskFree, sectors, q.SectorMax)
	if err != nil {
		return nil, err
	}
	// los pesos de la tabla son los de la primera cartera disponible: máximo retorno o, si no se alcanza el riesgo, Sharpe
	weight := map[string]float64{}
	for _, w := range frontier.Portfolios[0].Weights {
		weight[w.Ticker] = w.Weight
	}

	out := &Payload{
		AsOf:        dateutil.Day(prices.Dates[prices.Rows()-1]),
		GeneratedAt: time.Now().Format("2006-01-02T15:04:05"),
		Settings: Settings{TopN: topN, VolTarget: q.VolTarget, WMax: q.WMax, SectorMax: q.SectorMax, MuShrink: q.MuShrink,
			RiskFree: q.RiskFree, Prior: prior, Weights: weights, HealthWeights: hw},
		Coverage: coverage(tickers, momentum, health, withData, techs, len(ranked)),
	}
	for rank, s := range ranked {
		t := s.ticker
		r := RankRow{
			Rank: rank + 1, Ticker: t, Name: s.row.Name, Sector: s.row.Sector, MarketCap: Num(s.row.MarketCap),
			Score: Num(s.score), Momentum: Num(s.momentum), Health: Num(s.health.Health), Technical: Num(s.tech.Technical),
			Groups: s.health.Measure, HealthGroups: map[string]Num{}, TechnicalGroups: map[string]Num{},
			RSI: Num(s.tech.RSI), MacdHist: Num(s.tech.MacdHist), VsEma50: Num(s.tech.VsEma50), VsEma200: Num(s.tech.VsEma200),
			Mom121: Num(s.mom12), Mom61: Num(s.mom6),
			ROE: Num(s.row.V["roe"]), OperatingMargin: Num(s.row.V["operating_margin"]), DebtToEquity: Num(s.row.V["debt_to_equity"]),
			NetDebtEbitda: Num(s.row.V["net_debt_ebitda"]), CurrentRatio: Num(s.row.V["current_ratio"]), FcfMargin: Num(s.row.V["fcf_margin"]),
			FcfYield: Num(s.row.V["fcf_yield"]), CashConversion: Num(s.row.V["cash_conversion"]), OcfToDebt: Num(s.row.V["ocf_to_debt"]),
			FcfPositive: Num(s.row.V["fcf_positive_years"]),
		}
		for g, name := range HealthGroups {
			r.HealthGroups[name] = Num(s.health.Groups[g])
		}
		for g, name := range TechnicalGroups {
			r.TechnicalGroups[name] = Num(s.tech.Groups[g])
		}
		if rank < topN {
			r.Weight = Num(weight[t])
		}
		out.Ranking = append(out.Ranking, r)
	}
	main := frontier.Portfolios[0].Key
	mk := MarkowitzOut{Frontier: frontier, Main: main}
	for i, t := range top {
		mk.Tickers = append(mk.Tickers, MarkowitzTicker{Ticker: t, ExpReturn: Num(mu[i]), Vol: Num(math.Sqrt(sigma[i][i]))})
	}
	out.Markowitz = mk
	return out, nil
}

func series(m *model.Prices, ticker string) []float64 {
	if m == nil {
		return nil
	}
	if s := m.Series(ticker); s != nil {
		return s
	}
	return nil
}

func coverage(tickers []string, momentum []float64, health []Health, withData []string, techs []TechnicalScore, ranked int) Coverage {
	c := Coverage{Universe: len(tickers), Ranked: ranked}
	hasHealth := map[string]bool{}
	for k, h := range health {
		if !isNaN(h.Health) {
			hasHealth[withData[k]] = true
			c.Health++
		}
	}
	for i, t := range tickers {
		if isNaN(momentum[i]) {
			c.Excluded.NoMomentum = append(c.Excluded.NoMomentum, t)
		} else {
			c.Momentum++
		}
		if !hasHealth[t] {
			c.Excluded.NoFundamentals = append(c.Excluded.NoFundamentals, t)
		}
		if isNaN(techs[i].Technical) {
			c.Excluded.NoTechnical = append(c.Excluded.NoTechnical, t)
		} else {
			c.Technical++
		}
	}
	for _, l := range []*[]string{&c.Excluded.NoMomentum, &c.Excluded.NoFundamentals, &c.Excluded.NoTechnical} {
		sort.Strings(*l)
		if *l == nil {
			*l = []string{}
		}
	}
	return c
}

// Run actualiza precios, OHLCV y datos financieros, calcula los scores y Markowitz y guarda el resultado en SQLite.
func Run(ctx context.Context, cfg config.Config, y *yahoo.Client, useCache bool) (*Payload, error) {
	d, err := db.Open(cfg.Data.DBPath)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	dl := &market.Downloader{Y: y, Workers: cfg.Data.SharesWorkers}
	// El modelo mira las últimas DayYear+1 ruedas de precios (momentum y Markowitz): no hace falta cargar el histórico de 12 años.
	md, err := dl.RefreshRecent(ctx, d, cfg, useCache, dateutil.Today(), 450, DayYear+1)
	if err != nil {
		return nil, err
	}
	tickers := append([]string(nil), md.Constituents.Current...)
	sort.Strings(tickers)
	data, err := EnsureFundamentals(ctx, d, y, tickers, useCache, cfg.Quant.FundamentalsTTLDays, cfg.Data.SharesWorkers)
	if err != nil {
		return nil, err
	}
	if err := EnsureOHLCV(ctx, dl, d, tickers, useCache, cfg.Data.BatchSize); err != nil {
		return nil, err
	}
	ohlcv, err := repo.LoadOHLCV(d)
	if err != nil {
		return nil, err
	}
	payload, err := Compute(ComputeInput{Prices: md.Prices, Tickers: tickers, Data: data, OHLCV: ohlcv}, cfg)
	if err != nil {
		return nil, err
	}
	js, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if err := repo.SaveLatestQuant(d, payload.GeneratedAt, payload.AsOf, js); err != nil {
		return nil, err
	}
	logx.Infof("Modelo por score guardado en SQLite (tabla quant_runs)")
	return payload, nil
}

// Summary es el texto de la consola con las mejores acciones.
func (p *Payload) Summary() []string {
	lines := []string{fmt.Sprintf("Ranking al %s (%d acciones). Las mejores:", p.AsOf, p.Coverage.Ranked)}
	for _, r := range p.Ranking[:min(p.Settings.TopN, len(p.Ranking))] {
		lines = append(lines, fmt.Sprintf("  %2d. %-6s score %5.1f (momentum %5.1f, salud %5.1f, técnico %5.1f)",
			r.Rank, r.Ticker, float64(r.Score), float64(r.Momentum), float64(r.Health), float64(r.Technical)))
	}
	return lines
}
