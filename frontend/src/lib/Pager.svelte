<script>
  import { PAGE_SIZE } from './format.js';

  /** Paginador de tablas: solo aparece si hay más de PAGE_SIZE filas. `page` es 0-based y se enlaza con la vista. */
  let { total, page = $bindable(0) } = $props();

  const pages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

  // Si el filtro deja menos páginas que la actual, se vuelve a la última
  $effect(() => {
    if (page > pages - 1) page = pages - 1;
  });
</script>

{#if total > PAGE_SIZE}
  <nav class="pager" aria-label="Paginación de la tabla">
    <span class="muted">{page * PAGE_SIZE + 1}–{Math.min(total, (page + 1) * PAGE_SIZE)} de {total}</span>
    <span class="btns">
      <button onclick={() => (page = 0)} disabled={page === 0} aria-label="Primera página">«</button>
      <button onclick={() => (page -= 1)} disabled={page === 0} aria-label="Página anterior">‹</button>
      <span class="cur">Página {page + 1} de {pages}</span>
      <button onclick={() => (page += 1)} disabled={page >= pages - 1} aria-label="Página siguiente">›</button>
      <button onclick={() => (page = pages - 1)} disabled={page >= pages - 1} aria-label="Última página">»</button>
    </span>
  </nav>
{/if}

<style>
  .pager {
    display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 8px;
    margin: var(--space-2) 0 var(--space-3); font-size: .82rem; font-variant-numeric: tabular-nums;
  }
  .btns { display: inline-flex; align-items: center; gap: 4px; }
  .cur { padding: 0 var(--space-2); color: var(--muted); }
  button {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm);
    min-width: 28px; height: 26px; cursor: pointer; line-height: 1;
  }
  button:hover:not(:disabled) { border-color: var(--muted); }
  button:disabled { opacity: .4; cursor: default; }
</style>
