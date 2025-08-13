// @ts-check
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  site: 'https://gotunnel.dev',
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
