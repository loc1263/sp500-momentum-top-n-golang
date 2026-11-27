# sp500-momentum-top-n-golang

**Momentum y Scores.** Estrategia de momentum sobre el S&P 500: cada N meses se compran, con ponderación igual, las N
acciones que más subieron en los últimos N meses. El proyecto la evalúa con datos históricos (backtest), mide cuánto del
resultado depende del mes de arranque y muestra todo en un sitio web estático. Además rankea ETFs y acciones con un modelo
por score, para EE. UU.

**Ver el sitio: https://loc1263.github.io/sp500-momentum-top-n-golang/** (se actualiza solo cada día hábil).

> **Aviso:** es una herramienta de análisis, no una recomendación de inversión. El backtest cobra solo una comisión
> de 0,05% por lado, sin impuestos, y tiene sesgos de datos (ver [Limitaciones](TECHNICAL.md#limitaciones)).

## Qué incluye

La barra de pestañas tiene cinco grupos: **Momentum** (Top actual), **ETFs**, **Acciones por score**, **Análisis fundamental** (Evaluador de empresas) y **Visor**. El laboratorio de backtest está en una página aparte (`lab.html`), sin enlace desde el menú.

- **Top actual**: las N acciones líderes hoy, con suplentes, cambios y próximo rebalanceo.
- **ETFs por gestora**: los ETFs de iShares, Vanguard, SPDR, Invesco y Schwab, con momentum y calidad (costo y tamaño).
- **Modelo por score (EEUU)**: score de momentum, de salud financiera y técnico para cada acción del S&P 500, y Markowitz sobre las mejores.
- **Evaluador de empresas**: análisis fundamental propio de cada acción del S&P 500 (rentabilidad, crecimiento, caja, deuda, valoración, tendencia y comparación con competidores), con un veredicto por categoría. Es independiente de los scores.
- **Visor**: rendimientos de 1 a 12 meses y de 1 a 5 años, Sharpe, beta, caída máxima y comparación contra SPY de las acciones de EE. UU. con capitalización mayor a 100 B y de los 300 ETFs de mayor AUM, con búsqueda, filtros y orden por columna, fila marcada con un clic y descarga en CSV.
  Incluye la tabla **Más de 50 años**: acciones de EE. UU. de empresas fundadas hace más de 50 años, con ticker, sector, año de fundación, el rendimiento de cada uno de los últimos 5 años calendario y el anualizado y acumulado a 5 años.
- **Lab** (página aparte, `lab.html`): el laboratorio de backtest de la estrategia de momentum.
  - **Backtest** Point-in-Time, con curva de capital y selecciones en cada rebalanceo.
  - **Plazos y sensibilidad**: compara plazos de rebalanceo y mide cuánto depende el resultado del mes de arranque.
  - **N-0 vs. N-1**: el modelo de siempre frente a una variante que ignora el último mes al rankear.
- **About**: tarjetas que explican los conceptos (momentum, Sharpe, Markowitz, mínima varianza, scores...), las fuentes y los límites.

## Requisitos

- [Go](https://go.dev/dl/) 1.26 o superior
- [Node.js](https://nodejs.org/) 22.12 o superior (para armar el sitio con Astro)
- Conexión a internet

## Cómo ejecutarlo

```
git clone https://github.com/loc1263/sp500-momentum-top-n-golang.git
cd sp500-momentum-top-n-golang/backend
```

**1. Generar los datos** (la primera vez tarda varios minutos; luego solo actualiza lo que falta)

```
go run ./cmd/topwx pipeline
```

El pipeline termina dejando en `frontend/` (proyecto Astro, en la raíz del repositorio) los datos de cada página. Con `--skip-site` no los genera.
`go run ./cmd/topwx site` los regenera a partir de los JSON de `backend/output/`, sin volver a descargar nada.

**2. Abrir la web**

```
cd ../frontend
npm install        # solo la primera vez
npm run dev        # desarrollo: http://localhost:4321
```

Para el sitio estático final: `npm run build` (queda en `frontend/dist`) y `npm run preview` para verlo. Para actualizar los resultados, repita el paso 1.

## Problemas frecuentes

| Síntoma | Solución |
|---|---|
| Una página dice «No se pudo cargar el JSON...» o «Sin Top actual» | Falta el paso 1: ejecute `go run ./cmd/topwx pipeline` en `backend/` y vuelva a generar el sitio |
| El navegador no abre `localhost` | Use `http://127.0.0.1:4321` o elija otro puerto con `npm run dev -- --port 3000` |
| Errores `TOP-NET-...` durante la descarga | Suelen ser límites de Yahoo Finance o de red; reintente más tarde |

## Más información

- [`TECHNICAL.md`](TECHNICAL.md): cómo funciona, comandos y opciones, modelo por score, configuración, base de datos,
  códigos de error, estructura y tests.
- [`backend/README.md`](backend/README.md): paquetes del backend y rendimiento.

Los datos vienen de Yahoo Finance (vía [`gohoo`](https://github.com/loc1263/gohoo)), de
[`hanshof/sp500_constituents`](https://github.com/hanshof/sp500_constituents) y de Wikipedia. Proyecto de uso personal
y educativo.

## Licencia

[MIT](LICENSE)
