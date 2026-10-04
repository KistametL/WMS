// nuxt.config.ts
import vuetify from 'vite-plugin-vuetify'

export default defineNuxtConfig({
  css: ['vuetify/styles', '@mdi/font/css/materialdesignicons.min.css'],
  ssr: false,
  axios: {
    baseURL: process.env.NUXT_PUBLIC_BACKEND
  },
  runtimeConfig: {
    public: {
      backend: process.env.NUXT_PUBLIC_BACKEND
    }
  },
  build: {
    transpile: ['vuetify'],
  },
  vite: {
    plugins: [
      vuetify({ autoImport: true }),
    ],
  },
  modules: [
    '@pinia/nuxt'
  ]
})