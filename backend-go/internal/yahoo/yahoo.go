// Package yahoo envuelve gohoo (cliente de Yahoo Finance) con la política de tráfico del proyecto: ritmo de requests,
// reintentos ante rate limit con espera creciente (y su aviso TOP-NET-006) y clasificación de errores.
package yahoo

import (
	"errors"
	"strings"
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
		gohoo.WithLimiter(gohoo.NewIntervalLimiter(minInterval)),
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

// expectedAbsence son mensajes de «sin datos» que gohoo todavía no marca con ErrNotFound (p. ej. un chart.error de otro tipo).
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
	if errors.Is(err, gohoo.ErrNotFound) {
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
