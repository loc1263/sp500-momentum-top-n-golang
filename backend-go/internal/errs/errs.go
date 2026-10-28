// Package errs es el catálogo de códigos de error y aviso (al estilo ORA-00942): cada mensaje del log y de los errores
// lleva su ID.
//
// Formato: TOP-<ÁREA>-<NNN>. Áreas: CFG configuración, NET descargas, DAT datos, BKT backtest, ACT Top actual,
// ETF ETFs de Vanguard, GES ETFs por gestora, QNT modelo por score, IPS IPSA. Los IDs no se reutilizan ni se renumeran: documentarlos en la tabla de errores del README.
package errs

import "fmt"

// Code es un código del catálogo.
type Code struct {
	ID     string
	Level  string // "warning": se sigue con lo que haya; "error": la operación se corta o falla
	Title  string
	Action string // qué hacer
}

// Msg es `text` precedido por el ID: "TOP-BKT-003: Pocos tickers...". Sirve como formato de log.
func (c Code) Msg(text string) string { return c.ID + ": " + text }

// Errorf arma un error con el ID delante.
func (c Code) Errorf(format string, args ...any) error {
	return fmt.Errorf("%s: %s", c.ID, fmt.Sprintf(format, args...))
}

// Catalog tiene todos los códigos por ID.
var Catalog = map[string]Code{}

func code(id, level, title, action string) Code {
	if _, dup := Catalog[id]; dup {
		panic("código de error repetido: " + id)
	}
	c := Code{id, level, title, action}
	Catalog[id] = c
	return c
}

// Configuración
var (
	CfgMissing = code("TOP-CFG-001", "warning", "No existe el archivo de configuración",
		"Se usan los valores por defecto. Crea config.toml o pasa --config con la ruta correcta.")
	CfgBadDates = code("TOP-CFG-002", "error", "start_date no es anterior a end_date",
		"Corrige [backtest] start_date / end_date o backtest_years en config.toml.")
	CfgShortHistory = code("TOP-CFG-003", "warning", "El período del backtest empieza antes del histórico descargado",
		"Sube [data] history_years (debe superar el mayor período + 1 año).")
	CfgUnknownKey = code("TOP-CFG-004", "error", "Sección o clave desconocida en la configuración",
		"Casi siempre es un typo en config.toml: compáralo con backend-go/config.toml.")
)

// Descargas
var (
	NetComposition = code("TOP-NET-001", "error", "Falló la descarga de la composición del índice",
		"Revisa la conexión; se usa lo guardado en SQLite. Reintenta o usa --no-cache.")
	NetFallback = code("TOP-NET-002", "warning", "Se usa la composición guardada en SQLite",
		"Los datos pueden estar desactualizados; vuelve a correr cuando haya conexión.")
	NetCurrentList = code("TOP-NET-003", "error", "Falló la descarga de la lista actual del S&P 500",
		"Se usa la guardada en SQLite (puede ser vieja). Revisa la conexión.")
	NetWikiFormat = code("TOP-NET-004", "error", "No se encontró la tabla de cambios en Wikipedia",
		"Es probable que Wikipedia cambiara el formato de la página; hay que ajustar el lector.")
	NetRateLimit = code("TOP-NET-005", "error", "Yahoo limitó las solicitudes (rate limit) y quedaron tickers sin datos",
		"Ya se reintentó con espera. Vuelve a correr más tarde; los tickers afectados no entran en el ranking.")
	NetRateRetry = code("TOP-NET-006", "warning", "Yahoo limitó las solicitudes; se reintenta tras una espera",
		"No requiere acción: si el reintento funciona no se pierde nada.")
	NetYahoo = code("TOP-NET-007", "warning", "Yahoo informó un error al descargar datos",
		"Revisa el mensaje original; los tickers afectados pueden quedar sin datos.")
)

// Datos
var (
	DatBadDates = code("TOP-DAT-001", "warning", "Filas con fecha no interpretable en la tabla de cambios",
		"Se ignoran. Si son muchas, el formato de la fuente cambió.")
	DatNoHistory = code("TOP-DAT-002", "warning", "Sin composición histórica para esa fecha",
		"Esa fecha queda sin constituyentes; revisa history_years y el CSV histórico.")
	DatNoShares = code("TOP-DAT-003", "warning", "Sin dato de acciones en circulación para algunos tickers",
		"Su capitalización queda en n/d y no entran en un universo filtrado por capitalización.")
	DatNoCurrent = code("TOP-DAT-004", "error", "No hay lista actual del S&P 500",
		"No se descargó ni existe en SQLite. Corre con conexión a internet.")
	DatNoOHLCV = code("TOP-DAT-006", "warning", "Sin precios de apertura, máximo, mínimo o volumen para algunos tickers",
		"Quedan fuera del score técnico y del ranking final; suele ser un límite de Yahoo o un ticker nuevo. "+
			"Vuelve a correr más tarde.")
	DatNoFundamentals = code("TOP-DAT-005", "warning", "Sin datos financieros para algunos tickers",
		"Quedan fuera del score de salud y del ranking final; suele ser un límite de Yahoo o un ticker "+
			"nuevo. Vuelve a correr más tarde.")
	DatDBNewer = code("TOP-DAT-007", "error", "La base SQLite es de una versión más nueva del programa",
		"Actualiza el código (git pull) o usa otra base en [data] db_path; no se modifica la base.")
)

