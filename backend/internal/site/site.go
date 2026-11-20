// Package site genera el sitio estático sin Node ni bundler: Go arma el HTML de cada página (tablas ya formateadas, gráficos SVG) con
// html/template y copia un CSS y unos JS planos que filtran, ordenan y paginan lo que ya está en la página. El evaluador y el Lab son
// una página por vista que Alpine pinta con datos ya formateados por Go (empresas.js, lab-datos.js).
//
// Fecha: 2026-11-20
package site

import (
	"context"
	"embed"
	"errors"
	"net/http"
	"time"
)

//go:embed templates/*.html static/*
var assets embed.FS

// Serve sirve dir por HTTP hasta que ctx se cancele (para ver el sitio en local).
func Serve(ctx context.Context, dir, addr string) error {
	srv := &http.Server{Addr: addr, Handler: http.FileServer(http.Dir(dir)), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
