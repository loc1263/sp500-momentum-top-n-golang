package ipsa

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"top3/internal/config"
)

const page = `<table>
<tr><th>N.º</th><th>Nemotécnico</th><th>Empresa</th></tr>
<tr><td>1</td><td>AGUAS-A</td><td>Aguas Andinas</td></tr>
<tr><td>2</td><td>sqm-b</td><td>Sociedad Química y Minera de Chile</td></tr>
<tr><td colspan="3">[ 5 ]</td></tr>
<tr><td>3</td><td>SQM-B</td><td>repetida</td></tr>
<tr><td>4</td><td></td><td>sin nemotécnico</td></tr>
</table>`

func TestParseUniverseKeepsNemosInOrderWithoutDuplicates(t *testing.T) {
	got, err := ParseUniverse(page)
	if err != nil {
		t.Fatal(err)
	}
	want := []Stock{{"AGUAS-A", "Aguas Andinas"}, {"SQM-B", "Sociedad Química y Minera de Chile"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseUniverseFailsWithoutTheTable(t *testing.T) {
	if _, err := ParseUniverse(`<table><tr><th>Symbol</th></tr><tr><td>A</td></tr></table>`); err == nil {
		t.Fatal("esperaba error")
	}
}

func TestYahooTickerAddsSuffixAndAppliesAlias(t *testing.T) {
	c := config.Ipsa{Suffix: ".SN", Aliases: map[string]string{"ECL": "ENGIE"}}
	if got := YahooTicker(Stock{Nemo: "COPEC"}, c); got != "COPEC.SN" {
		t.Fatalf("got %s", got)
	}
	if got := YahooTicker(Stock{Nemo: "ECL"}, c); got != "ENGIE.SN" {
		t.Fatalf("got %s", got)
	}
}

func TestQuantConfigOverridesOnlyTheIpsaFields(t *testing.T) {
	cfg := config.Default()
	cfg.Quant.MuShrink = 0.4
	q := QuantConfig(cfg).Quant
	if q.RiskFree != 0.05 || q.TopN != 6 || q.MuShrink != 0.4 {
		t.Fatalf("got %+v", q)
	}
	if cfg.Quant.RiskFree != 0.04 {
		t.Fatal("no debe cambiar la configuración original")
	}
}

func TestEnsureUniverseReusesFreshFileAndFallsBackWhenStale(t *testing.T) {
	cfg := config.Default()
	cfg.Ipsa.UniverseFile = filepath.Join(t.TempDir(), "sub", "ipsa.json")
	cfg.Ipsa.WikipediaURL = "http://127.0.0.1:1/no-existe" // si se usara la red, falla
	cfg.Data.HTTPTimeout = 1
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	if _, err := EnsureUniverse(cfg, true, now); err == nil {
		t.Fatal("sin archivo ni red debía fallar")
	}
	if err := writeUniverse(cfg.Ipsa.UniverseFile, &Universe{UpdatedAt: "2026-10-20", Stocks: []Stock{{"COPEC", "Copec"}}}); err != nil {
		t.Fatal(err)
	}
	if u, err := EnsureUniverse(cfg, true, now); err != nil || len(u.Stocks) != 1 {
		t.Fatalf("archivo vigente: %v %v", u, err)
	}
	// vencido o con --no-cache: intenta la red, falla y usa la guardada
	for _, useCache := range []bool{true, false} {
		n := now
		if useCache {
			n = now.AddDate(0, 3, 0)
		}
		if u, err := EnsureUniverse(cfg, useCache, n); err != nil || u.UpdatedAt != "2026-10-20" {
			t.Fatalf("respaldo (cache=%v): %v %v", useCache, u, err)
		}
	}
	if _, err := os.Stat(cfg.Ipsa.UniverseFile); err != nil {
		t.Fatal(err)
	}
}
