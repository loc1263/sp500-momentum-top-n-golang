// Command top3 es el punto de entrada único del backend: `top3 <comando> [opciones]`.
//
// Comandos: backtest, actual, etf, quant, export, prune y pipeline (corre todos en orden, cada uno en su propio proceso).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"strings"

	_ "time/tzdata" // zonas horarias de los exchanges también en máquinas sin Go instalado (Windows)

	"top3/internal/actual"
	"top3/internal/backtest"
	"top3/internal/cleanup"
	"top3/internal/config"
	"top3/internal/console"
	"top3/internal/errs"
	"top3/internal/etf"
	"top3/internal/logx"
	"top3/internal/market"
	"top3/internal/pipeline"
	"top3/internal/quant"
	"top3/internal/report"
	"top3/internal/yahoo"
)

var commands = []string{"backtest", "actual", "etf", "quant", "export", "prune", "pipeline"}

func main() {
	// El proceso vive pocos segundos y mueve matrices grandes: se deja crecer el heap antes de cada GC (4x en vez de 2x),
	// con un tope blando de 2 GB para que no se dispare en máquinas chicas.
	debug.SetGCPercent(400)
	debug.SetMemoryLimit(2 << 30)
	logx.ConfigureConsole()
	if len(os.Args) < 2 || !contains(commands, os.Args[1]) {
		fmt.Fprintf(os.Stderr, "Uso: top3 {%s} [opciones]  (--help en cada comando)\n", strings.Join(commands, "|"))
		os.Exit(2)
	}
	name, args := os.Args[1], os.Args[2:]
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if name == "pipeline" {
		os.Exit(pipeline.Main(args))
	}
	// TOP3_CPUPROFILE=archivo escribe un perfil de CPU (go tool pprof) del comando.
	if path := os.Getenv("TOP3_CPUPROFILE"); path != "" {
		if f, err := os.Create(path); err == nil {
			if pprof.StartCPUProfile(f) == nil {
				code := logx.RunCLI(func() error { return run(ctx, name, args) })
				pprof.StopCPUProfile()
				f.Close()
				os.Exit(code)
			}
		}
	}
	os.Exit(logx.RunCLI(func() error { return run(ctx, name, args) }))
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// flags es un conjunto de opciones que sabe si el usuario pasó cada una (para distinguir "0" de "no pasada").
type flags struct {
	*flag.FlagSet
	config string
}

func newFlags(name, description string) *flags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\nUso: top3 %s [opciones]\n\n", description, name)
		fs.PrintDefaults()
	}
	f := &flags{FlagSet: fs}
	fs.StringVar(&f.config, "config", "config.toml", "Archivo de configuración")
	return f
}

// passed dice si la opción name se pasó en la línea de comandos.
func (f *flags) passed(name string) bool {
	found := false
	f.Visit(func(fl *flag.Flag) { found = found || fl.Name == name })
	return found
}

// parse lee los argumentos; --help sale sin error (código 0).
func (f *flags) parse(args []string) (stop bool, err error) {
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return true, err
	}
	return false, nil
}

// start es el arranque de un comando: carga la configuración y abre el log `[output] <logName>`.
func start(path string, logName func(config.Output) string) (config.Config, error) {
	cfg, existed, err := config.Load(path)
	if err != nil {
		return cfg, err
	}
	if err := logx.Setup(logName(cfg.Output), cfg.Output.LogDir); err != nil {
		return cfg, err
	}
	if existed {
		logx.Infof("Configuración cargada desde %s", path)
	} else {
		logx.Warnf(errs.CfgMissing.Msg("No existe %s, uso valores por defecto"), path)
	}
	logx.Debugf("Configuración efectiva: %+v", cfg)
	return cfg, nil
}

func newYahoo(cfg config.Config) (*yahoo.Client, error) { return yahoo.New(cfg.Data.UserAgent) }

func run(ctx context.Context, name string, args []string) error {
	switch name {
	case "backtest":
		return runBacktest(ctx, args)
	case "actual":
		return runActual(ctx, args)
	case "etf":
		return runEtf(ctx, args)
	case "quant":
		return runQuant(ctx, args)
	case "export":
		return runExport(args)
	case "prune":
		return runPrune(args)
	}
	return fmt.Errorf("comando desconocido: %s", name)
}

