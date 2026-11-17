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
  await router.push('/')
}
</script>

<template>
  <div class="layout">
    <a class="skip-link" href="#main">跳到主要内容</a>
    <header class="site-header">
      <div class="site-header-inner">
        <RouterLink class="brand" to="/"><span class="brand-mark">G</span>GCOJ</RouterLink>
        <button class="nav-toggle quiet sm" type="button" :aria-expanded="ui.mobileNavOpen" aria-controls="site-nav" @click="ui.toggleMobileNav()">
          {{ ui.mobileNavOpen ? '收起导航' : '菜单' }}
        </button>
        <nav id="site-nav" class="site-nav" :class="{ open: ui.mobileNavOpen }" aria-label="用户端导航">
          <RouterLink to="/">首页</RouterLink>
          <RouterLink to="/problems">题库</RouterLink>
          <RouterLink to="/contests">比赛</RouterLink>
          <RouterLink v-if="session" to="/submit">提交代码</RouterLink>
          <RouterLink v-if="session" to="/submissions">评测记录</RouterLink>
        </nav>
        <div class="header-side">
          <template v-if="session">
            <span class="session-chip">
              <RouterLink class="username" to="/account">{{ session.user.username }}</RouterLink>
              <button class="link sm" type="button" @click="logout">退出</button>
            </span>
          </template>
          <RouterLink v-else class="btn btn-quiet sm" to="/login">登录 / 注册</RouterLink>
        </div>
      </div>
    </header>
    <main id="main" class="layout-main" tabindex="-1">
      <div class="container">
        <SessionNotice />
        <RouterView />
      </div>
    </main>
    <footer class="site-footer">GCOJ · 算法竞赛在线评测平台</footer>
  </div>
</template>
