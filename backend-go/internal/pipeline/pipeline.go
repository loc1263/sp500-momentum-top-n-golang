// Package pipeline corre el proceso completo: tests (opcional) -> backtest de todos los períodos y universos -> Top actual de
// cada universo -> ranking de ETFs -> modelo por score -> evaluador de empresas -> JSON del sitio -> sitio estático -> limpieza de lo viejo.
//
//	top pipeline                  # todo, con caché y descarga incremental de precios
//	top pipeline --no-cache       # descarga todo de nuevo
//	top pipeline --tests          # corre primero los tests
//	top pipeline --skip-backtest  # solo Top actual + export
//	top pipeline --skip-visor     # omite el visor de acciones grandes y ETFs
//
// Cada paso es un comando (`top <comando>`) que se ejecuta en su propio proceso; si uno falla, el pipeline se corta.
// Períodos, plazos y universos salen de config.toml ([backtest] periods_years, [simulation] months,
// [universe] sweep_min_cap_billions).
//
// Fecha: 2026-11-19
package pipeline

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"top/internal/config"
	"top/internal/logx"
)

type Step struct {
	Name     string
	Cmd      []string
	Optional bool
}

type Options struct {
	Config        string
	NoCache       bool
	Years         float64
	Tests         bool
	SkipBacktest  bool
	SkipActual    bool
	SkipQuant     bool
	SkipEvaluator bool
	SkipVisor     bool
	SkipEtfq      bool
	SkipSite      bool
	SkipPrune     bool
}

func universeArgs(level float64) []string {
	if level == 0 {
		return nil
	}
	return []string{"--min-cap-billions", strconv.FormatFloat(level, 'g', -1, 64)}
}

func BuildSteps(cfg config.Config, o Options, self string) []Step {
	base := []string{"--config", o.Config}
	pending := o.NoCache
	cache := func() []string {
		if pending {
			pending = false
			return []string{"--no-cache"}
		}
		return nil
	}
	cmd := func(name string, extra ...[]string) []string {
		c := []string{self, name}
		c = append(c, base...)
		for _, e := range extra {
			c = append(c, e...)
		}
		return c
	}
	var steps []Step
	if o.Tests {
		steps = append(steps, Step{Name: "Tests", Cmd: []string{"go", "test", "./..."}})
	}
	if !o.SkipBacktest {
		var years []string
		if o.Years != 0 {
			years = []string{"--years", strconv.FormatFloat(o.Years, 'g', -1, 64)}
		}
		steps = append(steps, Step{Name: "Backtest: períodos x plazos x universos (sweep + offsets)", Cmd: cmd("backtest", []string{"--sweep"}, cache(), years)})
	}
	if !o.SkipActual {
		for _, level := range cfg.Universe.SweepMinCapBillions {
			label := "índice completo"
			if level != 0 {
				label = fmt.Sprintf("cap. >= US$ %g B", level)
			}
			steps = append(steps, Step{Name: "Top actual - " + label, Cmd: cmd("actual", cache(), universeArgs(level))})
		}
	}
	if !o.SkipEtfq {
		steps = append(steps, Step{Name: "ETFs por gestora: momentum y calidad (TER y AUM)", Cmd: cmd("etfq")})
	}
	if !o.SkipQuant {
		steps = append(steps, Step{Name: "Modelo por score: scores de momentum, salud y técnico, y Markowitz", Cmd: cmd("quant", cache())})
	}
	if !o.SkipEvaluator {
		steps = append(steps, Step{Name: "Evaluador de empresas: análisis fundamental del S&P 500", Cmd: cmd("evaluator"), Optional: true})
	}
	if !o.SkipVisor {
		steps = append(steps, Step{Name: "Visor: rendimientos y riesgo de acciones grandes y ETFs", Cmd: cmd("visor"), Optional: true})
	}
	steps = append(steps, Step{Name: "Exportar JSON para el sitio", Cmd: cmd("export")})
	if !o.SkipSite {
		steps = append(steps, Step{Name: "Generar el sitio estático (HTML, CSS y JS)", Cmd: cmd("site")})
	}
	if !o.SkipPrune {
		steps = append(steps, Step{Name: "Limpieza de datos y reportes viejos", Cmd: cmd("prune")})
	}
	return steps
}

