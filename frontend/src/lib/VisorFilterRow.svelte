<script>
  import { isText, label } from './visor.js';

  // Fila de filtros bajo los encabezados: texto, mín/máx según el tipo de columna, o SUPERA / NO SUPERA
  let { cols, filters, onchange } = $props(); // onchange(col, key, value)
</script>

<tr class="filters">
  {#each cols as col (col)}
    <th class:r={!isText(col)}>
      {#if col === 'vs_spy'}
        <select aria-label="Filtrar {label(col)}" value={filters[col]?.eq ?? ''} onchange={(e) => onchange(col, 'eq', e.currentTarget.value)}>
          <option value="">Todos</option>
          <option value="SUPERA">SUPERA</option>
          <option value="NO SUPERA">NO SUPERA</option>
        </select>
      {:else if isText(col)}
        <input type="search" aria-label="Filtrar {label(col)}" placeholder="Filtrar" value={filters[col]?.text ?? ''}
          oninput={(e) => onchange(col, 'text', e.currentTarget.value)} />
      {:else}
        <span class="range-in">
          <input type="number" step="any" aria-label="{label(col)} mín" placeholder="mín" value={filters[col]?.min ?? ''}
            oninput={(e) => onchange(col, 'min', e.currentTarget.value)} />
          <input type="number" step="any" aria-label="{label(col)} máx" placeholder="máx" value={filters[col]?.max ?? ''}
            oninput={(e) => onchange(col, 'max', e.currentTarget.value)} />
        </span>
      {/if}
    </th>
  {/each}
</tr>

<style>
  .filters th { font-weight: 400; text-transform: none; letter-spacing: 0; vertical-align: top; }
  input, select {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm); padding: 2px 4px; color: var(--fg);
    font: inherit; font-size: .75rem; width: 100%; min-width: 0; box-sizing: border-box;
  }
  input[type='number'] { width: 56px; }
  .range-in { display: inline-flex; gap: 2px; }
</style>
