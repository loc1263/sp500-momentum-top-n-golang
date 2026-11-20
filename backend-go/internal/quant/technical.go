// Indicadores técnicos (RSI, MACD, estocástico, ADX, OBV) y su score.
//
// Fecha: 2026-11-19

package quant

import (
	"math"

	"github.com/loc1263/gohoo"

	"top/internal/stats"
)

const DayYear = 252

var TechnicalGroups = []string{"tendencia", "impulso", "volumen", "posicion"}

type Bars struct {
	High, Low, Close, Volume []float64
}

func rsi(close []float64, n int) []float64 {
	delta := diff(close)
	gain := wilder(clipLower(delta, 0), n)
	loss := wilder(clipLower(mapf(delta, func(v float64) float64 { return -v }), 0), n)
	return zip2(gain, loss, func(g, l float64) float64 {
		if l == 0 {
			l = nan
		}
		ratio := 1 + g/l
		if isNaN(ratio) {
			ratio = math.Inf(1)
		}
		return 100 - 100/ratio
	})
}

func macd(close []float64, fast, slow, signal int) (line, hist []float64) {
	line = zip2(emaSpan(close, fast), emaSpan(close, slow), func(a, b float64) float64 { return a - b })
	sig := emaSpan(line, signal)
	return line, zip2(line, sig, func(a, b float64) float64 { return a - b })
}

func stochastic(high, low, close []float64, n, smooth int) []float64 {
	lowest, highest := rollingMin(low, n), rollingMax(high, n)
	k := make([]float64, len(close))
	for i := range k {
		rng := highest[i] - lowest[i]
		if rng == 0 {
			rng = nan
		}
		k[i] = 100 * (close[i] - lowest[i]) / rng
	}
	return rollingMean(k, smooth)
}

func adx(high, low, close []float64, n int) (adxLine, plusDI, minusDI []float64) {
	up := diff(high)
	down := mapf(diff(low), func(v float64) float64 { return -v })
	plusCond := make([]bool, len(up))
	minusCond := make([]bool, len(up))
	for i := range up {
		plusCond[i] = up[i] > down[i] && up[i] > 0
		minusCond[i] = down[i] > up[i] && down[i] > 0
	}
	plus, minus := whereCond(up, plusCond, 0), whereCond(down, minusCond, 0)
	prev := shift(close, 1)
	tr := make([]float64, len(close))
	for i := range tr {
		tr[i] = nan
		for _, v := range []float64{high[i] - low[i], math.Abs(high[i] - prev[i]), math.Abs(low[i] - prev[i])} {
			if !isNaN(v) && (isNaN(tr[i]) || v > tr[i]) {
				tr[i] = v
			}
		}
	}
	atr := wilder(tr, n)
	wp, wm := wilder(plus, n), wilder(minus, n)
	plusDI, minusDI = make([]float64, len(atr)), make([]float64, len(atr))
	dx := make([]float64, len(atr))
	for i := range atr {
		plusDI[i], minusDI[i] = 100*wp[i]/atr[i], 100*wm[i]/atr[i]
		sum := plusDI[i] + minusDI[i]
		if sum == 0 {
			sum = nan
		}
		dx[i] = 100 * math.Abs(plusDI[i]-minusDI[i]) / sum
	}
	return wilder(dx, n), plusDI, minusDI
}

