#!/usr/bin/env python3
"""Levanta el frontend (Svelte + Vite) tras validar el entorno. Funciona igual en Windows, macOS y Linux.

La página lee frontend/public/data/report.json, que deja el backend en Go; no necesita el backend en marcha.

    python start.py                  # valida y levanta el frontend en http://localhost:5173
    python start.py --port 3000      # otro puerto
    python start.py --refresh        # antes corre el pipeline del backend (go run ./cmd/top3 pipeline)
    python start.py --check          # solo valida, no levanta nada

Ctrl+C detiene el servidor.
"""
import argparse
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
FRONTEND = ROOT / "frontend"
BACKEND = ROOT / "backend-go"
REPORT = FRONTEND / "public" / "data" / "report.json"

MIN_PYTHON = (3, 8)
MIN_NODE = (20, 19)  # el README pide Node 20.19 o superior
STALE_DAYS = 3  # los datos se actualizan cada día hábil; más viejos que esto merecen un aviso

if sys.stdout.isatty() and os.environ.get("NO_COLOR") is None:
    if os.name == "nt":
        os.system("")  # activa los códigos ANSI en la consola de Windows
    COLORS = {"red": "31", "yellow": "33", "green": "32", "cyan": "36"}
else:
    COLORS = {}


def paint(text, color):
    return f"\033[{COLORS[color]}m{text}\033[0m" if color in COLORS else text


def info(text):
    print(paint(text, "cyan"))


def warn(text):
    print(paint(f"Aviso: {text}", "yellow"), file=sys.stderr)


def fail(text):
    print(paint(f"Error: {text}", "red"), file=sys.stderr)
    sys.exit(1)


def version_of(text):
    """(mayor, menor) del primer número de versión que aparezca en text, o None."""
    m = re.search(r"(\d+)\.(\d+)", text)
    return (int(m[1]), int(m[2])) if m else None


def run_capture(cmd):
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return (out.stdout + out.stderr).strip() if out.returncode == 0 else None


def check_python():
    if sys.version_info < MIN_PYTHON:
        fail(f"hace falta Python {'.'.join(map(str, MIN_PYTHON))} o superior (esta es {sys.version.split()[0]}).")


def check_node():
    """Devuelve la ruta de npm tras comprobar que Node y npm existen y que Node es lo bastante nuevo."""
    node, npm = shutil.which("node"), shutil.which("npm")
    if not node or not npm:
        fail("no se encontró " + ("Node.js" if not node else "npm") + " en el PATH. Instálalo desde https://nodejs.org/ "
             f"(versión {MIN_NODE[0]}.{MIN_NODE[1]} o superior; incluye npm).")
    found = version_of(run_capture([node, "--version"]) or "")
    if found is None:
        warn("no se pudo leer la versión de Node.")
    elif found < MIN_NODE:
        fail(f"Node {found[0]}.{found[1]} es muy viejo: hace falta {MIN_NODE[0]}.{MIN_NODE[1]} o superior.")
    return npm


def check_layout():
    if not (FRONTEND / "package.json").is_file():
        fail(f"no existe {FRONTEND / 'package.json'}: ejecuta este script desde la raíz del proyecto.")


def dependencies_outdated():
    """True si falta node_modules o package.json / package-lock.json son más nuevos que lo instalado."""
    marker = FRONTEND / "node_modules" / ".package-lock.json"
    if not marker.is_file():
        return True
    newest = max((p.stat().st_mtime for p in (FRONTEND / "package.json", FRONTEND / "package-lock.json") if p.is_file()), default=0)
    return newest > marker.stat().st_mtime


def install_dependencies(npm):
    info("Instalando dependencias del frontend...")
    # Con package-lock.json, npm ci instala exactamente lo bloqueado; sin él, npm install
    command = "ci" if (FRONTEND / "package-lock.json").is_file() and not (FRONTEND / "node_modules").exists() else "install"
    if subprocess.run([npm, command], cwd=FRONTEND).returncode != 0:
        fail(f"npm {command} falló.")


def check_go():
    """Devuelve la ruta de go tras comprobar que existe y que cumple la versión que pide backend-go/go.mod."""
    go = shutil.which("go")
    if not go:
        fail("no se encontró Go en el PATH (hace falta para generar los datos). Instálalo desde https://go.dev/dl/")
    m = re.search(r"^go\s+(\d+)\.(\d+)", (BACKEND / "go.mod").read_text(encoding="utf-8"), re.M) if (BACKEND / "go.mod").is_file() else None
    wanted, found = (int(m[1]), int(m[2])) if m else None, version_of(run_capture([go, "version"]) or "")
    if wanted and found and found < wanted:
        fail(f"Go {found[0]}.{found[1]} es muy viejo: backend-go/go.mod pide {wanted[0]}.{wanted[1]} o superior.")
    return go


def refresh_data():
    go = check_go()
    info("Generando los datos (go run ./cmd/top3 pipeline); la primera vez tarda varios minutos...")
    if subprocess.run([go, "run", "./cmd/top3", "pipeline"], cwd=BACKEND).returncode != 0:
        fail("el pipeline del backend falló; revisa backend-go/log/.")


