#!/usr/bin/env python3
"""Como start.py, pero antes corre el pipeline completo del backend (go run ./cmd/top3 pipeline) y luego levanta el frontend.

    python start_with_pipeline.py                  # pipeline + frontend en http://localhost:5173
    python start_with_pipeline.py --port 3000      # otro puerto
    python start_with_pipeline.py --check          # solo valida, no corre nada

Es start.py con --refresh siempre activo: las validaciones y las opciones son las mismas. La primera vez el pipeline tarda varios
minutos; después reutiliza lo ya descargado. Ctrl+C detiene el servidor.
"""
import importlib.util
import sys
from pathlib import Path


def load_start():
    """Carga start.py desde esta carpeta (no depende del directorio actual ni de sys.path)."""
    path = Path(__file__).resolve().parent / "start.py"
    spec = importlib.util.spec_from_file_location("start", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


if __name__ == "__main__":
    start = load_start()
    # con --check solo se valida: start.py correría el pipeline antes de revisar esa opción
    if "--refresh" not in sys.argv[1:] and "--check" not in sys.argv[1:]:
        sys.argv.append("--refresh")
    sys.exit(start.main())
