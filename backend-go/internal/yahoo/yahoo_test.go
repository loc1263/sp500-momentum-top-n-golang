package yahoo

import (
	"errors"
	"fmt"
	"testing"

	"github.com/loc1263/gohoo"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want Kind
	}{
		{fmt.Errorf("x: %w", gohoo.ErrNotFound), KindAbsent},
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
