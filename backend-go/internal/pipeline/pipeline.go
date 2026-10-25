// Package pipeline corre el proceso completo: tests (opcional) -> backtest de todos los períodos y universos -> Top actual de
// cada universo -> ranking de ETFs -> modelo por score -> JSON del frontend -> limpieza de lo viejo.
//
//	top3 pipeline                  # todo, con caché y descarga incremental de precios
//	top3 pipeline --no-cache       # descarga todo de nuevo
//	top3 pipeline --tests          # corre primero los tests
//	top3 pipeline --skip-backtest  # solo Top actual + export
//
// Cada paso es un comando (`top3 <comando>`) que se ejecuta en su propio proceso; si uno falla, el pipeline se corta.
// Períodos, plazos y universos salen de config.toml ([backtest] periods_years, [simulation] months,
// [universe] sweep_min_cap_billions).
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

	"top3/internal/config"
	"top3/internal/logx"
)

// Step es un paso del pipeline: su nombre y el comando que lo corre.
type Step struct {
	Name string
	Cmd  []string // el primer elemento es el ejecutable
}

// Options son las opciones del pipeline.
type Options struct {
	Config       string
	NoCache      bool
	Years        float64
	Tests        bool
	SkipBacktest bool
	SkipActual   bool
	SkipQuant    bool
	SkipEtf      bool
	SkipPrune    bool
}

func universeArgs(level float64) []string {
	if level == 0 {
		return nil // índice completo (sin filtro)
	}
	return []string{"--min-cap-billions", strconv.FormatFloat(level, 'g', -1, 64)}
}

// BuildSteps arma los pasos del pipeline. self es el ejecutable de top3.
func BuildSteps(cfg config.Config, o Options, self string) []Step {
	base := []string{"--config", o.Config}
	pending := o.NoCache
	cache := func() []string {
		// --no-cache solo para el primer paso que descarga: los siguientes reutilizan lo que ese paso acaba de guardar en
		// SQLite en vez de volver a bajar lo mismo (los Top actual repetían 3 veces las descargas)
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
		steps = append(steps, Step{"Tests", []string{"go", "test", "./..."}})
	}
	if !o.SkipBacktest {
		var years []string
		if o.Years != 0 {
			years = []string{"--years", strconv.FormatFloat(o.Years, 'g', -1, 64)}
		}
		steps = append(steps, Step{"Backtest: períodos x plazos x universos (sweep + offsets)", cmd("backtest", []string{"--sweep"}, cache(), years)})
	}
	if !o.SkipActual {
		for _, level := range cfg.Universe.SweepMinCapBillions {
			label := "índice completo"
			if level != 0 {
				label = fmt.Sprintf("cap. >= US$ %g B", level)
			}
			steps = append(steps, Step{"Top actual - " + label, cmd("actual", cache(), universeArgs(level))})
		}
	}
	if !o.SkipEtf {
		steps = append(steps, Step{"Ranking de ETFs de Vanguard", cmd("etf")})
	}
	if !o.SkipQuant {
		steps = append(steps, Step{"Modelo por score: scores de momentum, salud y técnico, y Markowitz", cmd("quant", cache())})
	}
	steps = append(steps, Step{"Exportar JSON del frontend", cmd("export")})
	if !o.SkipPrune {
		steps = append(steps, Step{"Limpieza de datos y reportes viejos", cmd("prune")})
	}
	return steps
}

type result struct {
	name string
	rc   int
	took time.Duration
}

// Main corre el pipeline con los argumentos de la línea de comandos y devuelve el código de salida.
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
	fs.BoolVar(&o.SkipEtf, "skip-etf", false, "Omite el ranking de ETFs de Vanguard")
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

	dir, err := os.MkdirTemp("", "top3-pipeline-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	summaryFile := filepath.Join(dir, "summary.jsonl") // cada paso deja ahí sus avisos y errores

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
		results = append(results, result{s.Name, rc, time.Since(t0)})
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
		if r.rc != 0 {
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
