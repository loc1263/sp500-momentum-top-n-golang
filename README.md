# sp500-momentum-top-n-golang

Estrategia de momentum sobre el S&P 500: cada N meses se compran, con ponderación igual, las N acciones que más
subieron en los últimos N meses. El proyecto la evalúa con datos históricos (backtest), mide cuánto del resultado
depende del mes de arranque y muestra todo en una aplicación web.

**Ver el sitio: https://loc1263.github.io/sp500-momentum-top-n-golang/** (se actualiza solo cada día hábil).

> **Aviso:** es una herramienta de análisis, no una recomendación de inversión. El backtest cobra solo una comisión
> de 0,05% por lado, sin impuestos, y tiene sesgos de datos (ver [Limitaciones](TECHNICAL.md#limitaciones)).

## Qué incluye

- **Backtest** Point-in-Time, con comparación de plazos y sensibilidad al mes de arranque.
- **N-0 vs. N-1**: el modelo de siempre frente a una variante que ignora el último mes al rankear.
- **Top actual**: las N acciones líderes hoy, con suplentes, cambios y próximo rebalanceo.
- **ETFs de Vanguard**: ranking de momentum ajustado por riesgo.
- **Modelo por score**: score de momentum, de salud financiera y técnico para cada acción, y Markowitz sobre las mejores.

## Requisitos

- [Go](https://go.dev/dl/) 1.26 o superior
- [Node.js](https://nodejs.org/) 20.19 o superior (incluye `npm`)
- Conexión a internet

## Cómo ejecutarlo

```
git clone https://github.com/loc1263/sp500-momentum-top-n-golang.git
cd sp500-momentum-top-n-golang
```

**1. Instalar el frontend**

```
cd frontend
npm install
cd ..
```

**2. Generar los datos** (la primera vez tarda varios minutos; luego solo actualiza lo que falta)

```
cd backend-go
go run ./cmd/top3 pipeline
cd ..
```

**3. Abrir la web**

| Sistema | Comando |
|---|---|
| Windows (PowerShell) | `.\start.ps1` |
| macOS / Linux | `./start.sh` |

Luego abra **http://localhost:5173**. Para detenerlo, `Ctrl+C`. Para actualizar los resultados, repita el paso 2 y
recargue la página.

## Problemas frecuentes

| Síntoma | Solución |
|---|---|
| PowerShell indica que la ejecución de scripts está deshabilitada | `powershell -ExecutionPolicy Bypass -File .\start.ps1`, o `cd frontend && npm run dev` |
| La web muestra «No se pudo cargar data/report.json», «Sin Top actual» o «Sin backtest» | Falta el paso 2: ejecute `go run ./cmd/top3 pipeline` en `backend-go/` y recargue |
| Puerto en uso | `.\start.ps1 -FrontendPort 3000` (Windows) o `FRONTEND_PORT=3000 ./start.sh` |
| Errores `TOP-NET-...` durante la descarga | Suelen ser límites de Yahoo Finance o de red; reintente más tarde |
| No aparece el conejito 🐇 de la web | Se oculta si el sistema tiene la reducción de movimiento activada. En Windows 11: Configuración → Accesibilidad → Efectos visuales → **Efectos de animación** en «Activado», y recargue la página |

## Más información

- [`TECHNICAL.md`](TECHNICAL.md): cómo funciona, comandos y opciones, modelo por score, configuración, base de datos,
  códigos de error, estructura y tests.
- [`backend-go/README.md`](backend-go/README.md): paquetes del backend y rendimiento.

Los datos vienen de Yahoo Finance (vía [`gohoo`](https://github.com/loc1263/gohoo)), de
[`hanshof/sp500_constituents`](https://github.com/hanshof/sp500_constituents) y de Wikipedia. Proyecto de uso personal
y educativo.

## Licencia

[MIT](LICENSE)