// Backtest
var (
	BktEmpty = code("TOP-BKT-001", "error", "El backtest no generó ningún rebalanceo",
		"Revisa las fechas, el histórico descargado y que el universo no sea demasiado chico.")
	BktNoConstituents = code("TOP-BKT-002", "warning", "Sin constituyentes en la fecha de rebalanceo",
		"Se omite ese rebalanceo.")
	BktFewTickers = code("TOP-BKT-003", "warning", "Pocos tickers disponibles en la fecha de rebalanceo",
		"Se omiten esos rebalanceos (menos de 10 candidatos); se avisa una vez por universo y período con el "+
			"rango de fechas. Frecuente en mega-cap con períodos largos.")
	BktShortWindow = code("TOP-BKT-004", "warning", "Ventana de precios insuficiente para rankear",
		"Se omite ese rebalanceo.")
	BktNoPrices = code("TOP-BKT-005", "warning", "Sin precios para mantener la selección durante el período",
		"El retorno de ese período no se computa.")
	BktTermFailed = code("TOP-BKT-006", "warning", "Un plazo/calendario de arranque no produjo resultado",
		"Se omite ese calendario; el resto continúa.")
)

// Top actual
var (
	ActShortWindow = code("TOP-ACT-001", "error", "No hay ruedas de precios suficientes",
		"Descarga más historia (--no-cache) o reduce el plazo en meses.")
	ActGaps = code("TOP-ACT-002", "warning", "Tickers con huecos en la ventana quedan fuera del ranking",
		"Misma regla que el backtest. Revisa si Yahoo falló para esos tickers y reintenta.")
)

// ETFs de Vanguard
var (
	EtfNoList = code("TOP-ETF-001", "error", "No hay lista de ETFs utilizable",
		"Revisa [etf] csv en config.toml: el archivo debe existir y tener las columnas ticker, name, category.")
	EtfNoPrices = code("TOP-ETF-002", "warning", "ETFs de la lista sin precios en Yahoo",
		"Pueden estar dados de baja o con el ticker mal escrito; quítalos o corrígelos en el CSV.")
	EtfExcluded = code("TOP-ETF-003", "warning", "ETFs fuera del ranking por huecos de precios o poca historia",
		"Los ETFs nuevos entran cuando acumulan historia para el plazo más largo.")
	EtfEmpty = code("TOP-ETF-004", "error", "Ningún ETF se pudo rankear",
		"Descarga más historia o reduce [etf] horizons_months.")
	EtfNoRiskFree = code("TOP-ETF-005", "warning", "La referencia libre de riesgo no está disponible",
		"El score se calcula sin restarla (los ETFs casi sin riesgo quedan arriba). "+
			"Revisa [etf] risk_free_ticker: debe estar en la lista y tener historia completa.")
	EtfListUpdate = code("TOP-ETF-006", "warning", "No se pudo actualizar la lista de ETFs de Vanguard",
		"Se usa la lista guardada en el CSV. Revisa la conexión; si Vanguard cambió el formato de su "+
			"respuesta hay que ajustar internal/etf/vanguard.go. Con --no-update o [etf] update_list = false no se intenta.")
)

// ETFs por gestora
var (
	GesNoFamily = code("TOP-GES-001", "warning", "Una gestora no devolvió ETFs",
		"Yahoo falló o no conoce ese nombre. Revisa [etfq] families: name debe ser el fundFamily de Yahoo (p. ej. 'iShares'). Las demás gestoras siguen.")
	GesNoCategory = code("TOP-GES-002", "warning", "ETFs sin categoría",
		"Yahoo no informó la categoría: esos ETFs se comparan entre sí como 'Sin categoría'. Se reintenta en la próxima corrida.")
	GesEmpty = code("TOP-GES-003", "error", "Ningún ETF de las gestoras se pudo rankear",
		"Revisa la conexión y [etfq] families; los ETFs nuevos entran cuando acumulan historia para el plazo más largo.")
)

// Modelo cuantitativo
var (
	QntNoSignal = code("TOP-QNT-001", "error", "No hay acciones suficientes para el ranking del modelo por score",
		"Descarga más historia (--no-cache) o baja [quant] top_n; revisa también TOP-DAT-005.")
)

// IPSA (Bolsa de Santiago)
var (
	IpsNoUniverse = code("TOP-IPS-001", "error", "No se pudo obtener la lista de acciones del IPSA",
		"Revisa [ipsa] wikipedia_url y la conexión; si hay una lista guardada (universe_file) se usa aunque esté vencida.")
	IpsNoSignal = code("TOP-IPS-002", "error", "No hay acciones suficientes para el ranking del IPSA",
		"Descarga de nuevo (--no-cache) o baja [ipsa] top_n; Yahoo tiene pocos datos financieros de acciones chilenas.")
)
