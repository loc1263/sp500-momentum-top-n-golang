// Los comandos del programa: cada runX define sus opciones, carga la configuración y llama al paquete que hace el trabajo.
//
// Fecha: 2026-11-19

package main

import (
	"context"
	"fmt"
	"strings"
	"topwx/internal/actual"
	"topwx/internal/backtest"
	"topwx/internal/cleanup"
	"topwx/internal/config"
	"topwx/internal/console"
	"topwx/internal/etfq"
	"topwx/internal/evaluator"
	"topwx/internal/market"
	"topwx/internal/quant"
	"topwx/internal/report"
	"topwx/internal/site"
	"topwx/internal/visor"
)

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

func runEtfq(ctx context.Context, args []string) error {
	f := newFlags("etfq", "ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum y calidad (TER y AUM)")
	topN := f.Int("top-n", 0, "Cuántos ETFs mostrar en la consola (default: [etfq] top_n del config)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.EtfqLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	if err := etfq.Run(ctx, cfg, y, etfq.Options{TopN: *topN}); err != nil {
		return err
	}
	_, err = report.Export(cfg)
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

func runEvaluator(ctx context.Context, args []string) error {
	f := newFlags("evaluator", "Evaluador de empresas: análisis fundamental de las acciones del S&P 500")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.EvaluatorLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	payload, err := evaluator.Run(ctx, cfg, y)
	if err != nil {
		return err
	}
	for _, l := range payload.Summary() {
		console.Println(l)
	}
	return nil
}

func runVisor(ctx context.Context, args []string) error {
	f := newFlags("visor", "Visor: rendimientos, riesgo y comparación contra SPY de las acciones grandes y los ETFs de mayor AUM de EE. UU.")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, err := start(f.config, func(o config.Output) string { return o.VisorLog })
	if err != nil {
		return err
	}
	y, err := newYahoo(cfg)
	if err != nil {
		return err
	}
	payload, err := visor.Run(ctx, cfg, y)
	if err != nil {
		return err
	}
	console.Println("Visor: " + payload.Summary())
	return nil
}

func runExport(args []string) error {
	f := newFlags("export", "Exporta los resultados de SQLite a JSON para el sitio")
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

func runSite(ctx context.Context, args []string) error {
	f := newFlags("site", "Genera los datos del sitio (un JSON por página para el proyecto Astro de frontend/) a partir de los JSON del backend")
	paths := site.Paths{}
	f.StringVar(&paths.Visor, "visor", "", "JSON del Visor (default: [visor] output del config)")
	f.StringVar(&paths.Report, "report", "", "JSON del reporte: Top actual, ETFs y About (default: [output] report_json del config)")
	f.StringVar(&paths.Evaluator, "evaluator", "", "JSON del evaluador (default: [evaluator] output del config)")
	out := f.String("out", "../frontend", "Proyecto Astro donde se escriben los datos (src/data y public)")
	if stop, err := f.parse(args); stop {
		return err
	}
	cfg, _, err := config.Load(f.config)
	if err != nil {
		return err
	}
	for path, def := range map[*string]string{&paths.Visor: cfg.Visor.Output, &paths.Report: cfg.Output.ReportJSON, &paths.Evaluator: cfg.Evaluator.Output} {
		if *path == "" {
			*path = def
		}
	}
	in, err := site.LoadInputs(paths)
	if err != nil {
		return err
	}
	dir, err := site.Build(in, *out)
	if err != nil {
		return err
	}
	console.Println("Datos del sitio generados: " + dir + " (para armar el HTML: npm run build en " + *out + ")")
	return nil
}

func runPrune(args []string) error {
	f := newFlags("prune", "Borra corridas del backtest y datos viejos de la base para no llenar el disco")
	keep := f.Int("keep-days", 0, "Días a conservar de corridas del backtest (default: [data] runs_keep_days del config)")
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
	for _, r := range rep.Rows {
		name := r.Name
		console.Println(fmt.Sprintf("%s: %d filas borradas", strings.ToUpper(name[:1])+name[1:], r.N))
	}
	return nil
}
