# Backend en Go

Backend en Go. Usa un `config.toml`,
una base SQLite y el `report.json` que lee el frontend; los datos de Yahoo Finance salen de [gohoo](https://github.com/loc1263/gohoo).

## Requisitos

- [Go](https://go.dev/dl/) 1.26 o superior (sin cgo: SQLite es [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)).
- Conexión a internet para las descargas.

## Cómo ejecutarlo

Desde esta carpeta (las rutas del `config.toml` son relativas a ella):

```
go build -o top3 ./cmd/top3        # top3.exe en Windows
./top3 pipeline                    # todo: backtest, Top actual, ETFs, modelo por score, JSON del frontend y limpieza
```

o sin compilar: `go run ./cmd/top3 pipeline`.

| Comando | Opciones |
|---|---|
| `top3 pipeline` | `[--no-cache] [--years N] [--tests] [--skip-...]` |
| `top3 backtest` | `[--sweep] [--offsets] [--months N] [--years N] [--large-cap] [--min-cap-billions N] [--no-cache]` |
| `top3 actual` | `[--months N] [--top-n N] [--large-cap] [--min-cap-billions N] [--no-cache]` |
| `top3 etf` | `[--as-of YYYY-MM-DD] [--top-n N] [--csv ruta] [--no-update]` |
| `top3 quant` | `[--no-cache]` |
| `top3 export` | |
| `top3 prune` | `[--keep-days N]` |

Todos aceptan `--config otro.toml` y `--help`. `--tests` corre `go test ./...` antes del pipeline.

La base y los logs quedan en `backend-go/data/` y `backend-go/log/`.

## Estructura

```
cmd/top3            punto de entrada: los siete comandos
internal/config     config.toml tipado (claves desconocidas = TOP-CFG-004)
internal/errs       catálogo de códigos TOP-XXX-NNN
internal/logx       log a consola (colores) y a archivo, y resumen de avisos y errores por código
internal/db         SQLite: migraciones (PRAGMA user_version) y metadatos de caché
internal/repo       todo el SQL del proyecto
internal/yahoo      gohoo con ritmo de requests, reintentos ante rate limit y clasificación de errores
internal/market     composición del S&P 500 (CSV + Wikipedia), precios, acciones en circulación y universos
internal/engine     motor del backtest y matriz de precios con ventanas O(1)
internal/backtest   corridas por plazo y calendario de arranque, y su flujo
internal/actual     Top actual y rotación
internal/etf        ranking de ETFs de Vanguard y actualización de la lista
internal/quant      scores de momentum, salud y técnico, Markowitz (puntos interiores) y el JSON del modelo
internal/report     el JSON del frontend
internal/history    historial del Top actual y de ETFs en JSON versionado (la base SQLite es solo caché)
internal/cleanup    limpieza de reportes, corridas y caché vencida
internal/pipeline   el pipeline completo
```

## Detalles de diseño

- **Sin reportes HTML.** La web los reemplaza; `output/` no se llena. El Top actual imprime por consola las notas de rotación.
- **Descargas.** gohoo pide cada ticker por separado: precios con `ChartRange` ajustado por splits y dividendos, acciones en
  circulación por cotizaciones en lote (con `Info` de respaldo) y datos financieros por `Info` + `fundamentals-timeseries`.
  Las requests salen a ≈25 por segundo como máximo y, ante rate limit, gohoo reintenta 3 veces esperando 10, 20 y 40 s
  (`TOP-NET-006`).
- **Markowitz.** Cada cartera es la solución exacta de un programa cuadrático convexo (puntos interiores propios); con pesos
  empatados el orden puede cambiar.
- **Un período sin precios.** Si ninguna de las acciones elegidas tiene datos completos en un período, se avisa
  (`TOP-BKT-005`) y el capital no cambia.
- `TOP-NET-007` cubre cualquier error de Yahoo.

## Rendimiento

Con los datos en caché (sin descargas), sobre la base de 12 años: `backtest --sweep` (1.260 corridas) tarda unos 3,9 s,
`actual` 0,50 s, `quant` 2,3 s y `export` 0,39 s. Lo que lo explica:

- **SQLite en WAL con `synchronous=NORMAL`.** Sin esto, cada corrida guardada sería un `fsync`. La base sigue consistente ante
  cualquier corte; solo un corte de luz puede perder las últimas transacciones. Al lado del `.db` aparecen `-wal` y `-shm`
  mientras hay un comando abierto. Además: caché de 64 MB, archivo mapeado en memoria y temporales en RAM.
- **Inserciones de a 100 filas** por sentencia al guardar selecciones y curva de capital.
- **Memoria de rankings por fecha** (`engine.Memo`): el ranking de un día depende de la fecha, el plazo y el universo, no del
  desfase ni del período, así que las corridas del barrido que caen en la misma fecha lo arman una sola vez.
- **Ordenamiento sin reflexión** (`slices.SortStableFunc`) y **GC menos frecuente** (`GOGC=400` con tope blando de 2 GB).
- **Log a archivo con búfer** de 64 KB (los avisos y errores se vuelcan al instante) y sin armar los mensajes que nadie ve.
- **Carga parcial de precios** en `quant`: solo usa el último año, así que lee 450 días en vez de los 12 años.
- **Tickers sin acciones en circulación** (dados de baja): Yahoo no los informa; se anotan por el día en `meta` y no se
  vuelven a pedir en cada corrida (con `--no-cache` sí).

Lo que no mejoró: leer los ~1,8 millones de precios desde SQLite (≈1,3 s) lo domina el driver; leerlos en paralelo con varias
conexiones no ayudó (se pelean por los mutex del driver), así que se dejó una sola lectura. Para perfilar cualquier comando:
`TOP3_CPUPROFILE=cpu.prof top3 backtest --sweep` y `go tool pprof -top -cum top3 cpu.prof`.

## Tests

```
go test ./...
```

No usan la red: Yahoo se simula con un servidor local. Los indicadores técnicos, la media exponencial con huecos, el score
de salud, la covarianza de Ledoit-Wolf y la frontera de Markowitz se comparan contra valores de referencia calculados con herramientas
estadísticas externas (`internal/quant/testdata/reference.json`).
