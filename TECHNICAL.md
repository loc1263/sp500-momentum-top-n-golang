# Documentación técnica

Detalle de funcionamiento, comandos, configuración, base de datos y códigos de error de
`sp500-momentum-top-n-golang`. Para instalar y ejecutar el proyecto, ver el [README](README.md); para los paquetes del
backend y el rendimiento, [`backend/README.md`](backend/README.md).

## Cómo funciona

1. **Composición del índice (Point-in-Time).** Se descarga el CSV histórico del repositorio
   [`hanshof/sp500_constituents`](https://github.com/hanshof/sp500_constituents), que llega hasta cierta
   fecha. Lo posterior se completa con las altas y bajas de la tabla de cambios de Wikipedia
   ([Historical components of the S&P 500](https://en.wikipedia.org/wiki/Historical_components_of_the_S%26P_500)).
   La lista actual de Wikipedia se usa para verificar y para fechas posteriores al último cambio.
2. **Alias de tickers.** Las tablas usan símbolos distintos tras un renombre (por ejemplo `BK` y `BNY`);
   `[tickers.aliases]` los unifica con el símbolo vigente en Yahoo.
3. **Precios.** Se bajan con `gohoo` (precios ajustados, un ticker por request) para todos los tickers que estuvieron en el índice
   durante el histórico, en lotes de 100. Si Yahoo rechaza tickers por rate limit, se reintentan hasta 3 veces
   esperando 10, 20 y 40 s (`TOP-NET-006`); si siguen rechazados quedan sin datos y se informa como error
   (`TOP-NET-005`). Los tickers dados de baja (esperables) solo quedan en el log a nivel DEBUG y se anotan en
   `no_data_tickers`: no se vuelven a pedir hasta pasados `[data] no_data_ttl_days` días (30), ni siquiera con
   `--no-cache`.
4. **Backtest.** En cada fecha de rebalanceo se rankean las acciones del índice vigentes ese día por retorno
   de los últimos N meses (N x 21 ruedas), se eligen las `top_n` y se mantienen N meses. El retorno del
   período es el promedio simple de las acciones elegidas. Se cobra una comisión por lado
   (`[backtest] commission`, 0,05%) al armar la cartera y por las posiciones que cambian en cada rebalanceo.
5. **Calendarios de arranque.** Un plazo de N meses se puede empezar en N meses distintos. Se prueban todos
   y se informa el calendario base (primer mes), la mediana y el rango, para medir cuánto del resultado
   depende de la fecha de arranque.
6. **Comparación con skip-month (N-1).** En cada sweep se corre además una variante que arma el ranking con los
   N meses que terminan un mes (21 ruedas) antes de la fecha de rebalanceo, para no premiar rebotes de corto
   plazo. Las compras y el rendimiento siguen contándose desde la fecha de rebalanceo. Usa los mismos precios,
   universos, períodos y fechas de arranque que el modelo de siempre (N-0), que es el que se usa en el Top
   actual. En las corridas de prueba N-0 rindió más que N-1 con el plazo de 4 meses; N-1 solo ganó en plazos
   cortos. El Top actual no calcula la variante N-1. Para ver si una ventaja depende de un solo tramo del
   mercado, la pestaña *N-0 vs N-1* parte cada período en dos mitades (por fecha) y muestra el CAGR mediano de
   cada modelo en cada una, a partir de las curvas de capital ya guardadas (sin simulaciones nuevas): si un modelo
   gana en las dos mitades, su ventaja se sostiene. Con períodos cortos cada mitad tiene muy pocos rebalanceos.
7. **Caché.** Todo se guarda en SQLite. Si se vuelve a correr el mismo día se reutiliza lo descargado;
   `--no-cache` fuerza una descarga nueva, pero no repite dentro de la misma corrida lo que ya bajó (p. ej. las
   acciones en circulación), y `topwx pipeline` lo aplica solo al primer paso que descarga: los siguientes leen de
   SQLite lo que ese paso acaba de guardar. Las acciones en circulación cambian por trimestre, así que se
   reutilizan hasta `[data] shares_ttl_days` días (30 por defecto) aunque sean de otro día. Si es otro día, los
   precios se actualizan de forma incremental: solo se baja lo que falta desde la última descarga (tickers
   nuevos completos; un ticker que pagó dividendo o hizo split desde entonces se vuelve a bajar entero, porque su
   precio ajustado cambió). Una descarga completa solo ocurre con `--no-cache`, sin datos guardados o si se pide
   más histórico que el guardado (`history_years` mayor).

### Universo y capitalización

Por defecto compite todo el índice. Con `--large-cap` o `--min-cap-billions N` solo compiten las empresas con
capitalización aproximada de al menos N miles de millones de US$ (10 = large-cap + mega-cap, 200 = mega-cap).
La capitalización se aproxima con **precio de esa fecha x acciones en circulación actuales**, porque Yahoo
no entrega la serie histórica de acciones gratis. La columna `Cap. aprox.` de las tablas es solo informativa
cuando no se filtra.

### Limitaciones

- **Sesgo de supervivencia.** Los tickers que ya no cotizan (adquiridos o eliminados) no tienen precios en
  Yahoo y quedan fuera del backtest. Los retornos probablemente están inflados.
- **Look-ahead en el filtro de capitalización.** Usar las acciones de hoy hace que empresas que hoy son
  grandes entren al universo antes de tiempo. Pesa mucho más en mega-cap que en large-cap.
- **Pocas decisiones.** Con 5 años hay entre 5 y 20 rebalanceos por plazo, y el resultado depende de unas
  pocas rachas. Por eso se informa la mediana y el rango de los calendarios y no un solo número.
- **Costos parciales.** Cobra una comisión por lado (`[backtest] commission`, 0,05%) al armar la cartera y por las
  posiciones que cambian en cada rebalanceo; no incluye impuestos ni deslizamiento (slippage), no cobra el
  reequilibrio de pesos de las que se mantienen y asume ponderación igual.
- **Dependencias externas.** Los datos vienen de Yahoo Finance, un repositorio de GitHub y Wikipedia. Si
  cambia el formato de alguna fuente, el script avisa en el log y sigue con lo que tenga guardado en SQLite.
- Un rebalanceo se decide con precios hasta esa misma fecha de rebalanceo.

## Uso

Los comandos del backend se corren desde `backend/` (las rutas del config son relativas a esa carpeta). Abajo `topwx` es
`go run ./cmd/topwx` o el binario que deja `go build -o bin/topwx ./cmd/topwx`.

### Generar y abrir el sitio

El sitio tiene dos partes: Go deja los datos de cada página y Astro (`frontend/`) los pinta en HTML estático.

```
topwx pipeline                          # datos + JSON + datos del sitio en ../frontend (src/data y public)
topwx site                              # regenera esos datos desde los JSON de output/ (sin descargar nada)
topwx site --out ../frontend --report output/report.json --visor output/visor.json --evaluator output/evaluator.json
cd ../frontend
npm install                             # solo la primera vez
npm run dev                             # desarrollo en http://localhost:4321
npm run build                           # sitio estático en frontend/dist (npm run preview para verlo)
```

`topwx site` (paquete `internal/site`) lee `output/report.json` (Top actual, ETFs, modelo por score, backtest y About), `output/evaluator.json`
(Evaluador) y `output/visor.json` (Visor), que dejan `topwx pipeline`, `topwx export`, `topwx evaluator` y `topwx visor`, y escribe en el proyecto
Astro un JSON por página en `frontend/src/data/` (tablas ya formateadas, gráficos SVG y números es-AR) y, en `frontend/public/`, `empresas.js`,
`lab-datos.js` y `evaluador.xlsx`. Si falta un JSON, esa página dice qué comando correr y el resto se genera igual. Primero hay que haber corrido
el pipeline al menos una vez. Esos datos no se versionan. Astro (`frontend/src/pages`, `layouts`, `components`) los convierte en una página `.html` por
vista, sin servidor, y `frontend/dist` se puede publicar en cualquier hosting estático. En una subruta (GitHub Pages) se arma con
`SITE_BASE=/<repositorio> npm run build`. Requiere Node.js 22.12 o superior.

Para no generar cientos de archivos, el evaluador y el Lab no tienen una página por empresa ni por universo y período: son una página por vista
y Go deja los datos ya formateados en un `.js` (`empresas.js`, `lab-datos.js`) que Alpine (integración `@astrojs/alpinejs`) pinta en el navegador. La
empresa, el universo y el período se eligen con el hash de la dirección (`empresa.html#AAPL`, `lab-backtest.html#200-10`). Los scripts de Astro
son módulos, así que el sitio no abre con `file://`: use `npm run preview` o un servidor estático. Sin JavaScript, esas páginas no muestran contenido.

### Pipeline completo

```
cd backend
topwx pipeline                           # todo: actualiza datos, backtest, Top actual, JSON y datos del sitio (luego npm run build en frontend/)
topwx pipeline --tests          # corre antes los tests
topwx pipeline --no-cache       # descarga todo de nuevo (solo el primer paso que descarga; los demás leen lo que acaba de guardar)
topwx pipeline --years 7        # backtest de un único período
topwx pipeline --skip-backtest  # solo Top actual + JSON (rápido)
topwx pipeline --skip-actual    # omite el Top actual
topwx pipeline --skip-etfq      # omite los ETFs por gestora
topwx pipeline --skip-quant     # omite el modelo por score
topwx pipeline --skip-evaluator # omite el evaluador de empresas
topwx pipeline --skip-visor     # omite el visor de acciones grandes y ETFs
topwx pipeline --skip-site      # no genera el sitio estático al final
```

Cada paso corre en su propio proceso y, si uno falla, se corta. Al final imprime el tiempo de cada paso.
Las excepciones son el evaluador de empresas y el visor, que son pasos **opcionales**: si falla (por ejemplo por un límite de Yahoo) se avisa y el pipeline sigue, así el resto se exporta y se publica igual; el resumen lo marca como «FALLÓ (opcional, se siguió)» y el código de salida no cambia.

### Scripts sueltos

```
topwx backtest                       # backtest normal (plazo de config.toml)
topwx backtest --months 3            # backtest normal con otro plazo
topwx backtest --sweep               # todo: 3 universos x (sweep + offsets) 
topwx backtest --sweep --large-cap   # solo ese universo (sweep + offsets)
topwx backtest --offsets             # solo el detalle de calendarios de arranque (universo completo)
topwx backtest --no-cache            # vuelve a descargar todo
topwx backtest --large-cap           # solo large-cap + mega-cap (>= 10 B)
topwx backtest --min-cap-billions 200  # solo mega-cap (>= 200 B)
topwx backtest --years 5             # un único período de 5 años
topwx backtest --config otro.toml    # otro archivo de configuración
topwx actual                         # Top N de hoy (plazo y top_n del config)
topwx actual --months 6 --top-n 3    # otro plazo / otra cantidad
topwx actual --min-cap-billions 200  # solo mega-cap
topwx etfq                           # ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum y calidad
topwx prune                            # borra reportes, corridas y caché viejos (7 días)
go test ./...                                 # tests del backend
```

Las opciones de universo (`--large-cap`, `--min-cap-billions`) se combinan con `--sweep`, `--offsets`
o el backtest normal.

**`--sweep` solo** corre todas las opciones paramétricas: los universos de `[universe] sweep_min_cap_billions`
(por defecto índice completo, large-cap + mega-cap y mega-cap) y, para cada uno, el sweep y la sensibilidad
al arranque. Si se pasa un filtro explícito
(`--large-cap` o `--min-cap-billions N`), corre solo ese universo.

### Qué muestra cada modo

- **Normal:** el calendario base (arranque en el primer mes), la tabla de selecciones con capital y
  ganancia/pérdida acumulada, la mediana y el rango de CAGR de todos los calendarios posibles del plazo,
  y una tabla con cada calendario.
- **`--sweep`:** una tabla que compara los plazos (los de `[simulation] months`), con el resultado
  del calendario base y la mediana y el rango de CAGR. Ordenado por CAGR mediano. Incluye las selecciones
  de cada plazo. Se repite para cada universo y se acompaña del reporte de sensibilidad al arranque.
- **`--offsets`:** el resumen por plazo (mediana, mínimo y máximo de CAGR y de capital final) y el detalle de
  cada calendario. Dentro de `--sweep` reutiliza los calendarios ya calculados.
- **Comparación N-0 vs. N-1 (dentro de `--sweep`):** después de la sensibilidad al arranque, una tabla por universo
  con el CAGR y el capital final típicos (mediana de los calendarios) del modelo de siempre (N-0) y del skip-month
  (N-1), la diferencia en puntos porcentuales y cuál gana en cada plazo. Las corridas del N-1 se guardan en SQLite
  con `skip_months = 1` y se ven en la pestaña *N-0 vs N-1*.

### Top actual (`topwx actual`)

Aplica a hoy la misma lógica del backtest: toma la lista actual del S&P 500 (con los alias de tickers),
calcula el retorno de las últimas N x 21 ruedas y muestra las `top_n` mejores con ponderación igual.
Usa `[backtest] months` y `top_n` del `config.toml`, y acepta `--months`, `--top-n`, `--large-cap`,
`--min-cap-billions`, `--no-cache` y `--config`. Por defecto: 4 meses y 4 acciones.

Reutiliza la base `data/topwx.db`: si el backtest ya descargó los precios hoy, los lee de ahí y solo baja lo que
falte; si no, descarga la ventana reciente sin tocar el histórico guardado. Cada resultado se guarda en la tabla
`actual_picks`. El último dato de la ventana es el precio más reciente disponible, que durante la
rueda puede ser intradía.

Además del Top, muestra:

- **Suplentes:** los `[backtest] alternates` puestos siguientes al `top_n` (4 por defecto), con el mismo criterio
  de ranking, para evaluar el reemplazo de alguna acción del top. No tienen peso asignado y no afectan el backtest
  ni la comparación con la corrida anterior. Se guardan en la tabla `actual_alternates`.
- **Cambios vs. la corrida anterior:** qué tickers entran, salen y se mantienen respecto de la última selección
  guardada en `actual_picks` (mismo plazo y universo).
- **Próximo rebalanceo:** el primer inicio de mes de la grilla "cada N meses" contada desde la primera corrida
  guardada. Es una referencia: el backtest no tiene un ancla fija para el Top de hoy.
- **Tickers excluidos:** los que tienen huecos de precios en la ventana no se pueden rankear (igual que en el
  backtest) y quedan listados como advertencia en la consola y en el log.
- **Seguimiento de selecciones anteriores (solo en la web):** para cada selección guardada en `actual_picks`, el
  retorno de ponderación igual entre el precio de cuando se eligió y el último precio disponible en la base,
  sin pasar de los N meses que dura la tenencia. Pasado ese plazo figura como *cerrada*. Es una verificación fuera
  del backtest, pero hace falta acumular selecciones (una por corrida con datos de un día nuevo) antes de que diga algo.

Correr el script dos veces con los mismos datos reemplaza el resultado en lugar de duplicarlo.

### Evaluador de empresas (`topwx evaluator`)

Análisis fundamental de cada acción del **S&P 500 actual** (~500 empresas), sin backtest: una foto de hoy. Es una
sección aparte: **no usa ni modifica** los scores, el ranking, las tablas de precios ni `fundamentals` de los otros modelos. De ellos
toma solo las listas de tickers (el S&P 500 actual de la base). Los datos salen de Yahoo Finance
vía `gohoo` (`quoteSummary`, `fundamentals-timeseries` y el historial diario de 1 año: 3 requests por empresa, más uno del S&P 500; ~70 segundos en total) y no se guardan en SQLite:
cada corrida los baja de nuevo y reemplaza el JSON.

```
topwx evaluator             # ~70 segundos
```

El resultado es `[evaluator] output` (`output/evaluator.json`, ignorado por git como `report.json`), que la página
carga solo al abrirse. Trae un diccionario de métricas (nombre, unidad y explicación, una sola vez) y, por empresa, las categorías con
sus métricas, nivel (`good`, `warn`, `bad`, `info`, `na`) y veredicto. Los umbrales están en `internal/evaluator/analyze.go`.

| Categoría | Métricas |
|---|---|
| Rentabilidad | margen neto, margen operativo del último trimestre, ROE, ROIC, margen bruto (informativo) |
| Crecimiento | crecimiento de ingresos y del EPS, calidad de la utilidad (Forward P/E ÷ P/E) |
| Caja | flujo de caja libre, su margen, cobertura del capex, rendimiento sobre la capitalización y payout (solo si paga dividendo) |
| Deuda y liquidez | deuda neta, deuda neta/EBITDA, años de flujo operativo para pagarla, cobertura de intereses, liquidez corriente, deuda/patrimonio |
| Valoración | P/E, Forward P/E, EV/EBITDA, EV/FCF, PEG, P/S y P/B (informativos), tendencia del precio (precio frente a la SMA 200, rendimiento de 3 y 6 meses, distancia al máximo de 52 semanas), potencial al precio objetivo, posición en el rango de 52 semanas, beta, dividendo, consenso (las tres últimas son informativas) |
| Tendencia | ingresos, utilidad, EPS, flujo libre, margen operativo, margen bruto y deuda de hasta 5 años fiscales (CAGR o variación; Yahoo suele traer 4) |
| Vs. competidores | las métricas clave frente a la mediana de hasta `[evaluator] peers` = 3 empresas de la misma industria (si no hay, del mismo sector) y mercado, con la capitalización más cercana |

- **Fuente de los datos de caja y resultado**: las series de 12 meses (`trailing*`) mandan sobre `quoteSummary`, porque `freeCashflow` y `ebitda` de
  `quoteSummary` no cuadran con los estados financieros (en MSFT el FCF de `quoteSummary` era 3 mil millones contra 60 reales). Con esas series se
  calculan el EV (capitalización + deuda − caja; sin dato si las monedas difieren), EV/EBITDA, EV/FCF, deuda neta/EBITDA y ROIC
  (EBIT × (1 − tasa) ÷ capital invertido).
- **Umbrales de valoración**: EV/EBITDA barato bajo 10 y caro sobre 18; EV/FCF bajo 15 y sobre 30; PEG bajo 1 y sobre 2 (como el P/E: bajo 15 y sobre 30).
  Un resultado negativo es «Sin utilidad», «Sin caja libre» o «Sin crecimiento». Deuda neta/EBITDA: bajo 2× bien, sobre 3× alerta. ROIC: 15 % o más bien, bajo 10 % alerta.
  Payout: bajo 60 % bien, sobre 80 % alerta.
- **Sólida y barata** (columna y filtros del listado, `internal/evaluator/screen.go`): *sólida* = Rentabilidad y Caja en «Bien» y solvencia en «Bien» (deuda neta/EBITDA hasta 2× o caja neta y, si hay dato, cobertura de intereses de 8× o más; no usa el veredicto de Deuda y liquidez porque deuda/patrimonio y la liquidez corriente se distorsionan en empresas que recompran acciones o en redes de pago);
  *barata* = al menos dos de P/E, EV/EBITDA y EV/FCF con dato y ninguno en «Caro». No usa el veredicto de Valoración, que también promedia beta y
  potencial al precio objetivo. Se calcula en el sitio a partir de los niveles ya guardados, así que no cambia el JSON. Los bancos no califican como
  sólidos (sin EBITDA). Es un filtro para revisar, no una recomendación.
- **Exportar Excel** (botón del listado, `evaluador.xlsx`): todas las empresas y todas las métricas, sin aplicar los filtros de la página. Cada valor lleva el color
  de su nivel (verde, amarillo, rojo; las informativas y sin dato quedan sin color) y los números van como números con su formato, para ordenarlos y filtrarlos.
  Lo arma `internal/site/evaluator_xlsx.go` con `internal/xlsx` al generar el sitio, así que se actualiza solo con `topwx site` o el pipeline.
- **Tendencia del precio** (`internal/evaluator/momentum.go`, en Valoración; sale del historial diario de 1 año): precio sobre su SMA 200 es «Bien» y bajo ella «Alerta».
  Rendimiento de 3 y 6 meses: «Bien» si es positivo y no queda bajo el S&P 500 en el mismo plazo; «Atención» si cumple solo una de las dos cosas (sube menos que el
  índice, o cae menos que él); «Alerta» si cae y además rinde menos que el índice. Distancia al máximo de 52 semanas: hasta 10 % «Bien», hasta 15 % «Atención», más «Alerta».
  Estas métricas también cuentan en el veredicto de Valoración, y miden el precio y no si la empresa es barata: la columna «Sólida y barata» no las usa.
  Sin historial del S&P 500, el rendimiento solo se compara con cero.
- **ROE**: sobre 60 % queda en «Atención» con la etiqueta «Muy alto», porque suele ser un patrimonio muy pequeño (recompras o pérdidas) y no eficiencia.
- **Veredicto**: «Bien» si el promedio de los niveles es alto (≥ 1,5 sobre 2) y ninguno es alerta, «Alerta» si es bajo (< 0,9). El
  general aplica la misma regla a los veredictos de las categorías. Las métricas informativas y sin datos no cuentan.
- **Bancos y aseguradoras** (sector `Financial Services` sin liquidez corriente): su deuda y su caja son materia prima del negocio,
  así que caja, deuda y liquidez no se evalúan. Visa y Mastercard, que sí informan liquidez, se miden con todo.
- **Monedas**: el precio y la capitalización van en la moneda de cotización (USD u otra); los montos de los estados financieros, en la
  suya (`fin_currency`, algunas empresas extranjeras informan en otra). Si difieren no se calcula el rendimiento de caja libre.
- **Cobertura mínima**: si menos de `[evaluator] min_coverage` (50%) de las empresas tienen datos, no se guarda el resultado y se
  conserva el JSON anterior (`TOP-EVA-002`). Las empresas sin datos se avisan con `TOP-EVA-001`.

### Visor (`topwx visor`)

Tabla de **rendimientos, riesgo y comparación contra SPY** de las acciones de EE. UU. con capitalización mayor a 100 B (mega cap desde
200 B) y de los 300 ETFs de mayor AUM (más los de `[visor] tickers`), sin backtest: una foto de hoy. Es una sección aparte: arma su
propio universo con el screener de Yahoo (`gohoo`) y no usa la base SQLite ni los otros modelos. Cada corrida baja de nuevo el
historial de 6 años (con dividendos) de cada instrumento y reemplaza el JSON (~450 instrumentos, ~50 segundos).

```
topwx visor
```

El resultado es `[visor] output` (`output/visor.json`, ignorado por git), con una tabla por tipo (`acciones`, `etf` y `veteranas`):
columnas, filas (`null` = sin dato) y cómo se formatea y se colorea cada columna; el sitio solo la dibuja.

| Columna | Cálculo |
|---|---|
| `ret_1m` … `ret_12m`, `ret_1y`, `ret_2y`, `ret_3y`, `ret_5y` | retorno acumulado %, con cierres ajustados; un mes = 21 ruedas y un año = 252 |
| `sharpe` | (CAGR a 3 años − `[visor] rf`) ÷ volatilidad anual a 3 años. No es el Sharpe clásico sobre retornos en exceso |
| `beta` | covarianza con SPY ÷ varianza de SPY, retornos diarios de 3 años (mínimo 252 fechas en común) |
| `max_drawdown`, `dist_max52`, `vs_ma200` | mayor caída a 5 años; distancia al máximo de 252 ruedas; precio contra la media de 200 ruedas |
| `rel_spy_1y`, `rel_spy_3y`, `ytd_vs_spy`, `vs_spy` | rendimiento menos el de SPY; `SUPERA` si el YTD supera al de SPY |
| `dividend_yield` | dividendos de los últimos 365 días ÷ último cierre |
| `verdes` | cantidad de celdas verdes de la fila (mismas reglas que el color de la tabla, definidas en `internal/visor/table.go`) |

- **Más de 50 años** (`veteranas`): acciones de EE. UU. con capitalización mayor a `[visor] veteran_cap_min_billions` (5 B) de empresas
  fundadas hace al menos `[visor] veteran_years` (50) años. Yahoo no tiene un campo de fundación: el año sale del resumen del negocio
  (`longBusinessSummary`) buscando «founded / established in AAAA» y, si no hay, «incorporated / organized / formed in AAAA»
  (`visor.FoundedYear`); si el resumen no lo dice, la empresa no aparece. «Incorporated» es la constitución legal y a veces no
  coincide con la fundación. Como pedir el `Info` de miles de acciones topa con el límite de Yahoo y el año no cambia, se guarda en
  `[visor] veteran_cache` (`data/founded.json`, que el workflow restaura entre corridas) y cada corrida solo consulta las que faltan;
  las que Yahoo no contesta se reintentan en la siguiente (la primera corrida es lenta). Columnas: `ticker`, `nombre`, `sector`,
  `fundada` (año), `anio_AAAA` (rendimiento % de cada uno de los últimos 5 años calendario completos: cierre de diciembre contra el de
  diciembre anterior, sin el año en curso), `ytd` (rendimiento del año en curso), `anualizado_5a` y `acumulado_5a` (esos cinco rendimientos compuestos y su media
  geométrica anual; `null` si falta alguno) y `market_cap_B`. No tiene `verdes` ni comparación contra SPY.
- **Color**: los rendimientos, `vs_ma200` y los relativos son verdes si son > 0 y rojos si son < 0. Verde también: Sharpe > 1, caída
  máxima ≥ −25%, distancia al máximo ≥ −10%, dividendo ≥ 2% y TER ≤ 0,20% (solo ETFs).
- **Apalancados**: se descartan los ETFs apalancados e inversos del universo (por nombre y categoría, `visor.IsLeveraged`), salvo los de
  `[visor] tickers` o con `include_leveraged = true`. Las acciones preferentes (sin capitalización propia) quedan fuera.
- **Cobertura mínima**: si menos de `[visor] min_coverage` (50%) de los instrumentos de un tipo tienen datos, no se guarda nada y se
  conserva el JSON anterior (`TOP-VIS-004`). Los que no tienen historial se avisan con `TOP-VIS-003`.

### Modelo por score (`topwx quant`)

Un score para cada acción del S&P 500 actual, **sin backtest**: es una foto de hoy. Corre dentro del pipeline, deja todo en
SQLite (`quant_runs`, `fundamentals` y `ohlcv`) y en el JSON que lee la web.

```
topwx quant             # ~1-2 minutos (baja datos financieros y OHLCV de ~500 empresas)
topwx quant --no-cache  # vuelve a descargar precios, OHLCV y datos financieros
```

1. **Score de momentum (0-100)**: promedio del ranking de cada acción en el retorno de 12 y de 6 meses, sin contar el último
   mes (momentum 12-1 y 6-1), entre todas las que tienen un año completo de precios.
2. **Score de salud financiera (0-100)**: se bajan de Yahoo Finance los indicadores de los últimos estados financieros de
   cada empresa (se reutilizan `[quant] fundamentals_ttl_days` días) y se rankean en cuatro grupos; cada grupo promedia su
   ranking en los indicadores que tenga y la salud promedia los grupos disponibles:
   - *Rentabilidad*: ROE, ROA, margen operativo y margen neto.
   - *Solvencia*: deuda/patrimonio, deuda neta/EBITDA y caja/deuda (menos deuda es mejor; patrimonio negativo o EBITDA no
     positivo con deuda neta cuentan como lo peor).
   - *Liquidez*: liquidez corriente y prueba ácida.
   - *Generación de caja*: flujo de caja libre (FCF) y flujo operativo sobre ventas, FCF sobre la capitalización, conversión de
     la utilidad en caja (FCF / utilidad neta, entre -100% y 200%; sin utilidad positiva no se mide), flujo operativo sobre la
     deuda y consistencia (parte de los últimos 4 años, de los estados anuales, con FCF positivo; hacen falta al menos 3).
     Una empresa que genera caja de forma sostenida es más confiable, así que este grupo pesa el doble en la salud
     (`[quant] health_weights`, en el orden rentabilidad, solvencia, liquidez y caja).

   Los bancos y aseguradoras (sector *Financial Services* sin liquidez corriente informada) no tienen deuda neta, liquidez ni flujo de caja comparables, así que
   solo se miden con rentabilidad. El selector «Detalle» de la tabla de la web muestra, en lugar de los indicadores, el score (0-100) de
   cada grupo de salud (y los «Grupos medidos», de 4) o de cada grupo técnico.
3. **Score técnico (0-100)**: se bajan de Yahoo los precios de apertura, máximo, mínimo y cierre ajustados y el volumen del
   último año y medio (se guardan en `ohlcv` y se renuevan una vez por día). Cada indicador se traduce a una señal de 0 a 1
   (1 = lo más alcista); las señales se promedian en cuatro grupos y el score es la posición de la acción según el
   promedio de los grupos:
   - *Tendencia*: precio sobre las EMA de 20, 50 y 200, EMA 20 sobre la 50 y la 50 sobre la 200, y dirección y fuerza de la
     tendencia (ADX de 14 con +DI y -DI).
   - *Impulso*: MACD (12, 26, 9: línea sobre la señal, línea sobre cero e histograma creciente), RSI de 14, estocástico lento
     (14, 3) y posición en las bandas de Bollinger (20, 2).
   - *Volumen*: OBV sobre su EMA de 20 y sobre su nivel de hace 20 ruedas, y MFI de 14.
   - *Posición*: cercanía al máximo de 52 semanas y lugar dentro del rango de 52 semanas.

   El RSI, el estocástico y el MFI no premian cualquier valor alto: favorecen la zona alcista (hasta 70, 80 y 70) y penalizan
   levemente la sobrecompra. Se calculan con una implementación propia (RSI y MACD coinciden con una referencia escrita a
   mano en los tests).
4. **Score final**: promedio de los tres (`[quant] weights` permite darles otro peso). La tabla definitiva ordena por él (las
   acciones sin momentum, por tener menos de un año de precios, sin datos financieros o sin datos técnicos quedan fuera y
   se listan).
5. **Markowitz** sobre las `top_n` (10) primeras, sin posiciones cortas y con un máximo de `w_max` por acción, un mínimo de `w_min` en las que se compran (0 o al menos `w_min`; se quitan de a una las de menor peso y se reoptimiza, una heurística) y de `sector_max` por sector (el sector sale de Yahoo; si con los
   sectores de las 10 acciones no se puede invertir todo, el tope se sube lo mínimo necesario y la web lo avisa). Tres carteras: máximo
   retorno con volatilidad anual de hasta `vol_target`, máximo Sharpe y mínima varianza (si el riesgo objetivo es menor que la
   mínima volatilidad posible, la primera se omite y la web lo avisa). La covarianza es la de Ledoit-Wolf de los últimos 252
   días; el retorno esperado de cada acción es su retorno de los últimos 252 días (limitado a ±100%) con un peso de
   `mu_shrink` más el resto en un retorno de largo plazo (`risk_free` + `equity_premium`), porque el retorno pasado de una
   sola acción es muy ruidoso y el de las líderes de momentum, desmesurado. **Los retornos esperados son una suposición: los
   pesos salen sobre todo del riesgo y deben leerse como orientativos.**

Límites: los datos financieros son una foto (no hay histórico, así que no se puede probar si el score habría funcionado); los
indicadores técnicos de tendencia se parecen mucho al momentum, así que el score final pesa dos veces ideas parecidas;
Yahoo puede tener datos faltantes o desactualizados (el flujo de caja libre de los últimos 12 meses, en particular, a veces no
coincide con los estados anuales); los promedios de rankings tratan igual a todos los indicadores; y el
S&P 500 actual no incluye a las empresas que salieron del índice.

### ETFs por gestora (`topwx etfq`)

Los ETFs de **iShares, Vanguard, SPDR (State Street), Invesco y
Schwab** rankeados por momentum **y por calidad** (costo y tamaño), **sin backtest**. Es la primera fase del modelo
multi-factor de [`docs/modelo-multifactor-etfs.md`](docs/modelo-multifactor-etfs.md): solo TER y AUM.

```
topwx etfq                # corre todo y guarda el resultado (la web lo muestra en «ETFs por gestora»)
topwx etfq --top-n 30     # más filas en la consola (el JSON del sitio trae todos)
topwx pipeline --skip-etfq
```

**De dónde sale cada dato.** Todo viene de Yahoo Finance: las páginas de las gestoras bloquean las descargas automáticas, y el
screener de Yahoo filtra por gestora (`fundFamily`) y trae en la misma consulta la lista de ETFs de EE. UU., el TER
(`netExpenseRatio`, en % anual) y el AUM (`netAssets`, en US$). La categoría (Morningstar, p. ej. «Large Blend») sale de `Info` y
se guarda en SQLite (`[etfq] category_ttl_days`). Los nombres de las gestoras son los de Yahoo (`[etfq] families`).

**Score.**
1. **Momentum:** retorno ajustado por riesgo. Para cada plazo de `[etfq] horizons_months` (3, 6 y 12 meses por defecto):
   `(retorno del plazo - retorno de la referencia libre de riesgo) / (volatilidad anual x raíz(meses/12))`; el score es el
   promedio de esos cocientes. La referencia es `[etfq] risk_free_ticker` (VUSB, bonos ultracortos): sin restarla, un ETF casi sin
   riesgo ganaría siempre. Los ETFs de tipo efectivo (volatilidad anual menor a `[etfq] min_vol`) no rankean. Se calcula para
   todos los ETFs de las cinco gestoras.
2. **Filtro de elegibilidad:** un ETF con TER mayor que `[etfq] max_ter` o AUM menor que `min_aum_millions` queda fuera del ranking
   final (se ve en la web con su motivo). Un dato que falta no excluye.
3. **Posiciones (0-100):** el momentum se compara con todos los ETFs elegibles; el TER (menos es mejor) y el AUM (más es mejor), con los
   de **su misma categoría, sea cual sea la gestora** (así VOO, IVV y SPY compiten entre sí). Se usan posiciones y no Z-scores
   para que unos pocos ETFs enormes no compriman a los demás.
4. **Calidad** = promedio ponderado de las posiciones de TER y AUM (`quality_weights`, 4 y 1 por defecto). Un ETF sin uno de los
   dos usa solo el otro; sin ninguno no tiene score final.
5. **Final** = `(1 - quality_weight)` × momentum + `quality_weight` × calidad (25 % de calidad por defecto).

Las limitaciones están en [`docs/modelo-multifactor-etfs.md`](docs/modelo-multifactor-etfs.md): la categoría de Yahoo es un dato de hoy,
el TER es el último informado y los pesos no están validados (no hay backtest). Un ETF sin historia suficiente para el plazo más largo,
con huecos o de tipo efectivo no entra (se cuenta en la web). Un aviso `TOP-GES-001` indica que una gestora no devolvió ETFs.

## Configuración (`backend/config.toml`)

Los valores de la columna «Por defecto» son los que se usan si la clave no está en el archivo; el `config.toml`
incluido puede traer otros (por ejemplo más períodos y plazos en `[simulation]`).

| Sección | Clave | Por defecto | Descripción |
|---|---|---|---|
| `[data]` | `history_years` | `12` | Años de histórico a descargar (composición y precios); debe superar el mayor período + 1 año de ranking |
| | `db_path` | `"data/topwx.db"` | Base SQLite |
| | `historical_url` | CSV de `hanshof` | Composición histórica del S&P 500 |
| | `wikipedia_url`, `wikipedia_changes_url` | páginas de Wikipedia | Lista actual y altas/bajas del índice |
| | `user_agent` | `"Mozilla/5.0"` | User-Agent de las descargas a Wikipedia |
| | `http_timeout`, `historical_timeout` | `30`, `60` | Timeouts HTTP en segundos (Wikipedia / CSV histórico) |
| | `batch_size` | `100` | Tickers por lote al pedir precios a Yahoo |
| | `shares_workers` | `8` | Hilos en paralelo para las acciones en circulación |
| | `price_overlap_days`, `stale_days`, `price_drift_tolerance` | `7`, `30`, `0.0001` | Actualización incremental: días de solape, días sin cotizar para dar un ticker de baja y diferencia que fuerza a rebajarlo completo |
| | `shares_ttl_days` | `30` | Días que se reutilizan las acciones en circulación guardadas (cambian por trimestre); `--no-cache` las vuelve a pedir |
| | `no_data_ttl_days` | `30` | Días que no se vuelve a pedir un ticker que Yahoo dijo que no existe o está dado de baja (`0` = pedirlos siempre) |
| | `runs_keep_days` | `7` | Días con corridas del backtest que se conservan en SQLite (mínimo 1); ver [Limpieza de la base](#limpieza-de-la-base) |
| `[backtest]` | `start_date`, `end_date` | `""` | Período a evaluar (`"YYYY-MM-DD"`); vacío = automático |
| | `backtest_years` | `5` | Años hacia atrás desde `end_date` si `start_date` está vacío |
| | `periods_years` | `[3, 5, 10]` | Períodos (años) que se simulan; el sitio deja elegir entre ellos. `--years N` corre uno solo |
| | `months` | `4` | Plazo del backtest normal: ranking de N meses y rebalanceo cada N meses |
| | `initial_capital` | `1000` | Capital inicial (US$) |
| | `top_n` | `4` | Cantidad de acciones por rebalanceo |
| | `commission` | `0.0005` | Comisión por lado (0,05%): se cobra al armar la cartera y por las posiciones que cambian en cada rebalanceo |
| | `alternates` | `4` | Suplentes del Top actual: puestos posteriores al `top_n` que se muestran como reemplazo posible (`0` = ninguno) |
| | `trading_days_per_month` | `21` | Ruedas por mes para convertir meses en ventana de ranking |
| `[tickers.aliases]` | `VIEJO = "NUEVO"` | 6 renombres | Unifica símbolos renombrados |
| `[universe]` | `min_cap_billions` | `10` | Umbral de `--large-cap` (miles de millones de US$) |
| | `sweep_min_cap_billions` | `[0, 10, 200]` | Universos que corre `--sweep` por defecto (0 = índice completo) |
| `[output]` | `report_json` | `"output/report.json"` | JSON de datos (Top actual, ETFs, modelo por score y backtest) que lee `topwx site` |
| | `history_file` | `"history/history.json"` | Historial del Top actual, suplentes y ETFs, versionado en git (ver [Historial en JSON](#historial-en-json)); `""` = desactivado |
| | `log_dir` | `"log"` | Carpeta de los logs |
| | `backtest_log`, `actual_log`, `etfq_log`, `quant_log` | `topwx_*.log` | Nombres de los archivos de log |
| `[etfq]` | `horizons_months` | `[3, 6, 12]` | Plazos del retorno ajustado por riesgo; el score es el promedio de sus cocientes |
| | `min_vol` | `0.01` | Volatilidad anual mínima para entrar al ranking (1%): los ETFs por debajo son de tipo efectivo (letras del Tesoro, bonos ultracortos) y quedan fuera; `0` = rankear todos |
| | `risk_free_ticker` | `"VUSB"` | ETF cuyo retorno se resta al de cada ETF (referencia libre de riesgo); `""` = no restar |
| | `families` | iShares, Vanguard, State Street Investment Management (SPDR), Invesco y Schwab ETFs | Gestoras a incluir, como `[[etfq.families]]` con `name` (el `fundFamily` de Yahoo) y `label` (el nombre que se muestra) |
| | `top_n` | `20` | Cuántos ETFs se muestran en la consola (el JSON del sitio trae todos) |
| | `max_ter` | `0.50` | TER máximo en % anual para ser elegible; `0` = sin tope |
| | `min_aum_millions` | `100` | AUM mínimo en millones de US$ para ser elegible; `0` = sin mínimo |
| | `quality_weight` | `0.25` | Peso de la calidad en el score final (0-1); el resto es momentum |
| | `quality_weights` | `[4, 1]` | Peso del TER y del AUM dentro de la calidad |
| | `category_ttl_days` | `30` | Días que se reutiliza la categoría guardada de cada ETF |
| `[quant]` | `weights` | `[1, 1, 1]` | Peso de cada score en el final: momentum, salud financiera y técnico |
| | `health_weights` | `[1, 1, 1, 2]` | Peso de cada grupo en el score de salud: rentabilidad, solvencia, liquidez y caja (un grupo sin datos no cuenta) |
| | `top_n` | `10` | Acciones del ranking final sobre las que corre Markowitz |
| | `vol_target`, `w_max`, `sector_max` | `0.20`, `0.25`, `0.40` | Volatilidad anual máxima de la cartera de máximo retorno, y peso máximo por acción y por sector |
| | `w_min` | `0.10` | Peso mínimo de cada posición (0 o al menos `w_min`; `0` lo desactiva) |
| | `w_step` | `0.01` | Los pesos finales se redondean a múltiplos de `w_step` (`0.01` = porcentajes enteros; `0` = sin redondear) |
| | `mu_shrink`, `risk_free`, `equity_premium` | `0.25`, `0.04`, `0.05` | Retorno esperado = `mu_shrink` x retorno de los últimos 12 meses + el resto x (`risk_free` + `equity_premium`); `risk_free` también para el Sharpe |
| | `fundamentals_ttl_days` | `7` | Días que se reutilizan los datos financieros descargados |
| `[simulation]` | `months` | `[3, 4, 6, 8, 12]` | Plazos que comparan `--sweep` y `--offsets` |

## Salidas y depuración

### Reportes

El backend en Go no genera reportes HTML: los resultados se ven en el sitio (`topwx site`) y por consola.

### Consola

Las tablas de selecciones tienen una fila por rebalanceo: `Fecha`, los tickers `#1`..`#N` con su retorno,
`Cap. aprox. (US$ B)` (informativa), `Capital (US$)` (valor al entrar a esa fila) y `Gan/Pérd` (acumulado
vs. el capital inicial, en verde si es positivo). Se puede desactivar el color con la variable `NO_COLOR=1`.

Los títulos y marcos de los resúmenes (cada paso del pipeline, `RESUMEN` y `RESUMEN DE AVISOS Y ERRORES`) salen en
**verde**. Los avisos del log salen en **amarillo** y los errores en **rojo** (incluidos los mensajes que cortan un
comando). Solo si la salida es una terminal: al redirigir a archivo o pipe, o con
`NO_COLOR=1`, va sin color. El archivo de log nunca lleva códigos de color.

Al terminar cada comando (y `topwx pipeline`, con los de todos sus pasos juntos) se imprime un **resumen de avisos y
errores** agrupado por código, con cuántas veces ocurrió, un ejemplo y qué hacer según la tabla de
[códigos de error](#códigos-de-error-y-aviso); los errores van primero. Si no hubo ninguno, no se imprime nada.

### Historial en JSON

La base SQLite es **solo caché**: todo lo que contiene se puede volver a bajar o calcular, salvo el historial de
`actual_picks`, `actual_alternates` y `etf_scores` (de ahí salen la comparación con la corrida anterior, el calendario de
rebalanceo y el seguimiento). Ese historial se guarda además en `backend/history/history.json`, que **sí se versiona en git**:

- Al terminar cualquier comando que exporta (`backtest`, `actual`, `etf`, `quant`, `export`, `pipeline`) se reescribe el
  archivo con el contenido actual de esas tres tablas (una fila por línea, para que `git diff` se lea bien). Si no hubo
  cambios, no se toca.
- Al arrancar cualquier comando, si la base no tiene historial (base nueva, caché de CI perdido) se restaura desde el archivo.
  Si la base ya tiene historial, no se toca.

Así se puede borrar `backend/data/` sin perder nada importante: solo hay que volver a descargar los precios. Para usar el
historial en otro entorno basta con clonar el repositorio.

### SQLite (`backend/data/topwx.db`)

| Tabla | Contenido |
|---|---|
| `constituents_history` | Composición histórica (CSV) |
| `index_changes` | Altas y bajas de Wikipedia |
| `constituents_current` | Lista actual de Wikipedia |
| `prices` | Precios ajustados por ticker y fecha |
| `no_data_tickers` | Tickers que Yahoo dijo que no existen o están dados de baja y la fecha de consulta (no se vuelven a pedir durante `no_data_ttl_days`) |
| `shares` | Acciones en circulación actuales (para la capitalización aproximada) |
| `meta` | Fechas de descarga y rango de precios (control del caché) |
| `runs`, `selections`, `equity` | Cada corrida: parámetros, selecciones y curva de capital |
| `etf_scores` | Histórico del ranking de ETFs de Vanguard por corrida (fecha de datos, posición, score, volatilidad, caída máxima, distancia a la SMA200, retornos por plazo). Ya no se escribe: se conserva por el historial en git |
| `etfq_category` | Categoría de cada ETF de las gestoras y el día en que se bajó (caché de `[etfq] category_ttl_days`) |
| `etfq_runs` | Último resultado de los ETFs por gestora (JSON: ranking con momentum, TER, AUM y calidad) |
| `fundamentals` | Últimos datos financieros descargados de cada empresa (JSON) y su fecha |
| `ohlcv` | Apertura, máximo, mínimo, cierre ajustado y volumen diarios del último año y medio de cada acción del S&P 500 (para los indicadores técnicos) |
| `quant_runs` | Último resultado del modelo por score (JSON: ranking y Markowitz) |
| `actual_picks`, `actual_alternates` | Resultados de `topwx actual` (fecha de datos, plazo, ticker, retorno, precio, capitalización) y sus suplentes |

### Limpieza de la base

Cada backtest guarda cientos de corridas (parámetros, selecciones y curva de capital) y el informe solo usa la última de
cada combinación del día más reciente. Para que la base no crezca sin límite:

- Al terminar `topwx backtest` (y por lo tanto `topwx pipeline`) se borran las corridas de más de
  `[data] runs_keep_days` días (contados desde el último día con corridas) y las que otra corrida del mismo día
  reemplaza: las que tienen el mismo universo, período, plazo, desfase y modelo (N-0 o N-1), que es lo que
  identifica una corrida en el informe. Es rápido, así que se hace en cada ejecución. Compactar el archivo
  (`VACUUM`) es lo lento: se hace como mucho una vez por día.
- Para forzarlo a mano: `topwx prune` (o `--keep-days N`). Además de las corridas, esa limpieza completa
  (que `topwx pipeline` corre como último paso; se omite con `--skip-prune`) borra:
  - los precios anteriores a la ventana de `[data] history_years`;
  - los datos de caché ya vencidos (acciones en circulación, datos financieros y tickers sin datos), y luego compacta
    la base.
- No se toca `actual_picks` ni `etf_scores` (historial del Top actual y de los ETFs: de ahí salen el calendario de
  rebalanceo, la comparación con la corrida anterior y el seguimiento; pesan muy poco) ni los precios dentro de la
  ventana (el backtest los necesita). Los logs de `log/` se sobrescriben en cada corrida, no se acumulan.

### Depuración

Cada ejecución escribe `log/topwx_backtest.log` (o `log/topwx_actual.log`) con nivel DEBUG: respuestas HTTP, forma de las
tablas, tickers sin datos, filtros de capitalización y retornos por período. El log se sobrescribe en cada
corrida; la consola muestra solo el nivel INFO. Los avisos y errores llevan un ID estable (`TOP-BKT-003: ...`).

## Códigos de error y aviso

Cada aviso, error y excepción del proyecto lleva un ID estable al principio del mensaje, al estilo de Oracle
(`ORA-00942`):

```
2026-10-20 15:31:22,327 [WARNING] TOP-BKT-003: Pocos tickers disponibles en 2016-11-01
```

Formato: `TOP-<ÁREA>-<NNN>`.

| Área | Qué cubre |
|---|---|
| `CFG` | Configuración y fechas |
| `NET` | Descargas (Yahoo, GitHub, Wikipedia) |
| `DAT` | Calidad y faltantes de datos |
| `BKT` | Backtest |
| `ACT` | Top actual |
| `ETF` | Referencia libre de riesgo de los ETFs |
| `GES` | ETFs por gestora |
| `QNT` | Modelo por score |
| `EVA` | Evaluador de empresas |

**Aviso** = se continúa con lo que haya. **Error** = la operación se corta o falla.

Para buscar en el log: `grep TOP-BKT backend/log/topwx_backtest.log`.

| ID | Nivel | Qué significa | Qué hacer |
|---|---|---|---|
| `TOP-CFG-001` | Aviso | No existe el archivo de configuración | Se usan los valores por defecto. Crea config.toml o pasa --config con la ruta correcta. |
| `TOP-CFG-002` | Error | start_date no es anterior a end_date | Corrige [backtest] start_date / end_date o backtest_years en config.toml. |
| `TOP-CFG-003` | Aviso | El período del backtest empieza antes del histórico descargado | Sube [data] history_years (debe superar el mayor período + 1 año). |
| `TOP-CFG-004` | Error | Sección o clave desconocida en la configuración | Casi siempre es un typo en config.toml: compáralo con backend/config.toml. |
| `TOP-NET-001` | Error | Falló la descarga de la composición del índice | Revisa la conexión; se usa lo guardado en SQLite. Reintenta o usa --no-cache. |
| `TOP-NET-002` | Aviso | Se usa la composición guardada en SQLite | Los datos pueden estar desactualizados; vuelve a correr cuando haya conexión. |
| `TOP-NET-003` | Error | Falló la descarga de la lista actual del S&P 500 | Se usa la guardada en SQLite (puede ser vieja). Revisa la conexión. |
| `TOP-NET-004` | Error | No se encontró la tabla de cambios en Wikipedia | Es probable que Wikipedia cambiara el formato de la página; hay que ajustar el lector. |
| `TOP-NET-005` | Error | Yahoo limitó las solicitudes (rate limit) y quedaron tickers sin datos | Ya se reintentó con espera. Vuelve a correr más tarde; los tickers afectados no entran en el ranking. |
| `TOP-NET-006` | Aviso | Yahoo limitó las solicitudes; se reintenta tras una espera | No requiere acción: si el reintento funciona no se pierde nada. |
| `TOP-NET-007` | Aviso | Yahoo informó un error al descargar | Revisa el mensaje original; los tickers afectados pueden quedar sin datos. |
| `TOP-DAT-001` | Aviso | Filas con fecha no interpretable en la tabla de cambios | Se ignoran. Si son muchas, el formato de la fuente cambió. |
| `TOP-DAT-002` | Aviso | Sin composición histórica para esa fecha | Esa fecha queda sin constituyentes; revisa history_years y el CSV histórico. |
| `TOP-DAT-003` | Aviso | Sin dato de acciones en circulación para algunos tickers | Su capitalización queda en n/d y no entran en un universo filtrado por capitalización. |
| `TOP-DAT-004` | Error | No hay lista actual del S&P 500 | No se descargó ni existe en SQLite. Corre con conexión a internet. |
| `TOP-DAT-005` | Aviso | Sin datos financieros para algunos tickers | Quedan fuera del score de salud y del ranking final; suele ser un límite de Yahoo o un ticker nuevo. Vuelve a correr más tarde. |
| `TOP-DAT-006` | Aviso | Sin precios de apertura, máximo, mínimo o volumen para algunos tickers | Quedan fuera del score técnico y del ranking final; suele ser un límite de Yahoo o un ticker nuevo. Vuelve a correr más tarde. |
| `TOP-DAT-007` | Error | La base SQLite es de una versión más nueva del programa | Actualiza el código (git pull) o usa otra base en [data] db_path; no se modifica la base. |
| `TOP-BKT-001` | Error | El backtest no generó ningún rebalanceo | Revisa las fechas, el histórico descargado y que el universo no sea demasiado chico. |
| `TOP-BKT-002` | Aviso | Sin constituyentes en la fecha de rebalanceo | Se omite ese rebalanceo. |
| `TOP-BKT-003` | Aviso | Pocos tickers disponibles en la fecha de rebalanceo | Se omiten esos rebalanceos (menos de 10 candidatos); se avisa una vez por universo y período con el rango de fechas. Frecuente en mega-cap con períodos largos. |
| `TOP-BKT-004` | Aviso | Ventana de precios insuficiente para rankear | Se omite ese rebalanceo. |
| `TOP-BKT-005` | Aviso | Sin precios para mantener la selección durante el período | El retorno de ese período no se computa. |
| `TOP-BKT-006` | Aviso | Un plazo/calendario de arranque no produjo resultado | Se omite ese calendario; el resto continúa. |
| `TOP-ACT-001` | Error | No hay ruedas de precios suficientes | Descarga más historia (--no-cache) o reduce el plazo en meses. |
| `TOP-ACT-002` | Aviso | Tickers con huecos en la ventana quedan fuera del ranking | Misma regla que el backtest. Revisa si Yahoo falló para esos tickers y reintenta. |
| `TOP-ETF-005` | Aviso | La referencia libre de riesgo no está disponible | El score se calcula sin restarla (los ETFs casi sin riesgo quedan arriba). Revisa [etfq] risk_free_ticker: debe ser un ETF de las gestoras y tener historia completa. |
| `TOP-GES-001` | Aviso | Una gestora no devolvió ETFs | Yahoo falló o no conoce ese nombre. Revisa `[etfq] families`: `name` debe ser el `fundFamily` de Yahoo (p. ej. 'iShares'). Las demás gestoras siguen. |
| `TOP-GES-002` | Aviso | ETFs sin categoría | Yahoo no informó la categoría: esos ETFs se comparan entre sí como 'Sin categoría'. Se reintenta en la próxima corrida. |
| `TOP-GES-003` | Error | Ningún ETF de las gestoras se pudo rankear | Revisa la conexión y `[etfq] families`; los ETFs nuevos entran cuando acumulan historia para el plazo más largo. |
| `TOP-QNT-001` | Error | No hay acciones suficientes para el ranking del modelo por score | Descarga más historia (`--no-cache`) o baja `[quant] top_n`; revisa también `TOP-DAT-005`. |
| `TOP-EVA-001` | Aviso | Empresas sin datos fundamentales en Yahoo | Quedan fuera del evaluador y se reintentan en la próxima corrida. |
| `TOP-EVA-003` | Aviso | Una de las listas de empresas está vacía | El evaluador sigue con las demás. Para el S&P 500, corre antes `topwx actual` o el pipeline completo; si no quieres ese universo, pon `[evaluator] sp500 = false`. |
| `TOP-EVA-002` | Error | Se obtuvieron datos de muy pocas empresas | No se guardó el resultado para no reemplazar el anterior con uno incompleto. Suele ser un límite de Yahoo: reintenta más tarde o baja `[evaluator] min_coverage`. |
| `TOP-VIS-001` | Error | No se pudo bajar el historial de SPY | SPY es la referencia de beta y de los rendimientos relativos. Revisa la conexión o reintenta más tarde (suele ser un límite de Yahoo). |
| `TOP-VIS-002` | Error | El screener de Yahoo no devolvió instrumentos | Revisa la conexión y `[visor]` (`cap_min_billions`, `etf_top`, `etf_aum_min_millions`); reintenta más tarde. |
| `TOP-VIS-003` | Aviso | Instrumentos sin historial en Yahoo | Quedan fuera de la tabla; se reintentan en la próxima corrida. |
| `TOP-VIS-004` | Error | Se obtuvieron datos de muy pocos instrumentos | No se guardó el resultado para no reemplazar el anterior con uno incompleto. Suele ser un límite de Yahoo: reintenta más tarde o baja `[visor] min_coverage`. |

## Estructura y desarrollo

```
backend/           # Go: backtest, Top actual, ranking de ETFs, modelo por score, evaluador de empresas, visor, datos del sitio y base SQLite
  config.toml         # configuración
  history/            # datos que se versionan en git: el historial
  cmd/topwx/           # línea de comandos: backtest, actual, etfq, quant, evaluator, visor, export, site, prune y pipeline
  internal/           # la lógica, un paquete por responsabilidad (internal/site genera los datos del sitio)
  output/             # JSON de datos que lee topwx site (report.json, evaluator.json, visor.json); se crea solo
frontend/             # proyecto Astro del sitio estático; src/data y public/*.js los genera topwx site (no se versionan)
```

| Comando | Para qué sirve |
|---|---|
| `topwx backtest` | Backtest Point-in-Time, comparación de plazos y sensibilidad al calendario de arranque |
| `topwx actual` | Top N de hoy: las acciones del S&P 500 con mejor retorno en el plazo configurado |
| `topwx etfq` | ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum y calidad (TER y AUM) dentro de su categoría (sin backtest) |
| `topwx quant` | Modelo por score: scores de momentum, de salud financiera y técnico del S&P 500 y Markowitz sobre las mejores (sin backtest) |
| `topwx evaluator` | Evaluador de empresas: análisis fundamental del S&P 500; escribe `output/evaluator.json` |
| `topwx visor` | Visor: rendimientos, riesgo y comparación contra SPY de las acciones grandes y los ETFs de mayor AUM; escribe `output/visor.json` |
| `topwx pipeline` | Pipeline completo: backtest (todos los períodos y universos) + Top actual de cada universo + ETFs + modelo por score + evaluador + visor + JSON + datos del sitio + limpieza |
| `topwx prune` | Limpieza de lo viejo: corridas del backtest y caché vencida (el pipeline la corre al final) |
| `topwx site` | Genera los datos del sitio en `frontend/` (proyecto Astro; el HTML sale de `npm run build`) |
| `topwx export` | Escribe el JSON en `output/report.json` (lo hacen solos los comandos al terminar) |
| `backend/data/`, `backend/log/` | Base SQLite y logs; se crean solos |

El detalle de los paquetes de `backend/internal/` (config, db, repo, yahoo, market, engine, backtest, actual, etf, etfq, quant,
report, cleanup, pipeline...) está en [`backend/README.md`](backend/README.md).

**Sitio (`backend/internal/site/`)**

| Archivo | Responsabilidad |
|---|---|
| `shell.go`, `templates/layout.html` | Cáscara común: encabezado, pestañas en cinco grupos (Momentum, ETFs, Acciones por score, Análisis fundamental y Visor), Home / About / tema, subtítulo y pie. El Lab tiene su propia cáscara |
| `site.go`, `build.go` | Paquete y servidor (`Serve`); `build.go` lee los JSON (`LoadInputs`), arma cada página y copia los archivos estáticos; si falta un JSON, la página lo avisa |
| `visor_page.go`, `visor_format.go`, `templates/visor.html`, `static/visor.js` | Visor: filas ya formateadas por Go; el JS filtra por columna, ordena, pagina, marca filas y descarga CSV con los valores crudos incrustados |
| `actual_page.go`, `templates/actual.html` | Top actual (Home): un bloque por universo con selector, cambios, suplentes y seguimiento |
| `etfq_page.go`, `templates/etfq.html` | ETFs por gestora: resumen, filtros y tabla de todos los ETFs |
| `quant_page.go`, `templates/quant.html` | Modelo por score: cómo se calcula, Markowitz y ranking con tres vistas de columnas |
| `evaluator_page.go`, `evaluator_xlsx.go`, `evalfmt.go`, `templates/evaluador.html`, `empresa.html`, `static/empresa.js` | Evaluador: listado en Go y una sola página de empresa (`empresa.html#<TICKER>`) que Alpine pinta con los datos de `empresas.js` (minigráficos SVG y pares ya formateados por Go) |
| `lab_page.go`, `chart.go`, `templates/lab_*.html`, `static/lab.js`, `static/chart.js` | Lab: tres páginas (`lab-<vista>.html`) que Alpine pinta con `lab-datos.js` (una entrada por universo y período, elegida con el hash `#<universo>-<años>`); gráficos SVG con cursor y valores |
| `about_page.go`, `templates/about.html` | About: qué se muestra, conceptos, fuentes y límites |
| `format.go`, `table.go` | Formato es-AR (porcentajes, dinero, miles) y las piezas comunes de las tablas (encabezados, filas, celdas, clases de color) |
| `static/table.js`, `static/pager.js` | JS común de las tablas: filtros, búsqueda, orden, paginación (`pager.js` también lo usa el Visor), vistas de columnas y selectores. Sin dependencias ni build |
| `static/site.css`, `static/site.js` | Estilos y tema claro / oscuro |

Los tests están junto al código (`visor_test.go`, `pages_test.go`, `lab_test.go`) y generan el sitio con JSON de ejemplo.

**Agregar algo nuevo.** Un comando: una fila en `commands` (`backend/cmd/topwx/main.go`) y su `runX` en `commands.go`. Una página del sitio: un `<nombre>_page.go` que arma los datos, una plantilla en `templates/`, su paso en `build.go` y, si lleva pestaña, una entrada en `tabs` (`shell.go`); las tablas reutilizan `table.go`, `partials.html` y `static/table.js`.

**Tests.** Backend: un archivo de tests por paquete; no usan red (Yahoo se simula con un servidor local) y los datos son
sintéticos. Los indicadores técnicos, Ledoit-Wolf y Markowitz se comparan contra valores de referencia calculados con herramientas externas.

```
cd backend && go test ./...
```

**Mantenimiento de los códigos de error.** El catálogo está en `backend/internal/errs`. Los IDs no se reutilizan ni se
renumeran; todo aviso o error usa `CODIGO.Msg("texto")` o `CODIGO.Errorf(...)`, y cada código nuevo se documenta en la tabla de arriba.


## Publicación (GitHub Actions y Pages)

`.github/workflows/deploy.yml` corre en días hábiles después del cierre de EE. UU., a mano (`workflow_dispatch`) y en cada
push a `main`:

1. Restaura la base SQLite desde el caché de Actions (`backend/data`), corre los tests y el pipeline.
2. Hace commit de `backend/history/history.json` si cambió (el commit del bot no
   dispara otra ejecución).
3. Arma el sitio con Astro (`npm ci && npm run build` en `frontend/`, con `SITE_BASE` según el nombre del repositorio) y publica `frontend/dist` en GitHub Pages.

Requiere activar Settings > Pages > Source = **GitHub Actions**. Si el caché se pierde (GitHub lo borra tras 7 días sin
uso), la siguiente ejecución vuelve a bajar los precios (varios minutos) y restaura el historial desde el JSON. Si el
pipeline falla (por ejemplo, rate limit de Yahoo), no se publica nada y la web sigue con la versión anterior.
