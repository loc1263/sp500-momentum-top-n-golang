//go:build !windows

package logx

import "os"

func enableVT(*os.File) {}

// ConfigureConsole no hace nada fuera de Windows (las terminales ya son UTF-8).
func ConfigureConsole() {}
