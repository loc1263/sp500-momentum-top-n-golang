<script>
  // Distintivo de un nivel (bien / atención / alerta / informativo / sin datos). Siempre lleva texto: el color no es lo único que informa.
  let { level, text } = $props();
</script>

<span class="pill {level}">{text}</span>

<style>
  .pill {
    display: inline-block; padding: 0 8px; border: 1px solid var(--line); border-radius: 999px; font-size: .72rem;
    font-weight: 600; line-height: 1.6; white-space: nowrap; color: var(--muted);
  }
  .good { border-color: var(--pos); color: var(--pos); }
  .warn { border-color: var(--s4); color: var(--warn); }
  .bad { border-color: var(--neg); color: var(--neg); }
</style>
