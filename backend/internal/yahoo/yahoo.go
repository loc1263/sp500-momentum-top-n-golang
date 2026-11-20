// Package yahoo envuelve gohoo (cliente de Yahoo Finance) con la política de tráfico del proyecto: ritmo de requests,
// reintentos ante rate limit con espera creciente (y su aviso TOP-NET-006) y clasificación de errores.
//
// Fecha: 2026-11-19
package yahoo

import (
	"errors"
	"strings"
	"time"

	"github.com/loc1263/gohoo"

	"topwx/internal/errs"
	"topwx/internal/logx"
)

const (
	RateLimitRetries = 3
	RateLimitWait    = 10 * time.Second
	minInterval      = 40 * time.Millisecond
)

type Client struct {
	*gohoo.Client
}

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
	if userAgent != "" && userAgent != "Mozilla/5.0" {
		opts = append(opts, gohoo.WithUserAgent(userAgent))
	}
	g, err := gohoo.NewClientWithOptions(append(opts, extra...)...)
	if err != nil {
		return nil, err
	}
	return &Client{g}, nil
}

type Kind int

const (
	KindOther Kind = iota
	KindAbsent
	KindRateLimit
)

var expectedAbsence = []string{"possibly delisted", "no price data", "no data found", "no timezone found",
	"quote not found", "http error 404", "not found", "sin datos", "data doesn't exist"}

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
