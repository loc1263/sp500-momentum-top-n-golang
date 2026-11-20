// Interactividad del Visor sobre el HTML que arma Go: pestañas, búsqueda de varios tickers, filtros por columna, orden,
// paginación, marca de fila y CSV. Trabaja con los valores crudos del <script id="data"> y solo mueve / oculta las filas.
// Sin dependencias ni build. Lógica equivalente a frontend/src/lib/visor.js.
//
// Fecha: 2026-11-19

(() => {
  const PAGE_SIZE = 12;
  const data = JSON.parse(document.getElementById('data').textContent);
  const $ = (sel, root = document) => root.querySelector(sel);

  const sections = data.sections.map((sec) => {
    const wrap = $(`.wrap[data-sec="${sec.id}"]`);
    const body = $('tbody', wrap);
    const trs = [...body.querySelectorAll('tr[data-i]')];
    return { ...sec, wrap, body, trs, empty: $('tr.empty', body), heads: [...wrap.querySelectorAll('.sort')], index: new Map(sec.rows.map((r, i) => [r, i])) };
  });

  const state = { section: 0, query: '', filters: {}, sortIndex: null, descending: true, page: 0, marked: null };

  const toNumber = (v) => {
    const t = String(v ?? '').trim();
    return t === '' || Number.isNaN(Number(t)) ? null : Number(t);
  };
  const hasFilter = (f) => !!f && (String(f.text ?? '').trim() !== '' || toNumber(f.min) !== null || toNumber(f.max) !== null || !!f.eq);

  function filterRows(sec, query) {
    const terms = query.split(',').map((t) => t.trim().toLowerCase()).filter(Boolean);
    if (!terms.length) return sec.rows;
    const tickerAt = sec.cols.indexOf('ticker');
    const isTicker = (t) => terms.length > 1 && tickerAt >= 0 && sec.rows.some((r) => String(r[tickerAt]).toLowerCase() === t);
    return sec.rows.filter((r) =>
      terms.some((t) => (isTicker(t) ? String(r[tickerAt]).toLowerCase() === t : sec.search.some((i) => String(r[i] ?? '').toLowerCase().includes(t)))),
    );
  }

  function filterColumns(sec, rows, filters) {
    const active = sec.cols.map((col, i) => [col, i, filters[col]]).filter(([, , f]) => hasFilter(f));
    if (!active.length) return rows;
    return rows.filter((r) =>
      active.every(([col, i, f]) => {
        const v = r[i];
        if (col === 'vs_spy') return !f.eq || v === f.eq;
        if (sec.text[i]) return String(v ?? '').toLowerCase().includes(String(f.text ?? '').trim().toLowerCase());
        const lo = toNumber(f.min), hi = toNumber(f.max);
        if (lo === null && hi === null) return true;
        return v !== null && v !== undefined && (lo === null || v >= lo) && (hi === null || v <= hi);
      }),
    );
  }

  function sortRows(rows, index, descending) {
    if (index === null) return rows;
    const d = descending ? -1 : 1;
    return [...rows].sort((a, b) => {
      const x = a[index], y = b[index];
      if (x === null && y === null) return 0;
      if (x === null) return 1;
      if (y === null) return -1;
      return (typeof x === 'number' ? x - y : String(x).localeCompare(String(y), 'es')) * d;
    });
  }

  function toCsv(sec, rows) {
    const cell = (v) => {
      if (v === null || v === undefined) return '';
      const s = String(v);
      return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
    };
    return [sec.cols, ...rows].map((r) => r.map(cell).join(',')).join('\n');
  }

  const current = () => {
    const sec = sections[state.section];
    return { sec, rows: sortRows(filterColumns(sec, filterRows(sec, state.query), state.filters), state.sortIndex, state.descending) };
  };

  function render() {
    const { sec, rows } = current();
    const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
    state.page = Math.min(state.page, pages - 1);

    const shown = rows.slice(state.page * PAGE_SIZE, (state.page + 1) * PAGE_SIZE);
    for (const tr of sec.trs) tr.hidden = true;
    for (const r of shown) {
      const tr = sec.trs[sec.index.get(r)];
      tr.hidden = false;
      sec.body.insertBefore(tr, sec.empty);
    }
    sec.empty.hidden = rows.length > 0;

    sec.heads.forEach((b, i) => {
      const on = state.sortIndex === i;
      b.parentElement.setAttribute('aria-sort', on ? (state.descending ? 'descending' : 'ascending') : 'none');
      $('.arrow', b).textContent = on ? (state.descending ? ' ▼' : ' ▲') : '';
    });

    document.querySelectorAll('.seg button').forEach((b, i) => {
      b.classList.toggle('active', i === state.section);
      b.setAttribute('aria-selected', String(i === state.section));
      $('.n', b).textContent = i === state.section ? rows.length : sections[i].rows.length;
    });
    sections.forEach((s, i) => (s.wrap.hidden = i !== state.section));

    const nFilters = Object.values(state.filters).filter(hasFilter).length;
    $('#clear').hidden = nFilters === 0;
    $('#clear-n').textContent = nFilters;
    $('#csv').disabled = rows.length === 0;
    renderPager(rows.length, pages);
  }

  function renderPager(total, pages) {
    const nav = $('#pager');
    nav.hidden = total <= PAGE_SIZE;
    if (nav.hidden) return;
    const p = state.page;
    const btn = (go, label, aria, off) => `<button type="button" data-go="${go}" aria-label="${aria}"${off ? ' disabled' : ''}>${label}</button>`;
    nav.innerHTML =
      `<span class="muted">${p * PAGE_SIZE + 1}–${Math.min(total, (p + 1) * PAGE_SIZE)} de ${total}</span>` +
      `<span class="btns">${btn('first', '«', 'Primera página', p === 0)}${btn('prev', '‹', 'Página anterior', p === 0)}` +
      `<span class="cur">Página ${p + 1} de ${pages}</span>` +
      `${btn('next', '›', 'Página siguiente', p >= pages - 1)}${btn('last', '»', 'Última página', p >= pages - 1)}</span>`;
  }

  function resetFilters() {
    state.filters = {};
    document.querySelectorAll('.filters input, .filters select').forEach((el) => (el.value = ''));
  }

  function selectSection(i) {
    state.section = i;
    state.sortIndex = null;
    state.page = 0;
    resetFilters();
    render();
  }

  function sortBy(i) {
    const sec = sections[state.section];
    if (state.sortIndex === i) state.descending = !state.descending;
    else {
      state.sortIndex = i;
      state.descending = !sec.text[i];
    }
    state.page = 0;
    render();
  }

  function download() {
    const { sec, rows } = current();
    const url = URL.createObjectURL(new Blob([toCsv(sec, rows)], { type: 'text/csv;charset=utf-8' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `visor_${sec.id}_${data.date}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }

  document.querySelectorAll('.seg button').forEach((b, i) => b.addEventListener('click', () => selectSection(i)));
  $('#q').addEventListener('input', (e) => {
    state.query = e.target.value;
    state.page = 0;
    render();
  });
  $('#clear').addEventListener('click', () => {
    resetFilters();
    state.page = 0;
    render();
  });
  $('#csv').addEventListener('click', download);

  const onFilter = (e) => {
    const el = e.target;
    if (!el.dataset.key) return;
    state.filters = { ...state.filters, [el.dataset.col]: { ...state.filters[el.dataset.col], [el.dataset.key]: el.value } };
    state.page = 0;
    render();
  };
  document.addEventListener('input', onFilter);
  document.addEventListener('change', (e) => e.target.tagName === 'SELECT' && onFilter(e));

  document.addEventListener('click', (e) => {
    const sort = e.target.closest('.sort');
    if (sort) return sortBy(Number(sort.dataset.col));
    const go = e.target.closest('[data-go]');
    if (go) {
      const { rows } = current();
      const last = Math.max(0, Math.ceil(rows.length / PAGE_SIZE) - 1);
      state.page = { first: 0, prev: state.page - 1, next: state.page + 1, last }[go.dataset.go];
      return render();
    }
    const tr = e.target.closest('tbody tr[data-i]');
    if (tr && !e.target.closest('a')) {
      const on = !tr.classList.contains('marked');
      document.querySelectorAll('tr.marked').forEach((r) => r.classList.remove('marked'));
      tr.classList.toggle('marked', on);
    }
  });

  render();
})();
