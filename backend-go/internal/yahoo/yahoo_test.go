package yahoo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/loc1263/gohoo"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want Kind
	}{
		{fmt.Errorf("x: %w", gohoo.ErrNotFound), KindAbsent},
		{ErrNoBars, KindAbsent},
		{errors.New("gohoo: chart de VBCA devolvió error: Bad Request: Data doesn't exist for startDate = 1, endDate = 2"), KindAbsent},
		{errors.New("HTTP Error 404: Quote not found for symbol: XYZ"), KindAbsent},
		{errors.New("$ADM: possibly delisted; no price data found"), KindAbsent},
		{fmt.Errorf("agotados: %w", gohoo.ErrRateLimit), KindRateLimit},
		{errors.New("connection reset by peer"), KindOther},
	}
	for _, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("Classify(%q) = %d, quiero %d", c.err, got, c.want)
		}
	}
}

func TestParallelMapKeepsOrderAndErrors(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	out := ParallelMap(context.Background(), items, 4, func(_ context.Context, n int) (int, error) {
		time.Sleep(time.Duration(10-n) * time.Millisecond) // los últimos terminan antes
		if n == 5 {
			return 0, errors.New("cinco")
		}
		return n * n, nil
	})
	for i, r := range out {
		if items[i] == 5 {
			if r.Err == nil {
				t.Fatal("falta el error de 5")
			}
			continue
		}
		if r.Err != nil || r.Value != items[i]*items[i] {
			t.Fatalf("posición %d: %+v", i, r)
		}
	}
}

func TestLimiterSpacesCalls(t *testing.T) {
	l := newLimiter(20 * time.Millisecond)
	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 75*time.Millisecond {
		t.Fatalf("5 llamadas en %s: no se espaciaron", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newLimiter(time.Hour).Wait(ctx); err != nil { // la primera pasa sin espera
		t.Fatal(err)
	}
	l2 := newLimiter(time.Hour)
	_ = l2.Wait(context.Background())
	if err := l2.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