func sign(v float64) float64 {
	switch {
	case isNaN(v):
		return nan
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func obv(close, volume []float64) []float64 {
	d := diff(close)
	flow := make([]float64, len(d))
	for i, v := range d {
		s := sign(v)
		if isNaN(s) {
			s = 0
		}
		flow[i] = s * volume[i]
	}
	return cumsum(flow)
}

func peakScore(value, low, peak, top float64) float64 {
	if value <= peak {
		return clipBoth((value-low)/(peak-low), 0, 1)
	}
	return 1 - 0.5*clipBoth((value-peak)/(top-peak), 0, 1)
}

type TechInd struct {
	Close, Ema20, Ema50, Ema200            float64
	Macd, MacdSignal, MacdHist, MacdHist3d float64
	RSI, Stoch, PctB                       float64
	ADX, PlusDI, MinusDI                   float64
	OBV, ObvEma, Obv20d                    float64
	MFI, High52, Low52                     float64
}

func last(x []float64) float64 {
	if len(x) == 0 {
		return nan
	}
	return x[len(x)-1]
}

func nanIndicators() TechInd {
	var ind TechInd
	nans := []*float64{&ind.Close, &ind.Ema20, &ind.Ema50, &ind.Ema200, &ind.Macd, &ind.MacdSignal, &ind.MacdHist, &ind.MacdHist3d,
		&ind.RSI, &ind.Stoch, &ind.PctB, &ind.ADX, &ind.PlusDI, &ind.MinusDI, &ind.OBV, &ind.ObvEma, &ind.Obv20d, &ind.MFI,
		&ind.High52, &ind.Low52}
	for _, p := range nans {
		*p = nan
	}
	return ind
}

func tail(x []float64, n int) []float64 { return x[max(len(x)-n, 0):] }

func ComputeIndicators(b Bars) TechInd {
	n := DayYear + 60
	high, low, close, volume := tail(b.High, n), tail(b.Low, n), tail(b.Close, n), tail(b.Volume, n)
	for _, v := range tail(close, DayYear) {
		if isNaN(v) {
			return nanIndicators()
		}
	}
	if len(close) < DayYear {
		return nanIndicators()
	}
	ema20, ema50, ema200 := emaSpan(close, 20), emaSpan(close, 50), emaSpan(close, 200)
	line, hist := macd(close, 12, 26, 9)
	sma20, std20 := rollingMean(close, 20), rollingStd(close, 20)
	adxLine, plusDI, minusDI := adx(high, low, close, 14)
	flow := obv(close, volume)
	lastIdx := len(close) - 1
	pctB := (close[lastIdx] - (sma20[lastIdx] - 2*std20[lastIdx])) / zeroNaN(4*std20[lastIdx])
	return TechInd{
		Close: close[lastIdx], Ema20: last(ema20), Ema50: last(ema50), Ema200: last(ema200),
		Macd: last(line), MacdSignal: last(line) - last(hist), MacdHist: last(hist), MacdHist3d: last(shift(hist, 3)),
		RSI: last(rsi(close, 14)), Stoch: last(stochastic(high, low, close, 14, 3)), PctB: pctB,
		ADX: last(adxLine), PlusDI: last(plusDI), MinusDI: last(minusDI),
		OBV: last(flow), ObvEma: last(emaSpan(flow, 20)), Obv20d: last(shift(flow, 20)),
		MFI: last(gohoo.MFISkipNaN(high, low, close, volume, 14)), High52: last(rollingMax(high, DayYear)), Low52: last(rollingMin(low, DayYear)),
	}
}

func zeroNaN(v float64) float64 {
	if v == 0 {
		return nan
	}
	return v
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type TechnicalScore struct {
	Groups    [4]float64
	GroupMean float64
	Technical float64
	RSI       float64
	MacdHist  float64
	VsEma50   float64
	VsEma200  float64
}

func ScoreOne(ind TechInd) TechnicalScore {
	c := ind.Close
	alignment := stats.Mean([]float64{b2f(c > ind.Ema20), b2f(c > ind.Ema50), b2f(c > ind.Ema200), b2f(ind.Ema20 > ind.Ema50), b2f(ind.Ema50 > ind.Ema200)})
	direction := 0.5 + 0.5*sign(ind.PlusDI-ind.MinusDI)*(math.Min(ind.ADX, 50)/50)
	if isNaN(ind.ADX) {
		direction = nan
	}
	macdVotes := stats.Mean([]float64{b2f(ind.Macd > ind.MacdSignal), b2f(ind.Macd > 0), b2f(ind.MacdHist > ind.MacdHist3d)})
	groups := [4]float64{
		stats.MeanSkipNaN([]float64{alignment, direction}),
		stats.MeanSkipNaN([]float64{macdVotes, peakScore(ind.RSI, 30, 70, 100), peakScore(ind.Stoch, 0, 80, 100), clipBoth(ind.PctB, 0, 1)}),
		stats.MeanSkipNaN([]float64{(b2f(ind.OBV > ind.ObvEma) + b2f(ind.OBV > ind.Obv20d)) / 2, peakScore(ind.MFI, 30, 70, 100)}),
		stats.MeanSkipNaN([]float64{c / ind.High52, (c - ind.Low52) / zeroNaN(ind.High52-ind.Low52)}),
	}
	out := TechnicalScore{Technical: nan, GroupMean: nan}
	measured := !isNaN(c) && !isNaN(ind.Ema200) && !isNaN(ind.High52)
	for i, g := range groups {
		if !measured {
			g = nan
		}
		out.Groups[i] = g * 100
		groups[i] = g
	}
	out.GroupMean = stats.Mean(groups[:])
	out.RSI, out.MacdHist = ind.RSI, ind.MacdHist/c
	out.VsEma50, out.VsEma200 = c/ind.Ema50-1, c/ind.Ema200-1
	return out
}

func TechnicalRank(scores []TechnicalScore) {
	means := make([]float64, len(scores))
	for i, s := range scores {
		means[i] = s.GroupMean
	}
	for i, r := range stats.RankPct(means, true) {
		scores[i].Technical = r * 100
	}
}
