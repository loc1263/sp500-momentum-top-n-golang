//go:build windows

package logx

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT habilita las secuencias ANSI en la consola de Windows.
func enableVT(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) == nil {
		_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

// ConfigureConsole pone la consola de Windows en UTF-8 para que se vean → ≥ … y los acentos (solo si la salida es una
// terminal; al redirigir a archivo o pipe Go ya escribe UTF-8).
func ConfigureConsole() {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil {
			_ = windows.SetConsoleOutputCP(65001)
			return
		}
	}
}
