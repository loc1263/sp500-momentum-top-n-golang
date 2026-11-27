# Frontend (Astro)

Sitio estático del proyecto. Los datos de cada página los genera el backend (`topwx site`, o el final de `topwx pipeline`) en
`src/data/*.json` y `public/` (`empresas/<TICKER>.json`, `lab/<vista>/<clave>.json`, `evaluador.xlsx`); no se versionan. Astro los convierte en un `.html` por página.

```
npm install
npm run dev        # http://localhost:4321
npm run build      # frontend/dist
npm run preview
npm test           # vitest + jsdom (src/**/*.test.js)
```

- `src/pages`: una página por vista; `src/layouts/Layout.astro`: cáscara común; `src/components`: tablas y piezas del Lab.
- `src/scripts`: JS de las tablas, el Visor, los gráficos, el tema y los componentes de Alpine (`src/alpine.js`); los tests van junto (`*.test.js`).
- `src/styles/site.css`: estilos y temas; las reglas para móvil (≤ 640 px) están en los bloques `@media`. El sitio debe verse bien en PC y smartphones.
- `SITE_BASE=/<repo> npm run build` para publicar bajo una subruta (GitHub Pages).
