<script setup lang="ts">
import SessionNotice from '../ui/SessionNotice.vue'
import { watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useApi } from '../../composables/useApi'
import { useSession } from '../../composables/useSession'
import { useUiStore } from '../../stores/ui'

const api = useApi()
const session = useSession()
const ui = useUiStore()
const route = useRoute()
const router = useRouter()

watch(() => route.fullPath, () => ui.closeMobileNav())

async function logout() {
  await api.logout()
  await router.push('/admin/login')
}
</script>

<template>
  <div class="layout">
    <a class="skip-link" href="#main">跳到主要内容</a>
    <header class="site-header">
      <div class="site-header-inner">
        <RouterLink class="brand" to="/admin"><span class="brand-mark">G</span>GCOJ<span class="brand-scope">管理</span></RouterLink>
        <button class="nav-toggle quiet sm" type="button" :aria-expanded="ui.mobileNavOpen" aria-controls="admin-nav" @click="ui.toggleMobileNav()">
          {{ ui.mobileNavOpen ? '收起导航' : '菜单' }}
        </button>
        <nav id="admin-nav" class="site-nav" :class="{ open: ui.mobileNavOpen }" aria-label="管理端导航">
          <RouterLink :to="{ name: 'admin-home' }" exact>工作台</RouterLink>
          <RouterLink :to="{ name: 'admin-problems' }">题目管理</RouterLink>
          <RouterLink :to="{ name: 'admin-submissions' }">提交与重判</RouterLink>
          <RouterLink :to="{ name: 'admin-contests' }">比赛管理</RouterLink>
          <RouterLink :to="{ name: 'admin-users' }">用户管理</RouterLink>
        </nav>
        <div class="header-side">
          <template v-if="session">
            <span class="session-chip">
              <RouterLink class="username" :to="{ name: 'admin-account' }">{{ session.user.username }}</RouterLink>
              <button class="link sm" type="button" @click="logout">退出</button>
            </span>
          </template>
          <RouterLink class="btn btn-quiet sm" to="/">返回用户端</RouterLink>
        </div>
      </div>
    </header>
    <main id="main" class="layout-main" tabindex="-1">
      <div class="container">
        <SessionNotice />
        <RouterView />
      </div>
    </main>
    <footer class="site-footer">GCOJ 管理工作台</footer>
  </div>
</template>