func runBacktest(ctx context.Context, args []string) error {
	f := newFlags("backtest", "Backtest Top N S&P 500 (Point-in-Time) con caché SQLite")
	noCache := f.Bool("no-cache", false, "Ignora la información guardada hoy y vuelve a descargar todo")
	sweep := f.Bool("sweep", false, "Simula varios plazos (config [simulation] months) y compara el retorno final")
	months := f.Int("months", 0, "Plazo del backtest normal en meses (default: [backtest] months del config)")
	offsets := f.Bool("offsets", false, "Para cada plazo de --sweep prueba todos los calendarios de arranque posibles y reporta mediana y rango")
	largeCap := f.Bool("large-cap", false, "Universo solo large-cap + mega-cap (config [universe] min_cap_billions)")
	minCap := f.Float64("min-cap-billions", 0, "Como --large-cap pero con otro umbral de capitalización (miles de millones de US$); por ejemplo 200 = mega-cap")
	years := f.Float64("years", 0, "Corre un único período de N años (default: todos los de [backtest] periods_years)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.BacktestLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	job := backtest.Job{Sweep: *sweep, Offsets: *offsets, Months: *months, Years: *years, LargeCap: *largeCap, UseCache: !*noCache}
	if f.passed("min-cap-billions") {
		job.MinCapBillions = minCap
	}
	if err := backtest.RunJob(ctx, cfg, y, job); err != nil {
		return err
	}
	if _, err := report.Export(cfg); err != nil {
		return err
	}
	return backtest.PruneAfter(cfg)
}

func runActual(ctx context.Context, args []string) error {
	f := newFlags("actual", "Top N actual del S&P 500 (mismo criterio que el backtest)")
	noCache := f.Bool("no-cache", false, "Ignora lo descargado hoy y vuelve a descargar")
	months := f.Int("months", 0, "Plazo en meses (default: [backtest] months del config)")
	topN := f.Int("top-n", 0, "Cantidad de acciones (default: [backtest] top_n del config)")
	largeCap := f.Bool("large-cap", false, "Solo large-cap + mega-cap (config [universe] min_cap_billions)")
	minCap := f.Float64("min-cap-billions", 0, "Como --large-cap con otro umbral de capitalización (miles de millones de US$)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.ActualLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	if *months == 0 {
		*months = cfg.Backtest.Months
	}
	if *topN == 0 {
		*topN = cfg.Backtest.TopN
	}
	var explicit *float64
	if f.passed("min-cap-billions") {
		explicit = minCap
	}
	cap, _ := market.ExplicitMinCap(cfg, *largeCap, explicit)
	if err := actual.Run(ctx, cfg, y, *months, *topN, cap, !*noCache); err != nil {
		return err
	}
	_, err = report.Export(cfg)
	return err
}

func runEtf(ctx context.Context, args []string) error {
	f := newFlags("etf", "Ranking de momentum ajustado por riesgo de los ETFs de Vanguard")
	asOf := f.String("as-of", "", "Calcula el ranking con los datos hasta esa fecha YYYY-MM-DD (default: hoy)")
	topN := f.Int("top-n", 0, "Cuántos ETFs mostrar en la consola (default: [etf] top_n del config)")
	csvPath := f.String("csv", "", "Otra lista de ETFs (default: [etf] csv del config)")
	noUpdate := f.Bool("no-update", false, "No agrega a la lista los ETFs nuevos de Vanguard (default: los agrega, [etf] update_list)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.EtfLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	if err := etf.Run(ctx, cfg, y, etf.Options{AsOf: *asOf, TopN: *topN, CSV: *csvPath, UpdateList: !*noUpdate}); err != nil {
		return err
	}
	if *asOf == "" { // una consulta a una fecha pasada no se guarda, así que no cambia lo que muestra la web
		_, err = report.Export(cfg)
	}
	return err
}

func runQuant(ctx context.Context, args []string) error {
	f := newFlags("quant", "Scores de momentum, salud financiera y técnico del S&P 500 y Markowitz del top")
	noCache := f.Bool("no-cache", false, "Vuelve a descargar precios, OHLCV y datos financieros en lugar de reutilizarlos")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.QuantLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	payload, err := quant.Run(ctx, cfg, y, !*noCache)
	if err != nil {
		return err
	}
	for _, l := range payload.Summary() {
		console.Println(l)
	}
	_, err = report.Export(cfg)
	return err
}

func runExport(args []string) error {
	f := newFlags("export", "Exporta los resultados de SQLite a JSON para el frontend")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.ExportLog })
	if err != nil {
		return err
	}
	path, err := report.Export(cfg)
	if err != nil {
		return err
	}
	console.Println(path)
	return nil
}

func runPrune(args []string) error {
	f := newFlags("prune", "Borra reportes, corridas y datos viejos para no llenar el disco")
	keep := f.Int("keep-days", 0, "Días a conservar de reportes HTML y corridas del backtest (default: [output] keep_days y [data] runs_keep_days del config)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.PruneLog })
	if err != nil {
		return err
	}
	rep, err := cleanup.Run(cfg, *keep)
	if err != nil {
		return err
	}
	console.Println(fmt.Sprintf("Reportes HTML borrados: %d", len(rep.Files)))
	for _, r := range rep.Rows {
		name := r.Name
		console.Println(fmt.Sprintf("%s: %d filas borradas", strings.ToUpper(name[:1])+name[1:], r.N))
	}
	return nil
}