type result struct {
	name     string
	rc       int
	took     time.Duration
	optional bool
}

func Main(args []string) int {
	fs := flag.NewFlagSet("pipeline", flag.ContinueOnError)
	var o Options
	fs.StringVar(&o.Config, "config", "config.toml", "Archivo de configuración")
	fs.BoolVar(&o.NoCache, "no-cache", false, "Vuelve a descargar todo en lugar de actualizar")
	fs.Float64Var(&o.Years, "years", 0, "Backtest de un único período de N años (default: todos los de periods_years)")
	fs.BoolVar(&o.Tests, "tests", false, "Corre los tests antes del pipeline")
	fs.BoolVar(&o.SkipBacktest, "skip-backtest", false, "Omite el backtest")
	fs.BoolVar(&o.SkipActual, "skip-actual", false, "Omite el Top actual")
	fs.BoolVar(&o.SkipQuant, "skip-quant", false, "Omite el modelo por score (scores y Markowitz)")
	fs.BoolVar(&o.SkipEvaluator, "skip-evaluator", false, "Omite el evaluador de empresas")
	fs.BoolVar(&o.SkipVisor, "skip-visor", false, "Omite el visor de acciones grandes y ETFs")
	fs.BoolVar(&o.SkipSite, "skip-site", false, "Omite la generación del sitio estático")
	fs.BoolVar(&o.SkipPrune, "skip-prune", false, "Omite la limpieza final de datos y reportes viejos")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	cfg, _, err := config.Load(o.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, logx.Paint(err.Error(), logx.Error))
		return 1
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	steps := BuildSteps(cfg, o, self)
	fmt.Printf("Pipeline: %d pasos\n\n", len(steps))

	dir, err := os.MkdirTemp("", "top-pipeline-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	summaryFile := filepath.Join(dir, "summary.jsonl")

	rule := logx.Heading(strings.Repeat("=", 78), os.Stdout)
	var results []result
	start := time.Now()
	for i, s := range steps {
		fmt.Println(rule)
		fmt.Println(logx.Heading(fmt.Sprintf("[%d/%d] %s", i+1, len(steps), s.Name), os.Stdout))
		fmt.Println(rule)
		t0 := time.Now()
		c := exec.Command(s.Cmd[0], s.Cmd[1:]...)
		c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
		c.Env = append(os.Environ(), logx.SummaryFileEnv+"="+summaryFile, logx.StepEnv+"="+s.Name)
		rc := 0
		if err := c.Run(); err != nil {
			rc = 1
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		results = append(results, result{s.Name, rc, time.Since(t0), s.Optional})
		if rc != 0 && s.Optional {
			fmt.Fprintln(os.Stderr, logx.Paint(fmt.Sprintf("\nFALLÓ el paso opcional «%s» (código %d). El pipeline sigue sin él.", s.Name, rc), logx.Warning))
			continue
		}
		if rc != 0 {
			fmt.Fprintln(os.Stderr, logx.Paint(fmt.Sprintf("\nFALLÓ el paso «%s» (código %d). Se corta el pipeline.", s.Name, rc), logx.Error))
			break
		}
	}

	fmt.Println()
	fmt.Println(rule)
	fmt.Println(logx.Heading("RESUMEN", os.Stdout))
	fmt.Println(rule)
	failed := false
	for _, r := range results {
		state := "OK   "
		switch {
		case r.rc != 0 && r.optional:
			state = "FALLÓ (opcional, se siguió)"
		case r.rc != 0:
			state, failed = "FALLÓ", true
		}
		fmt.Printf("  %s  %6.0f s  %s\n", state, r.took.Seconds(), r.name)
	}
	for _, s := range steps[len(results):] {
		fmt.Printf("  omitido         %s\n", s.Name)
	}
	fmt.Printf("\nTotal: %.0f s\n", time.Since(start).Seconds())
	if items := logx.MergeSummaries(summaryFile); len(items) > 0 {
		for _, l := range logx.FormatSummary(items, filepath.Join(cfg.Output.LogDir, "*.log")) {
			fmt.Fprintln(os.Stderr, l)
		}
	}
	if failed {
		return 1
	}
	return 0
}
