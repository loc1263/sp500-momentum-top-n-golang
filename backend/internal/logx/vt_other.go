//go:build !windows

// Configuración de consola en sistemas distintos de Windows (no requiere habilitar secuencias VT).
//
// Fecha: 2026-11-19

package logx

import "os"

func enableVT(*os.File) {}

func ConfigureConsole() {}