def check_report():
    """Valida frontend/public/data/report.json. Devuelve False si no se puede usar."""
    hint = "Ejecuta `python start.py --refresh` (o `go run ./cmd/top3 pipeline` en backend-go/) para generarlo."
    if not REPORT.is_file():
        warn(f"falta {REPORT.relative_to(ROOT)}: la página mostrará un error. {hint}")
        return False
    try:
        report = json.loads(REPORT.read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        warn(f"{REPORT.relative_to(ROOT)} no es un JSON válido ({e}). {hint}")
        return False
    if not isinstance(report, dict) or not {"settings", "actual", "backtest"} <= report.keys():
        warn(f"{REPORT.relative_to(ROOT)} no tiene la forma esperada. {hint}")
        return False
    age = (time.time() - REPORT.stat().st_mtime) / 86400
    if age > STALE_DAYS:
        warn(f"los datos tienen {age:.0f} días. {hint}")
    missing = [name for name, label in (("etf", "ETFs Vanguard"), ("etfq", "ETFs por gestora"), ("quant", "Modelo por score"), ("ipsa", "IPSA (Chile)")) if not report.get(name)]
    if missing:
        warn("el JSON no trae: " + ", ".join(missing) + ". Esas pestañas saldrán vacías hasta correr el pipeline.")
    return True


def port_in_use(port):
    """True si algo ya escucha en el puerto (Vite usa strictPort: no busca otro)."""
    for family, host in ((socket.AF_INET, "127.0.0.1"), (socket.AF_INET6, "::1")):
        try:
            with socket.socket(family, socket.SOCK_STREAM) as s:
                s.settimeout(0.5)
                if s.connect_ex((host, port)) == 0:
                    return True
        except OSError:
            continue  # sin IPv6, por ejemplo
    return False


def check_port(port):
    if not 1 <= port <= 65535:
        fail(f"el puerto {port} no es válido (1-65535).")
    if port_in_use(port):
        fail(f"el puerto {port} ya está en uso. Prueba con otro: python start.py --port {port + 1}")


def console_input_restorer():
    """Guarda el modo de entrada de la consola de Windows y devuelve la función que lo restaura.

    Node/Vite cambian ese modo (entrada "raw") y, si se los mata a la fuerza, lo dejan así: la terminal queda con teclas que no
    responden. En otros sistemas no hace nada."""
    if os.name != "nt":
        return lambda: None
    try:
        import ctypes

        kernel = ctypes.windll.kernel32
        kernel.GetStdHandle.restype = ctypes.c_void_p
        kernel.SetConsoleMode.argtypes = [ctypes.c_void_p, ctypes.c_uint32]
        handle, mode = kernel.GetStdHandle(-10), ctypes.c_uint32()  # -10 = entrada estándar
        if not kernel.GetConsoleMode(ctypes.c_void_p(handle), ctypes.byref(mode)):
            return lambda: None  # la entrada no es una consola (redirigida)
        return lambda: kernel.SetConsoleMode(handle, mode.value)
    except (OSError, AttributeError):
        return lambda: None


def serve(npm, port):
    env = {**os.environ, "FRONTEND_PORT": str(port)}
    print(paint(f"Frontend -> http://localhost:{port}   (Ctrl+C para detener)", "green"))
    # En Windows el proceso hijo se crea en su propio grupo para poder detenerlo entero con Ctrl+C
    flags = subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0
    restore_console = console_input_restorer()
    # Sin entrada (stdin) Vite no toca el modo de la consola ni escucha atajos de teclado (h + enter, etc.)
    proc = subprocess.Popen([npm, "run", "dev"], cwd=FRONTEND, env=env, creationflags=flags, stdin=subprocess.DEVNULL)
    try:
        # En Windows proc.wait() sin plazo bloquea sin atender Ctrl+C: se espera en tramos cortos para que llegue el KeyboardInterrupt
        while True:
            try:
                return proc.wait(timeout=0.5)
            except subprocess.TimeoutExpired:
                continue
    except KeyboardInterrupt:
        if os.name == "nt":  # mata el árbol: npm lanza node y éste a vite
            subprocess.run(["taskkill", "/F", "/T", "/PID", str(proc.pid)], capture_output=True)
        else:
            proc.terminate()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
        return 0
    finally:
        restore_console()


def main():
    parser = argparse.ArgumentParser(description="Valida el entorno y levanta el frontend.", formatter_class=argparse.RawTextHelpFormatter)
    parser.add_argument("--port", type=int, default=int(os.environ.get("FRONTEND_PORT") or 5173), help="puerto del frontend (default: 5173 o $FRONTEND_PORT)")
    parser.add_argument("--refresh", action="store_true", help="corre antes el pipeline del backend para actualizar los datos")
    parser.add_argument("--check", action="store_true", help="solo valida el entorno; no levanta el servidor")
    args = parser.parse_args()

    check_python()
    check_layout()
    npm = check_node()
    if args.refresh:
        refresh_data()
    if dependencies_outdated():
        install_dependencies(npm)
    check_report()
    check_port(args.port)
    if args.check:
        print(paint("Todo en orden.", "green"))
        return 0
    return serve(npm, args.port)


if __name__ == "__main__":
    sys.exit(main())
