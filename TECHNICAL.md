# Documentación técnica

Detalle de funcionamiento, comandos, configuración, base de datos y códigos de error de
`sp500-momentum-top-n-golang`. Para instalar y ejecutar el proyecto, ver el [README](README.md); para los paquetes del
backend y el rendimiento, [`backend-go/README.md`](backend-go/README.md).

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
   acciones en circulación), y `top3 pipeline` lo aplica solo al primer paso que descarga: los siguientes leen de
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

Los comandos del backend se corren desde `backend-go/` (las rutas del config son relativas a esa carpeta). Abajo `top3` es
`go run ./cmd/top3` o el binario que deja `go build -o bin/top3 ./cmd/top3`.

### Levantar todo

```
python start.py                  # valida el entorno y levanta el frontend en http://localhost:5173
python start.py --port 3000      # otro puerto (o FRONTEND_PORT=3000)
python start.py --refresh        # antes corre el pipeline del backend (go run ./cmd/top3 pipeline)
python start.py --check          # solo valida: Python, Node 20.19+, npm, dependencias, puerto y report.json
```

La página no pide nada al backend: lee `data/report.json`, el archivo estático que dejan al terminar
`top3 pipeline`, `top3 backtest`, `top3 actual`, `top3 etf` y `top3 etfq` (también `top3 export` a mano). Primero
hay que haber corrido el pipeline al menos una vez. `npm run build` en `frontend/` genera la versión estática en
`frontend/dist/`, que se puede publicar en cualquier hosting estático con el JSON de ese momento.

### Pipeline completo

```
cd backend-go
top3 pipeline                           # todo: actualiza datos, backtest, Top actual y JSON del frontend
top3 pipeline --tests          # corre antes los tests
top3 pipeline --no-cache       # descarga todo de nuevo (solo el primer paso que descarga; los demás leen lo que acaba de guardar)
top3 pipeline --years 7        # backtest de un único período
top3 pipeline --skip-backtest  # solo Top actual + JSON (rápido)
top3 pipeline --skip-actual    # omite el Top actual
top3 pipeline --skip-etf       # omite el ranking de ETFs de Vanguard
top3 pipeline --skip-etfq      # omite los ETFs por gestora
top3 pipeline --skip-quant     # omite el modelo por score
top3 pipeline --skip-ipsa      # omite el modelo por score del IPSA
```

Cada paso corre en su propio proceso y, si uno falla, se corta. Al final imprime el tiempo de cada paso.

### Scripts sueltos

