<script>
  // Ayuda breve junto a un encabezado: aparece al pasar el mouse, al enfocar con teclado o al tocar.
  // Usa posición fija para que no la recorte el scroll horizontal de las tablas.
  let { text } = $props();

  let open = $state(false);
  let x = $state(0);
  let y = $state(0);
  let button;

  function show() {
    const r = button.getBoundingClientRect();
    x = Math.min(Math.max(r.left + r.width / 2, 130), window.innerWidth - 130);
    y = r.bottom + 6;
    open = true;
  }
</script>

<svelte:window onscroll={() => (open = false)} />

<button
  type="button"
  class="hint"
  bind:this={button}
  aria-label="Qué significa"
  aria-expanded={open}
  onmouseenter={show}
  onmouseleave={() => (open = false)}
  onfocus={show}
  onblur={() => (open = false)}
  onclick={(e) => { e.stopPropagation(); open ? (open = false) : show(); }}
  onkeydown={(e) => e.key === 'Escape' && (open = false)}
>?</button>
{#if open}
  <span class="tip" role="tooltip" style="left:{x}px; top:{y}px">{text}</span>
{/if}

<style>
  .hint {
    display: inline-flex; align-items: center; justify-content: center; width: 11px; height: 11px; margin-left: 3px;
    padding: 0; border: 1px solid var(--muted); border-radius: 50%; background: none; color: var(--muted);
    font: 600 .5rem/1 inherit; cursor: help; vertical-align: middle;
  }
  .hint:hover, .hint:focus-visible { color: var(--accent); border-color: var(--accent); }
  .tip {
    position: fixed; transform: translateX(-50%); z-index: 50; width: max-content; max-width: 260px;
    background: var(--card); color: var(--fg); border: 1px solid var(--line); border-radius: var(--radius-sm);
    box-shadow: var(--shadow); padding: 6px 10px; font: 400 .78rem/1.35 inherit; text-transform: none;
    letter-spacing: normal; text-align: left; white-space: normal; pointer-events: none;
  }
</style>
