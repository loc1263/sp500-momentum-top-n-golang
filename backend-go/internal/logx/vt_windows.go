//go:build windows

// Configuración de consola en Windows: habilita las secuencias de terminal virtual para los colores.
//
// Fecha: 2026-11-19

package logx

import (
	"os"

	"golang.org/x/sys/windows"
)

func enableVT(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) == nil {
		_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

func ConfigureConsole() {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil {
			_ = windows.SetConsoleOutputCP(65001)
			return
		}
	}
}
