import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { routes } from './core/routes'
import { installGuards } from './core/guards'
import './styles/tokens.css'
import './styles/base.css'
import './styles/ui.css'
import './styles/layout.css'

const router = createRouter({ history: createWebHistory(), routes, scrollBehavior: () => ({ top: 0 }) })
installGuards(router)

createApp(App).use(createPinia()).use(router).mount('#app')
