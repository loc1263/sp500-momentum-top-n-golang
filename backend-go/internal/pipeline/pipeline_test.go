// Pruebas del pipeline: pasos, caché solo en el primer paso de descarga, pasos opcionales y ruta de
// configuración.
//
// Fecha: 2026-11-19

package pipeline

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"topwx/internal/config"
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
	steps := BuildSteps(cfg, Options{Config: "config.toml", NoCache: true, Years: 7, Tests: true}, "topwx")
	n := names(steps)
	if n[0] != "Tests" || !reflect.DeepEqual(n[len(n)-3:], []string{"Exportar JSON para el sitio", "Generar el sitio estático (HTML, CSS y JS)", "Limpieza de datos y reportes viejos"}) {
		t.Fatalf("pasos: %v", n)
	}
	if len(steps) != 1+1+3+1+1+1+1+1+1+1 {
		t.Fatalf("%d pasos: %v", len(steps), n)
	}
	if !slices.ContainsFunc(n, func(s string) bool { return strings.HasPrefix(s, "ETFs por gestora") }) || !slices.ContainsFunc(n, func(s string) bool { return strings.HasPrefix(s, "Modelo por score") }) || !slices.ContainsFunc(n, func(s string) bool { return strings.HasPrefix(s, "Evaluador") }) {
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

	only := BuildSteps(cfg, Options{Config: "config.toml", SkipBacktest: true, SkipActual: true, SkipEtfq: true, SkipQuant: true, SkipEvaluator: true, SkipVisor: true, SkipSite: true, SkipPrune: true}, "topwx")
	if !reflect.DeepEqual(names(only), []string{"Exportar JSON para el sitio"}) {
		t.Fatalf("solo export: %v", names(only))
	}
}

func TestPipelinePassesNoCacheOnlyToTheFirstDownloadingStep(t *testing.T) {
	cfg := config.Default()
	flagged := func(o Options) []string {
		var out []string
		for _, s := range BuildSteps(cfg, o, "topwx") {
			if slices.Contains(s.Cmd, "--no-cache") {
				out = append(out, s.Name)
			}
		}
		return out
	}
	o := Options{Config: "config.toml", NoCache: true, SkipQuant: true, SkipEvaluator: true, SkipPrune: true}
	if got := flagged(o); len(got) != 1 || !strings.HasPrefix(got[0], "Backtest") {
		t.Fatalf("con backtest: %v", got)
	}
	o.SkipBacktest = true
	if got := flagged(o); !reflect.DeepEqual(got, []string{"Top actual - índice completo"}) {
		t.Fatalf("sin backtest: %v", got)
	}
	o.NoCache = false
	if got := flagged(o); len(got) != 0 {
		t.Fatalf("sin --no-cache: %v", got)
	}
}

func TestOnlyEvaluatorAndVisorAreOptional(t *testing.T) {
	for _, s := range BuildSteps(config.Default(), Options{Config: "c.toml"}, "topwx") {
		if want := strings.HasPrefix(s.Name, "Evaluador") || strings.HasPrefix(s.Name, "Visor"); s.Optional != want {
			t.Errorf("%s: Optional = %v, quiero %v", s.Name, s.Optional, want)
		}
	}
}

func TestStepCommandsCarryTheConfigPath(t *testing.T) {
	for _, s := range BuildSteps(config.Default(), Options{Config: "otro.toml"}, "/bin/topwx") {
		if s.Cmd[0] == "/bin/topwx" && !(slices.Contains(s.Cmd, "--config") && slices.Contains(s.Cmd, "otro.toml")) {
			t.Errorf("%s no lleva --config: %v", s.Name, s.Cmd)
		}
	}
}

func TestSkipVisorOmitsTheStep(t *testing.T) {
	has := func(o Options) bool {
		for _, s := range BuildSteps(config.Default(), o, "topwx") {
			if strings.HasPrefix(s.Name, "Visor") {
				return slices.Contains(s.Cmd, "visor")
			}
		}
		return false
	}
	if !has(Options{Config: "c.toml"}) || has(Options{Config: "c.toml", SkipVisor: true}) {
		t.Fatal("--skip-visor debe omitir el paso y, sin la opción, el paso debe estar")
	}
}
