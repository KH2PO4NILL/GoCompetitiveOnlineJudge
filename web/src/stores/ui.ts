import { defineStore } from 'pinia'

// Pinia 仅承载纯客户端 UI 状态；服务端数据一律走 vue-query 缓存
export const useUiStore = defineStore('ui', {
  state: () => ({ mobileNavOpen: false }),
  actions: {
    toggleMobileNav() { this.mobileNavOpen = !this.mobileNavOpen },
    closeMobileNav() { this.mobileNavOpen = false },
  },
})