```
top3 backtest                       # backtest normal (plazo de config.toml)
top3 backtest --months 3            # backtest normal con otro plazo
top3 backtest --sweep               # todo: 3 universos x (sweep + offsets) 
top3 backtest --sweep --large-cap   # solo ese universo (sweep + offsets)
top3 backtest --offsets             # solo el detalle de calendarios de arranque (universo completo)
top3 backtest --no-cache            # vuelve a descargar todo
top3 backtest --large-cap           # solo large-cap + mega-cap (>= 10 B)
top3 backtest --min-cap-billions 200  # solo mega-cap (>= 200 B)
top3 backtest --years 5             # un único período de 5 años
top3 backtest --config otro.toml    # otro archivo de configuración
top3 actual                         # Top N de hoy (plazo y top_n del config)
top3 actual --months 6 --top-n 3    # otro plazo / otra cantidad
top3 actual --min-cap-billions 200  # solo mega-cap
top3 etf                            # ranking de ETFs de Vanguard
top3 etfq                           # ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum y calidad
top3 prune                            # borra reportes, corridas y caché viejos (7 días)
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

### Top actual (`top3 actual`)

Aplica a hoy la misma lógica del backtest: toma la lista actual del S&P 500 (con los alias de tickers),
calcula el retorno de las últimas N x 21 ruedas y muestra las `top_n` mejores con ponderación igual.
Usa `[backtest] months` y `top_n` del `config.toml`, y acepta `--months`, `--top-n`, `--large-cap`,
`--min-cap-billions`, `--no-cache` y `--config`. Por defecto: 4 meses y 4 acciones.

Reutiliza la base `data/top3.db`: si el backtest ya descargó los precios hoy, los lee de ahí y solo baja lo que
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

### IPSA (`top3 ipsa`)

El mismo modelo por score (momentum, salud financiera y técnico, su promedio y Markowitz sobre las mejores) aplicado a las 30
acciones del IPSA de la Bolsa de Santiago, en **pesos chilenos**. Es una sección aparte: no usa ni modifica las tablas de
precios u OHLCV del S&P 500, ni el resultado de `top3 quant`. Reutiliza de `[quant]` los pesos de los scores, el peso de la
salud y `mu_shrink`; lo propio está en `[ipsa]`.

```
top3 ipsa             # ~10 segundos (30 acciones)
top3 ipsa --no-cache  # además vuelve a bajar la lista del IPSA y los datos financieros
```

- **Lista de acciones**: la tabla de Wikipedia (columna Nemotécnico). Se guarda en `[ipsa] universe_file`
  (`history/ipsa_universe.json`, versionado en git) con la fecha, y se renueva cuando tiene más de `universe_ttl_days` días
  (30 = mensual) o con `--no-cache`. Si Wikipedia falla se usa la guardada aunque esté vencida (aviso `TOP-IPS-001` si no hay
  ninguna). El archivo se puede editar a mano; mientras no esté vencido se respeta.
- **Tickers de Yahoo**: nemotécnico + `[ipsa] suffix` (`.SN`); `[ipsa.aliases]` cubre renombres (nemotécnico = símbolo de Yahoo sin sufijo).
- **Moneda y tasa**: precios y capitalización en CLP; tasa libre de riesgo `[ipsa] risk_free` = 5% más `equity_premium` = 6% de prima
  (retorno de largo plazo esperado de 11%). Markowitz sobre las `top_n` = 6 mejores, con volatilidad objetivo, peso máximo por
  acción y por sector propios.
- Yahoo tiene datos financieros menos completos de acciones chilenas que de EE. UU.: las que queden sin algún score salen en `coverage`.

### Modelo por score (`top3 quant`)

Un score para cada acción del S&P 500 actual, **sin backtest**: es una foto de hoy. Corre dentro del pipeline, deja todo en
SQLite (`quant_runs`, `fundamentals` y `ohlcv`) y en el JSON que lee la web.

```
top3 quant             # ~1-2 minutos (baja datos financieros y OHLCV de ~500 empresas)
top3 quant --no-cache  # vuelve a descargar precios, OHLCV y datos financieros
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
5. **Markowitz** sobre las `top_n` (10) primeras, sin posiciones cortas y con un máximo de `w_max` por acción y de `sector_max` por sector (el sector sale de Yahoo; si con los
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

### ETFs de Vanguard (`top3 etf`)

Ranking global de momentum de los ETFs de Vanguard, **sin backtest**: qué ETFs tienen hoy (o a una fecha dada) más
retorno por unidad de riesgo.

```
top3 etf                       # ranking de hoy; muestra el top de [etf] top_n
top3 etf --top-n 30            # más filas en la consola (el JSON del frontend siempre trae todos)
top3 etf --as-of 2025-06-30    # ranking con los datos hasta esa fecha
top3 etf --csv otra_lista.csv  # otra lista de ETFs (esa lista no se actualiza sola)
top3 etf --no-update           # no agrega los ETFs nuevos de Vanguard a la lista
top3 pipeline --skip-etf        # el pipeline lo incluye salvo que se omita
```

**Score.** Para cada plazo de `[etf] horizons_months` (3, 6 y 12 meses por defecto):
`(retorno del plazo - retorno de la referencia libre de riesgo) / (volatilidad anual x raíz(meses/12))`. El score es el
promedio de esos cocientes. La referencia es `[etf] risk_free_ticker` (VUSB, bonos ultracortos): sin restarla, un ETF
casi sin riesgo ganaría siempre. Los ETFs de tipo efectivo (volatilidad anual menor a `[etf] min_vol`, como VBIL, VGUS y la propia VUSB) no entran al ranking: con tan poco riesgo cualquier exceso mínimo sobre la referencia daría un score enorme sin ser una señal de momentum; se listan aparte en la consola y el HTML. Además del score la tabla muestra el retorno de cada plazo, la volatilidad anual y la
caída máxima del plazo más largo, y la distancia del precio a su media de 200 ruedas (positivo = tendencia alcista).

**La lista** está en `backend-go/history/vanguard_etfs.csv` (`ticker,name,category`). Cada vez que se corre el ranking se
consulta el servicio que alimenta la página de productos de Vanguard y se **agregan al final del archivo los ETFs que
falten** (con la categoría que se deduce del estilo que informa Vanguard); las filas que ya existen no se tocan, así
que nombres y categorías se pueden corregir a mano, y nunca se borra ninguna. Si Vanguard ya no lista un ETF de la
lista, se informa en el log para que se quite a mano. Si la consulta falla (sin red, formato distinto) se usa la lista
guardada y se avisa con `TOP-ETF-006`. Es un servicio no documentado de la web de Vanguard, de uso personal. Un ticker sin precios en Yahoo avisa con `TOP-ETF-002`; los ETFs sin historia suficiente para el
plazo más largo quedan fuera del ranking y se listan (`TOP-ETF-003`).

**`--as-of`** usa la lista actual, así que no incluye los ETFs lanzados después de esa fecha ni los cerrados: sirve
para ver cómo se veía el ranking, no como backtest, y no se guarda. Cada corrida normal se guarda en la tabla
`etf_scores` y se compara el top con el de la corrida anterior (entran / salen).

### ETFs por gestora (`top3 etfq`)

Sección aparte de la anterior (no la modifica ni la reemplaza): los ETFs de **iShares, Vanguard, SPDR (State Street), Invesco y
Schwab** rankeados por momentum **y por calidad** (costo y tamaño), **sin backtest**. Es la primera fase del modelo
multi-factor de [`docs/modelo-multifactor-etfs.md`](docs/modelo-multifactor-etfs.md): solo TER y AUM.

```
top3 etfq                # corre todo y guarda el resultado (la web lo muestra en «ETFs por gestora»)
top3 etfq --top-n 30     # más filas en la consola (el JSON del frontend trae todos)
top3 pipeline --skip-etfq
```

**De dónde sale cada dato.** Todo viene de Yahoo Finance: las páginas de las gestoras bloquean las descargas automáticas, y el
screener de Yahoo filtra por gestora (`fundFamily`) y trae en la misma consulta la lista de ETFs de EE. UU., el TER
(`netExpenseRatio`, en % anual) y el AUM (`netAssets`, en US$). La categoría (Morningstar, p. ej. «Large Blend») sale de `Info` y
se guarda en SQLite (`[etfq] category_ttl_days`). Los nombres de las gestoras son los de Yahoo (`[etfq] families`).

**Score.**
1. **Momentum:** el mismo score de retorno ajustado por riesgo de la sección de Vanguard (`[etf] horizons_months`,
   `risk_free_ticker` y `min_vol`), calculado para todos los ETFs de las cinco gestoras.
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

## Configuración (`backend-go/config.toml`)

Los valores de la columna «Por defecto» son los que se usan si la clave no está en el archivo; el `config.toml`
incluido puede traer otros (por ejemplo más períodos y plazos en `[simulation]`).

| Sección | Clave | Por defecto | Descripción |
|---|---|---|---|
| `[data]` | `history_years` | `12` | Años de histórico a descargar (composición y precios); debe superar el mayor período + 1 año de ranking |
| | `db_path` | `"data/top3.db"` | Base SQLite |
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
| | `periods_years` | `[3, 5, 10]` | Períodos (años) que se simulan; el frontend deja elegir entre ellos. `--years N` corre uno solo |
| | `months` | `4` | Plazo del backtest normal: ranking de N meses y rebalanceo cada N meses |
| | `initial_capital` | `1000` | Capital inicial (US$) |
| | `top_n` | `4` | Cantidad de acciones por rebalanceo |
| | `commission` | `0.0005` | Comisión por lado (0,05%): se cobra al armar la cartera y por las posiciones que cambian en cada rebalanceo |
| | `alternates` | `4` | Suplentes del Top actual: puestos posteriores al `top_n` que se muestran como reemplazo posible (`0` = ninguno) |
| | `trading_days_per_month` | `21` | Ruedas por mes para convertir meses en ventana de ranking |
| `[tickers.aliases]` | `VIEJO = "NUEVO"` | 6 renombres | Unifica símbolos renombrados |
| `[universe]` | `min_cap_billions` | `10` | Umbral de `--large-cap` (miles de millones de US$) |
| | `sweep_min_cap_billions` | `[0, 10, 200]` | Universos que corre `--sweep` por defecto (0 = índice completo) |
| `[output]` | `dir`, `keep_days`, `*_subdir`, `*_prefix` | | Reservadas; no se generan reportes HTML. `dir` y `keep_days` solo sirven para limpiar archivos viejos |
| | `frontend_data` | `"../frontend/public/data/report.json"` | JSON estático para el frontend |
| | `history_file` | `"history/history.json"` | Historial del Top actual, suplentes y ETFs, versionado en git (ver [Historial en JSON](#historial-en-json)); `""` = desactivado |
| | `log_dir` | `"log"` | Carpeta de los logs |
| | `backtest_log`, `actual_log`, `etf_log`, `etfq_log`, `quant_log` | `top3_*.log` | Nombres de los archivos de log |
| `[etf]` | `update_list` | `true` | Al correr el ranking de ETFs, agrega a la lista los ETFs nuevos que Vanguard publica (nunca borra ni edita filas) |
| | `list_url` | página de productos de Vanguard | Servicio del que se baja la lista de ETFs de Vanguard |
| | `csv` | `"history/vanguard_etfs.csv"` | Lista de ETFs de Vanguard (columnas `ticker`, `name`, `category`) |
| | `horizons_months` | `[3, 6, 12]` | Plazos del retorno ajustado por riesgo; el score es el promedio de sus cocientes |
| | `top_n` | `15` | Cuántos ETFs se muestran en la consola (el JSON del frontend trae todos) |
| | `min_vol` | `0.01` | Volatilidad anual mínima para entrar al ranking (1%): los ETFs por debajo son de tipo efectivo (letras del Tesoro, bonos ultracortos) y quedan fuera; `0` = rankear todos |
| | `risk_free_ticker` | `"VUSB"` | ETF cuyo retorno se resta al de cada ETF (referencia libre de riesgo); `""` = no restar |
| `[etfq]` | `families` | iShares, Vanguard, State Street Investment Management (SPDR), Invesco y Schwab ETFs | Gestoras a incluir, como `[[etfq.families]]` con `name` (el `fundFamily` de Yahoo) y `label` (el nombre que se muestra) |
| | `top_n` | `20` | Cuántos ETFs se muestran en la consola (el JSON del frontend trae todos) |
| | `max_ter` | `0.50` | TER máximo en % anual para ser elegible; `0` = sin tope |
| | `min_aum_millions` | `100` | AUM mínimo en millones de US$ para ser elegible; `0` = sin mínimo |
| | `quality_weight` | `0.25` | Peso de la calidad en el score final (0-1); el resto es momentum |
| | `quality_weights` | `[4, 1]` | Peso del TER y del AUM dentro de la calidad |
| | `category_ttl_days` | `30` | Días que se reutiliza la categoría guardada de cada ETF |
| `[quant]` | `weights` | `[1, 1, 1]` | Peso de cada score en el final: momentum, salud financiera y técnico |
| | `health_weights` | `[1, 1, 1, 2]` | Peso de cada grupo en el score de salud: rentabilidad, solvencia, liquidez y caja (un grupo sin datos no cuenta) |
| | `top_n` | `10` | Acciones del ranking final sobre las que corre Markowitz |
| | `vol_target`, `w_max`, `sector_max` | `0.20`, `0.25`, `0.40` | Volatilidad anual máxima de la cartera de máximo retorno, y peso máximo por acción y por sector |
| | `mu_shrink`, `risk_free`, `equity_premium` | `0.25`, `0.04`, `0.05` | Retorno esperado = `mu_shrink` x retorno de los últimos 12 meses + el resto x (`risk_free` + `equity_premium`); `risk_free` también para el Sharpe |
| | `fundamentals_ttl_days` | `7` | Días que se reutilizan los datos financieros descargados |
| `[simulation]` | `months` | `[3, 4, 6, 8, 12]` | Plazos que comparan `--sweep` y `--offsets` |

## Salidas y depuración

### Reportes

El backend en Go no genera reportes HTML: los resultados se ven en la web y por consola.

### Consola

Las tablas de selecciones tienen una fila por rebalanceo: `Fecha`, los tickers `#1`..`#N` con su retorno,
`Cap. aprox. (US$ B)` (informativa), `Capital (US$)` (valor al entrar a esa fila) y `Gan/Pérd` (acumulado
vs. el capital inicial, en verde si es positivo). Se puede desactivar el color con la variable `NO_COLOR=1`.

Los títulos y marcos de los resúmenes (cada paso del pipeline, `RESUMEN` y `RESUMEN DE AVISOS Y ERRORES`) salen en
**verde**. Los avisos del log salen en **amarillo** y los errores en **rojo** (incluidos los mensajes que cortan un
comando). Solo si la salida es una terminal: al redirigir a archivo o pipe, o con
`NO_COLOR=1`, va sin color. El archivo de log nunca lleva códigos de color.

Al terminar cada comando (y `top3 pipeline`, con los de todos sus pasos juntos) se imprime un **resumen de avisos y
errores** agrupado por código, con cuántas veces ocurrió, un ejemplo y qué hacer según la tabla de
[códigos de error](#códigos-de-error-y-aviso); los errores van primero. Si no hubo ninguno, no se imprime nada.

### Historial en JSON

La base SQLite es **solo caché**: todo lo que contiene se puede volver a bajar o calcular, salvo el historial de
`actual_picks`, `actual_alternates` y `etf_scores` (de ahí salen la comparación con la corrida anterior, el calendario de
rebalanceo y el seguimiento). Ese historial se guarda además en `backend-go/history/history.json`, que **sí se versiona en git**:

- Al terminar cualquier comando que exporta (`backtest`, `actual`, `etf`, `quant`, `export`, `pipeline`) se reescribe el
  archivo con el contenido actual de esas tres tablas (una fila por línea, para que `git diff` se lea bien). Si no hubo
  cambios, no se toca.
- Al arrancar cualquier comando, si la base no tiene historial (base nueva, caché de CI perdido) se restaura desde el archivo.
  Si la base ya tiene historial, no se toca.

Así se puede borrar `backend-go/data/` sin perder nada importante: solo hay que volver a descargar los precios. Para usar el
historial en otro entorno basta con clonar el repositorio.

### SQLite (`backend-go/data/top3.db`)

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
| `etf_scores` | Ranking de ETFs de Vanguard por corrida (fecha de datos, posición, score, volatilidad, caída máxima, distancia a la SMA200, retornos por plazo) |
| `etfq_category` | Categoría de cada ETF de las gestoras y el día en que se bajó (caché de `[etfq] category_ttl_days`) |
| `etfq_runs` | Último resultado de los ETFs por gestora (JSON: ranking con momentum, TER, AUM y calidad) |
| `fundamentals` | Últimos datos financieros descargados de cada empresa (JSON) y su fecha |
| `ohlcv` | Apertura, máximo, mínimo, cierre ajustado y volumen diarios del último año y medio de cada acción del S&P 500 (para los indicadores técnicos) |
| `quant_runs` | Último resultado del modelo por score (JSON: ranking y Markowitz) |
| `ipsa_runs` | Último resultado del modelo por score del IPSA (JSON, igual que `quant_runs` más moneda y origen de la lista) |
| `actual_picks`, `actual_alternates` | Resultados de `top3 actual` (fecha de datos, plazo, ticker, retorno, precio, capitalización) y sus suplentes |

### Limpieza de la base

Cada backtest guarda cientos de corridas (parámetros, selecciones y curva de capital) y el informe solo usa la última de
cada combinación del día más reciente. Para que la base no crezca sin límite:

- Al terminar `top3 backtest` (y por lo tanto `top3 pipeline`) se borran las corridas de más de
  `[data] runs_keep_days` días (contados desde el último día con corridas) y las que otra corrida del mismo día
  reemplaza: las que tienen el mismo universo, período, plazo, desfase y modelo (N-0 o N-1), que es lo que
  identifica una corrida en el informe. Es rápido, así que se hace en cada ejecución. Compactar el archivo
  (`VACUUM`) es lo lento: se hace como mucho una vez por día.
- Para forzarlo a mano: `top3 prune` (o `--keep-days N`). Además de las corridas, esa limpieza completa
  (que `top3 pipeline` corre como último paso; se omite con `--skip-prune`) borra:
  - los precios anteriores a la ventana de `[data] history_years`;
  - los datos de caché ya vencidos (acciones en circulación, datos financieros y tickers sin datos), y luego compacta
    la base.
- No se toca `actual_picks` ni `etf_scores` (historial del Top actual y de los ETFs: de ahí salen el calendario de
  rebalanceo, la comparación con la corrida anterior y el seguimiento; pesan muy poco) ni los precios dentro de la
  ventana (el backtest los necesita). Los logs de `log/` se sobrescriben en cada corrida, no se acumulan.

### Depuración

Cada ejecución escribe `log/top3_backtest.log` (o `log/top3_actual.log`) con nivel DEBUG: respuestas HTTP, forma de las
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
| `ETF` | Ranking de ETFs de Vanguard |
| `GES` | ETFs por gestora |
| `QNT` | Modelo por score |
| `IPS` | IPSA (Bolsa de Santiago) |

**Aviso** = se continúa con lo que haya. **Error** = la operación se corta o falla.

Para buscar en el log: `grep TOP-BKT backend-go/log/top3_backtest.log`.

| ID | Nivel | Qué significa | Qué hacer |
|---|---|---|---|
| `TOP-CFG-001` | Aviso | No existe el archivo de configuración | Se usan los valores por defecto. Crea config.toml o pasa --config con la ruta correcta. |
| `TOP-CFG-002` | Error | start_date no es anterior a end_date | Corrige [backtest] start_date / end_date o backtest_years en config.toml. |
| `TOP-CFG-003` | Aviso | El período del backtest empieza antes del histórico descargado | Sube [data] history_years (debe superar el mayor período + 1 año). |
| `TOP-CFG-004` | Error | Sección o clave desconocida en la configuración | Casi siempre es un typo en config.toml: compáralo con backend-go/config.toml. |
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
| `TOP-ETF-001` | Error | No hay lista de ETFs utilizable | Revisa [etf] csv en config.toml: el archivo debe existir y tener las columnas ticker, name, category. |
| `TOP-ETF-002` | Aviso | ETFs de la lista sin precios en Yahoo | Pueden estar dados de baja o con el ticker mal escrito; quítalos o corrígelos en el CSV. |
| `TOP-ETF-003` | Aviso | ETFs fuera del ranking por huecos de precios o poca historia | Los ETFs nuevos entran cuando acumulan historia para el plazo más largo. |
| `TOP-ETF-004` | Error | Ningún ETF se pudo rankear | Descarga más historia o reduce [etf] horizons_months. |
| `TOP-ETF-005` | Aviso | La referencia libre de riesgo no está disponible | El score se calcula sin restarla (los ETFs casi sin riesgo quedan arriba). Revisa [etf] risk_free_ticker: debe estar en la lista y tener historia completa. |
| `TOP-ETF-006` | Aviso | No se pudo actualizar la lista de ETFs de Vanguard | Se usa la lista guardada en el CSV. Revisa la conexión; si Vanguard cambió el formato de su respuesta hay que ajustar `internal/etf/vanguard.go`. Con `--no-update` o `[etf] update_list = false` no se intenta. |
| `TOP-GES-001` | Aviso | Una gestora no devolvió ETFs | Yahoo falló o no conoce ese nombre. Revisa `[etfq] families`: `name` debe ser el `fundFamily` de Yahoo (p. ej. 'iShares'). Las demás gestoras siguen. |
| `TOP-GES-002` | Aviso | ETFs sin categoría | Yahoo no informó la categoría: esos ETFs se comparan entre sí como 'Sin categoría'. Se reintenta en la próxima corrida. |
| `TOP-GES-003` | Error | Ningún ETF de las gestoras se pudo rankear | Revisa la conexión y `[etfq] families`; los ETFs nuevos entran cuando acumulan historia para el plazo más largo. |
| `TOP-QNT-001` | Error | No hay acciones suficientes para el ranking del modelo por score | Descarga más historia (`--no-cache`) o baja `[quant] top_n`; revisa también `TOP-DAT-005`. |
| `TOP-IPS-001` | Error | No se pudo obtener la lista de acciones del IPSA | Revisa `[ipsa] wikipedia_url` y la conexión. Si hay una lista guardada (`universe_file`) se usa aunque esté vencida (en ese caso es un aviso). |
| `TOP-IPS-002` | Error | No hay acciones suficientes para el ranking del IPSA | Descarga de nuevo (`--no-cache`) o baja `[ipsa] top_n`; Yahoo tiene pocos datos financieros de acciones chilenas. |

## Estructura y desarrollo

```
start.py              # valida el entorno y levanta el frontend (Windows, macOS y Linux)
start_with_pipeline.py  # igual, pero corre antes el pipeline completo del backend
backend-go/           # Go: backtest, Top actual, ranking de ETFs, modelo por score, IPSA y base SQLite
  config.toml         # configuración
  history/            # datos que se versionan en git: historial, lista de ETFs de Vanguard y lista del IPSA
  cmd/top3/           # línea de comandos: backtest, actual, etf, quant, export, prune y pipeline
  internal/           # la lógica, un paquete por responsabilidad
frontend/             # Aplicación web en Svelte 5 + Vite
```

| Comando | Para qué sirve |
|---|---|
| `top3 backtest` | Backtest Point-in-Time, comparación de plazos y sensibilidad al calendario de arranque |
| `top3 actual` | Top N de hoy: las acciones del S&P 500 con mejor retorno en el plazo configurado |
| `top3 etf` | Ranking de momentum ajustado por riesgo de los ETFs de Vanguard (sin backtest) |
| `top3 etfq` | ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum y calidad (TER y AUM) dentro de su categoría (sin backtest) |
| `top3 quant` | Modelo por score: scores de momentum, de salud financiera y técnico del S&P 500 y Markowitz sobre las mejores (sin backtest) |
| `top3 ipsa` | Modelo por score de las 30 acciones del IPSA (Bolsa de Santiago, en CLP) y Markowitz sobre las mejores (sin backtest) |
| `top3 pipeline` | Pipeline completo: backtest (todos los períodos y universos) + Top actual de cada universo + ETFs + modelo por score + IPSA + JSON del frontend + limpieza |
| `top3 prune` | Limpieza de lo viejo: corridas del backtest y caché vencida (el pipeline la corre al final) |
| `top3 export` | Escribe el JSON en `frontend/public/data/report.json` (lo hacen solos los comandos al terminar) |
| `backend-go/data/`, `backend-go/log/` | Base SQLite y logs; se crean solos |

El detalle de los paquetes de `backend-go/internal/` (config, db, repo, yahoo, market, engine, backtest, actual, etf, etfq, quant,
report, cleanup, pipeline...) está en [`backend-go/README.md`](backend-go/README.md).

**Frontend (`frontend/src/`)**

| Archivo | Responsabilidad |
|---|---|
| `App.svelte` | Carga del JSON estático, pestañas, filtros de universo y período, tema |
| `lib/ActualView.svelte` | Top actual, suplentes y seguimiento de selecciones anteriores |
| `lib/BacktestView.svelte` | Tarjetas, curva de capital, historial de selecciones y calendarios |
| `lib/SweepView.svelte` | Comparación de plazos y sensibilidad al arranque |
| `lib/CompareView.svelte` | Modelo de siempre (N-0) frente al skip-month (N-1), plazo por plazo y por mitades del período |
| `lib/EtfView.svelte` | Ranking de ETFs de Vanguard |
| `lib/EtfqView.svelte` | ETFs por gestora: ranking con momentum, calidad (TER y AUM), filtros por gestora y categoría |
| `lib/QuantView.svelte` | Modelo por score: top 10 con Markowitz y ranking completo con filtros |
| `lib/IpsaView.svelte` | Chile (IPSA): reutiliza `QuantView` con el ranking del IPSA, aclara la moneda (CLP) y el origen de la lista |
| `lib/AboutView.svelte` | Pestaña About: qué se muestra en la aplicación, fuentes de datos y límites |
| `lib/Chart.svelte`, `Range.svelte`, `TickerLink.svelte`, `Glossary.svelte` | Componentes compartidos |
| `lib/Bunny.svelte` | Detalle decorativo: un conejo que pasea por el pie de la pantalla, se aleja del cursor y saluda al hacer clic (se desactiva con «reducir movimiento» del sistema) |
| `lib/format.js` | Formato de porcentajes, dinero y signos |

**Tests.** Backend: un archivo de tests por paquete; no usan red (Yahoo se simula con un servidor local) y los datos son
sintéticos. Los indicadores técnicos, Ledoit-Wolf y Markowitz se comparan contra valores de referencia calculados con herramientas externas.

```
cd backend-go && go test ./...
cd frontend && npm test
```

**Mantenimiento de los códigos de error.** El catálogo está en `backend-go/internal/errs`. Los IDs no se reutilizan ni se
renumeran; todo aviso o error usa `CODIGO.Msg("texto")` o `CODIGO.Errorf(...)`, y cada código nuevo se documenta en la tabla de arriba.


## Publicación (GitHub Actions y Pages)

`.github/workflows/deploy.yml` corre en días hábiles después del cierre de EE. UU., a mano (`workflow_dispatch`) y en cada
push a `main`:

1. Restaura la base SQLite desde el caché de Actions (`backend-go/data`), corre los tests y el pipeline.
2. Hace commit de `backend-go/history/history.json`, `backend-go/history/ipsa_universe.json` y `backend-go/history/vanguard_etfs.csv` si cambiaron (el commit del bot no
   dispara otra ejecución).
3. Prueba y compila el frontend y lo publica en GitHub Pages.

Requiere activar Settings > Pages > Source = **GitHub Actions**. Si el caché se pierde (GitHub lo borra tras 7 días sin
uso), la siguiente ejecución vuelve a bajar los precios (varios minutos) y restaura el historial desde el JSON. Si el
pipeline falla (por ejemplo, rate limit de Yahoo), no se publica nada y la web sigue con la versión anterior.
