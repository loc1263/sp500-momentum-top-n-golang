// Command topwx es el punto de entrada único del backend: `topwx <comando> [opciones]`.
//
// Comandos: backtest, actual, etfq, quant, evaluator, visor, export, site, prune y pipeline (corre todos en orden, cada uno en su propio proceso).
//
// Fecha: 2026-11-19
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strings"

	_ "time/tzdata"

	"topwx/internal/logx"
	"topwx/internal/pipeline"
)

// command es un comando de la línea de comandos. pipeline no está aquí: corre cada paso en su propio proceso y devuelve un código de salida.
type command struct {
	name string
	run  func(ctx context.Context, args []string) error
}

// withoutContext adapta los comandos que no necesitan cancelación.
func withoutContext(run func(args []string) error) func(context.Context, []string) error {
	return func(_ context.Context, args []string) error { return run(args) }
}

// commands es la única lista de comandos: agregar uno nuevo es sumar una fila (y su runX en commands.go).
var commands = []command{
	{"backtest", runBacktest},
	{"actual", runActual},
	{"etfq", runEtfq},
	{"quant", runQuant},
	{"evaluator", runEvaluator},
	{"visor", runVisor},
	{"export", withoutContext(runExport)},
	{"site", runSite},
	{"prune", withoutContext(runPrune)},
}

const pipelineName = "pipeline"

func usage() string {
	names := []string{}
	for _, c := range commands {
		names = append(names, c.name)
	}
	return strings.Join(append(names, pipelineName), "|")
}

func main() {
	debug.SetGCPercent(400)
	debug.SetMemoryLimit(2 << 30)
	logx.ConfigureConsole()
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Uso: topwx {%s} [opciones]  (--help en cada comando)\n", usage())
		os.Exit(2)
	}
	name, args := os.Args[1], os.Args[2:]
	if name == pipelineName {
		os.Exit(pipeline.Main(args))
	}
	i := slices.IndexFunc(commands, func(c command) bool { return c.name == name })
	if i < 0 {
		fmt.Fprintf(os.Stderr, "Uso: topwx {%s} [opciones]  (--help en cada comando)\n", usage())
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runIt := func() error { return commands[i].run(ctx, args) }

	if path := os.Getenv("TOPWX_CPUPROFILE"); path != "" {
		if f, err := os.Create(path); err == nil {
			if pprof.StartCPUProfile(f) == nil {
				code := logx.RunCLI(runIt)
				pprof.StopCPUProfile()
				f.Close()
				os.Exit(code)
			}
		}
	}
	os.Exit(logx.RunCLI(runIt))
}
