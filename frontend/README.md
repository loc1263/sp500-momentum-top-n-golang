# Frontend (Astro)

Sitio estático del proyecto. Los datos de cada página los genera el backend (`topwx site`, o el final de `topwx pipeline`) en
`src/data/*.json` y `public/` (`empresas/<TICKER>.json`, `lab/<vista>/<clave>.json`, `evaluador.xlsx`); no se versionan. Astro los convierte en un `.html` por página.

```
npm install
npm run dev        # http://localhost:4321
npm run build      # frontend/dist
npm run preview
```

- `src/pages`: una página por vista; `src/layouts/Layout.astro`: cáscara común; `src/components`: tablas y piezas del Lab.
- `src/scripts`: JS de las tablas, el Visor, el tema y los componentes de Alpine (`src/alpine.js`).
- `SITE_BASE=/<repo> npm run build` para publicar bajo una subruta (GitHub Pages).
