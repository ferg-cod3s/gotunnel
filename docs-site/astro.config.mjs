// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

// https://astro.build/config
export default defineConfig({
  site: 'https://gotunnel.dev',
  integrations: [sitemap()],
  build: {
    format: 'directory'
  },
  markdown: {
    syntaxHighlight: 'prism',
    shikiConfig: {
      themes: {
        light: 'github-light',
        dark: 'github-dark'
      }
    }
  }
});
