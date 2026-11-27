// Paginador compartido (« ‹ Página N de M › ): dibuja los botones dentro de nav, con los atributos data-go que leen visor.js y
// table.js, y lo oculta si todas las filas caben en una página.
//
// Fecha: 2026-11-19

window.renderPager = (nav, { page, size, total }) => {
  const pages = Math.max(1, Math.ceil(total / size));
  nav.hidden = total <= size;
  if (nav.hidden) return;
  const btn = (go, label, aria, off) => `<button type="button" data-go="${go}" aria-label="${aria}"${off ? ' disabled' : ''}>${label}</button>`;
  nav.innerHTML =
    `<span class="muted">${page * size + 1}–${Math.min(total, (page + 1) * size)} de ${total}</span>` +
    `<span class="btns">${btn('first', '«', 'Primera página', page === 0)}${btn('prev', '‹', 'Página anterior', page === 0)}` +
    `<span class="cur">Página ${page + 1} de ${pages}</span>` +
    `${btn('next', '›', 'Página siguiente', page >= pages - 1)}${btn('last', '»', 'Última página', page >= pages - 1)}</span>`;
};
