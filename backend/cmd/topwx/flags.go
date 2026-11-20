// Opciones comunes de los comandos (-config), carga de la configuración con su log y cliente de Yahoo.
//
// Fecha: 2026-11-19

package main

import (
	"flag"
	"fmt"
	"os"
	"topwx/internal/config"
	"topwx/internal/errs"
	"topwx/internal/history"
	"topwx/internal/logx"
	"topwx/internal/yahoo"
)

type flags struct {
	*flag.FlagSet
	config string
}

func newFlags(name, description string) *flags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\nUso: topwx %s [opciones]\n\n", description, name)
		fs.PrintDefaults()
	}
	f := &flags{FlagSet: fs}
	fs.StringVar(&f.config, "config", "config.toml", "Archivo de configuración")
	return f
}

func (f *flags) passed(name string) bool {
	found := false
	f.Visit(func(fl *flag.Flag) { found = found || fl.Name == name })
	return found
}

func (f *flags) parse(args []string) (stop bool, err error) {
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return true, err
	}
	return false, nil
}

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
	if err := history.Restore(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func newYahoo(cfg config.Config) (*yahoo.Client, error) { return yahoo.New(cfg.Data.UserAgent) }
