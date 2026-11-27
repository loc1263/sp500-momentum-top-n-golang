// Interactividad común de las tablas armadas en las páginas de Astro: filtros (select, casilla, búsqueda de texto), orden por columna, paginación,
// clic en la fila y selector que muestra un bloque a la vez. Solo mueve y oculta las filas que ya están en el HTML.
//
// Contrato (todo en atributos data-*):
//   [data-table]            contenedor; data-page-size = filas por página
//   tbody tr[data-row]      fila; data-q = texto buscable, data-f-<clave> = valor a filtrar, data-href = destino del clic
//   td[data-v]              valor crudo para ordenar (si falta, la fila va al final)
//   th[data-type]           columna ordenable (num | text); data-dir = sentido del primer clic; data-initial = orden de origen
//   [data-filter=clave]     select; [data-filter-bool=clave] casilla (exige data-f-<clave>="1"); [data-search] texto libre
//   [data-count]            «N de M»; nav.pager = paginador
//   select[data-view]      muestra solo las columnas (th / td [data-views]) que incluyen la vista elegida
//   select[data-switch=k]   muestra el [data-case-of=k][data-case=valor] elegido y oculta los demás
//
// Fecha: 2026-11-19

import './pager.js';

(() => {
  const collator = new Intl.Collator('es');

  function initTable(root) {
    const size = Number(root.dataset.pageSize || 12);
    const body = root.querySelector('tbody');
    const rows = [...body.querySelectorAll('tr[data-row]')];
    const empty = body.querySelector('tr.empty');
    const pager = root.querySelector('.pager');
    const count = root.querySelector('[data-count]');
    const heads = [...root.querySelectorAll('thead th[data-type]')];
    const initial = heads.find((th) => th.dataset.initial);

    const state = { page: 0, filters: {}, bools: {}, q: '', sort: initial ? initial.cellIndex : null, desc: initial?.dataset.initial === 'desc', original: true };

    root.querySelectorAll('[data-filter]').forEach((el) => {
      state.filters[el.dataset.filter] = el.value;
      el.addEventListener('change', () => { state.filters[el.dataset.filter] = el.value; state.page = 0; render(); });
    });
    root.querySelectorAll('[data-filter-bool]').forEach((el) => {
      state.bools[el.dataset.filterBool] = el.checked;
      el.addEventListener('change', () => { state.bools[el.dataset.filterBool] = el.checked; state.page = 0; render(); });
    });
    // Vistas de columnas: las celdas con data-views solo se ven si incluyen la vista elegida.
    const viewSelect = root.querySelector('select[data-view]');
    const viewCells = [...root.querySelectorAll('[data-views]')];
    function applyView() {
      if (!viewSelect) return;
      viewCells.forEach((el) => (el.hidden = !el.dataset.views.split(' ').includes(viewSelect.value)));
      const sorted = heads.find((th) => th.cellIndex === state.sort);
      if (sorted?.hidden) {
        state.sort = initial ? initial.cellIndex : null;
        state.desc = initial?.dataset.initial === 'desc';
        state.original = true;
      }
    }
    if (viewSelect) viewSelect.addEventListener('change', () => { applyView(); state.page = 0; render(); });

    root.querySelectorAll('[data-search]').forEach((el) => {
      el.addEventListener('input', () => { state.q = el.value.trim().toLowerCase(); state.page = 0; render(); });
    });

    heads.forEach((th) => th.querySelector('.sort').addEventListener('click', () => {
      const i = th.cellIndex;
      if (state.sort === i) state.desc = !state.desc;
      else { state.sort = i; state.desc = th.dataset.dir === 'desc'; }
      state.original = initial && i === initial.cellIndex && (state.desc ? 'desc' : 'asc') === initial.dataset.initial;
      state.page = 0;
      render();
    }));

    if (pager) pager.addEventListener('click', (e) => {
      const go = e.target.closest('[data-go]')?.dataset.go;
      if (!go) return;
      const last = Math.max(0, Math.ceil(matching().length / size) - 1);
      state.page = { first: 0, prev: state.page - 1, next: state.page + 1, last }[go];
      render();
    });

    root.addEventListener('click', (e) => {
      const tr = e.target.closest('tr[data-href]');
      if (tr && !e.target.closest('a')) location.href = tr.dataset.href;
    });

    const matches = (tr) =>
      Object.entries(state.filters).every(([k, v]) => !v || tr.getAttribute(`data-f-${k}`) === v) &&
      Object.entries(state.bools).every(([k, on]) => !on || tr.getAttribute(`data-f-${k}`) === '1') &&
      (!state.q || (tr.dataset.q ?? '').includes(state.q));

    const value = (tr, i) => {
      const td = tr.cells[i];
      return td.dataset.v === undefined ? (heads.find((h) => h.cellIndex === i).dataset.type === 'num' ? null : td.textContent.trim()) : Number(td.dataset.v);
    };

    function matching() {
      const kept = rows.filter(matches);
      if (state.original || state.sort === null) return kept;
      const d = state.desc ? -1 : 1;
      return kept.sort((a, b) => {
        const x = value(a, state.sort), y = value(b, state.sort);
        if (x === null && y === null) return 0;
        if (x === null) return 1;
        if (y === null) return -1;
        return (typeof x === 'number' ? x - y : collator.compare(x, y)) * d;
      });
    }

    function render() {
      const list = matching();
      const pages = Math.max(1, Math.ceil(list.length / size));
      state.page = Math.min(Math.max(state.page, 0), pages - 1);
      rows.forEach((tr) => (tr.hidden = true));
      list.slice(state.page * size, (state.page + 1) * size).forEach((tr) => {
        tr.hidden = false;
        body.insertBefore(tr, empty);
      });
      if (empty) empty.hidden = list.length > 0;
      if (count) count.textContent = `${list.length} de ${rows.length}`;
      heads.forEach((th) => {
        const on = state.sort === th.cellIndex;
        th.setAttribute('aria-sort', on ? (state.desc ? 'descending' : 'ascending') : 'none');
        th.querySelector('.arrow').textContent = on ? (state.desc ? ' ▼' : ' ▲') : '';
      });
      if (pager) window.renderPager(pager, { page: state.page, size, total: list.length });
    }

    applyView();
    render();
  }

  document.querySelectorAll('[data-table]').forEach(initTable);

  document.querySelectorAll('select[data-switch]').forEach((sel) => {
    const key = sel.dataset.switch;
    const apply = () => document.querySelectorAll(`[data-case-of="${key}"]`).forEach((el) => (el.hidden = el.dataset.case !== sel.value));
    sel.addEventListener('change', apply);
    apply();
  });
})();
