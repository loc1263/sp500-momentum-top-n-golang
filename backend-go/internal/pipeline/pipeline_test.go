package pipeline

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"top3/internal/config"
)

func names(steps []Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Name
	}
	return out
}

func TestPipelineSteps(t *testing.T) {
	cfg := config.Default()
	steps := BuildSteps(cfg, Options{Config: "config.toml", NoCache: true, Years: 7, Tests: true}, "top3")
	n := names(steps)
	if n[0] != "Tests" || !reflect.DeepEqual(n[len(n)-2:], []string{"Exportar JSON del frontend", "Limpieza de datos y reportes viejos"}) {
		t.Fatalf("pasos: %v", n)
	}
	if len(steps) != 1+1+3+1+1+1+1 { // tests + backtest + 3 universos + ETFs + modelo + export + limpieza
		t.Fatalf("%d pasos: %v", len(steps), n)
	}
	if !slices.Contains(n, "Ranking de ETFs de Vanguard") || !slices.ContainsFunc(n, func(s string) bool { return strings.HasPrefix(s, "Modelo por score") }) {
		t.Fatalf("pasos: %v", n)
	}
	bt := steps[1].Cmd
	if !slices.Contains(bt, "--sweep") || !slices.Contains(bt, "--no-cache") || bt[slices.Index(bt, "--years")+1] != "7" {
		t.Fatalf("backtest: %v", bt)
	}
	var withCap []bool
	for _, s := range steps {
		if strings.HasPrefix(s.Name, "Top actual") {
			withCap = append(withCap, slices.Contains(s.Cmd, "--min-cap-billions"))
		}
	}
	if !reflect.DeepEqual(withCap, []bool{false, true, true}) {
		t.Fatalf("universos del Top actual: %v", withCap)
	}

	only := BuildSteps(cfg, Options{Config: "config.toml", SkipBacktest: true, SkipActual: true, SkipEtf: true, SkipQuant: true, SkipPrune: true}, "top3")
	if !reflect.DeepEqual(names(only), []string{"Exportar JSON del frontend"}) {
		t.Fatalf("solo export: %v", names(only))
	}
}

func TestPipelinePassesNoCacheOnlyToTheFirstDownloadingStep(t *testing.T) {
	cfg := config.Default()
	flagged := func(o Options) []string {
		var out []string
		for _, s := range BuildSteps(cfg, o, "top3") {
			if slices.Contains(s.Cmd, "--no-cache") {
				out = append(out, s.Name)
			}
		}
		return out
	}
	o := Options{Config: "config.toml", NoCache: true, SkipEtf: true, SkipQuant: true, SkipPrune: true}
	if got := flagged(o); len(got) != 1 || !strings.HasPrefix(got[0], "Backtest") { // solo el backtest
		t.Fatalf("con backtest: %v", got)
	}
	o.SkipBacktest = true // sin backtest, el primer Top actual
	if got := flagged(o); !reflect.DeepEqual(got, []string{"Top actual - índice completo"}) {
		t.Fatalf("sin backtest: %v", got)
	}
	o.NoCache = false
	if got := flagged(o); len(got) != 0 {
		t.Fatalf("sin --no-cache: %v", got)
	}
}

func TestStepCommandsCarryTheConfigPath(t *testing.T) {
	for _, s := range BuildSteps(config.Default(), Options{Config: "otro.toml"}, "/bin/top3") {
		if s.Cmd[0] == "/bin/top3" && !(slices.Contains(s.Cmd, "--config") && slices.Contains(s.Cmd, "otro.toml")) {
			t.Errorf("%s no lleva --config: %v", s.Name, s.Cmd)
		}
	}
}
