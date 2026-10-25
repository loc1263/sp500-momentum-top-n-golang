// Package yahoo envuelve gohoo (cliente de Yahoo Finance) con la política de tráfico del proyecto: ritmo de requests,
// reintentos ante rate limit con espera creciente (y su aviso TOP-NET-006), clasificación de errores y descargas en
// paralelo.
package yahoo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"

	"top3/internal/errs"
	"top3/internal/logx"
)

// Política ante rate limit: hasta 3 reintentos esperando 10 s (el doble en cada uno).
const (
	RateLimitRetries = 3
	RateLimitWait    = 10 * time.Second
	minInterval      = 40 * time.Millisecond // ritmo máximo ≈ 25 requests por segundo en todo el proceso
)

// Client es el cliente de Yahoo del proyecto. Es seguro usarlo desde varias goroutines.
type Client struct {
	*gohoo.Client
}

// New crea el cliente. Cookie y crumb se piden recién cuando hacen falta (chart no los necesita). Las opciones extra se
// aplican al final (los tests apuntan el cliente a un servidor local con gohoo.WithBaseURL).
func New(userAgent string, extra ...gohoo.Option) (*Client, error) {
	policy := gohoo.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		RateLimit: &gohoo.RateLimitPolicy{
			MaxRetries:      RateLimitRetries,
			BaseDelay:       RateLimitWait,
			Jitter:          time.Second,
			HonorRetryAfter: true,
		},
		OnRetry: func(attempt int, err error, delay time.Duration) {
			if gohoo.IsRateLimit(err) {
				logx.Warnf(errs.NetRateRetry.Msg("Yahoo limitó las solicitudes; reintento en %s"), delay.Round(time.Second))
			}
		},
	}
	opts := []gohoo.Option{
		gohoo.WithLazyBootstrap(),
		gohoo.WithRetryPolicy(policy),
		gohoo.WithLimiter(newLimiter(minInterval)),
	}
	if userAgent != "" && userAgent != "Mozilla/5.0" { // el user agent por defecto de gohoo parece un navegador real
		opts = append(opts, gohoo.WithUserAgent(userAgent))
	}
	g, err := gohoo.NewClientWithOptions(append(opts, extra...)...)
	if err != nil {
		return nil, err
	}
	return &Client{g}, nil
}

// limiter espacia las requests de todo el proceso.
type limiter struct {
	mu   sync.Mutex
	next time.Time
	gap  time.Duration
}

func newLimiter(gap time.Duration) *limiter { return &limiter{gap: gap} }

func (l *limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.gap)
	l.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Kind clasifica un error de Yahoo.
type Kind int

const (
	// KindOther es cualquier otro error (red, 5xx, formato).
	KindOther Kind = iota
	// KindAbsent: el ticker no existe, está dado de baja o no tiene datos en el rango.
	KindAbsent
	// KindRateLimit: Yahoo limitó las solicitudes y se agotaron los reintentos.
	KindRateLimit
)

var expectedAbsence = []string{"possibly delisted", "no price data", "no data found", "no timezone found",
	"quote not found", "http error 404", "not found", "sin datos", "data doesn't exist"}

// Classify dice qué clase de error es err.
func Classify(err error) Kind {
	if err == nil {
		return KindOther
	}
	if gohoo.IsRateLimit(err) {
		return KindRateLimit
	}
	if errors.Is(err, gohoo.ErrNotFound) || errors.Is(err, ErrNoBars) {
		return KindAbsent
	}
	lower := strings.ToLower(err.Error())
	for _, s := range expectedAbsence {
		if strings.Contains(lower, s) {
			return KindAbsent
		}
	}
	return KindOther
}

// ErrNoBars es el error de un ticker al que Yahoo respondió sin ninguna barra.
var ErrNoBars = errors.New("sin datos de precios")

// DailyBars descarga las barras diarias de ticker entre start y end (end exclusivo), ajustadas por dividendos y splits
// ajustado por splits y dividendos y con la hora llevada a la medianoche del día en la zona del exchange.
func (c *Client) DailyBars(ctx context.Context, ticker string, start, end time.Time) ([]gohoo.Bar, error) {
	res, err := c.ChartRange(ctx, ticker, gohoo.ChartParams{Start: start, End: end, Interval: "1d"})
	if err != nil {
		return nil, err
	}
	bars := gohoo.AtMidnight(res.AutoAdjusted())
	if len(bars) == 0 {
		return nil, ErrNoBars
	}
	return bars, nil
}

// Result es lo que devolvió fn para el elemento i de ParallelMap.
type Result[R any] struct {
	Value R
	Err   error
}

// ParallelMap corre fn sobre cada item con hasta `workers` goroutines y devuelve los resultados en el mismo orden.
func ParallelMap[T, R any](ctx context.Context, items []T, workers int, fn func(ctx context.Context, item T) (R, error)) []Result[R] {
	out := make([]Result[R], len(items))
	workers = max(1, min(workers, len(items)))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				v, err := fn(ctx, items[i])
				out[i] = Result[R]{v, err}
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}
