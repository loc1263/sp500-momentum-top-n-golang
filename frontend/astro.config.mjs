// Sitio estático: cada página sale como <nombre>.html en la raíz (los enlaces que arma Go son relativos a ella).
// En GitHub Pages el sitio vive en /<repositorio>/: el workflow pasa esa ruta en SITE_BASE.
import { defineConfig } from 'astro/config';
import alpinejs from '@astrojs/alpinejs';

export default defineConfig({
  base: process.env.SITE_BASE || '/',
  build: { format: 'file' },
  devToolbar: { enabled: false },
  integrations: [alpinejs({ entrypoint: '/src/alpine' })],
});
