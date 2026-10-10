<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { auth, fetchMe, logout } from './store'

const router = useRouter()

onMounted(() => fetchMe())

function onLogout() {
  logout()
  router.push('/login')
}
</script>

<template>
  <el-container class="layout">
    <el-header class="header">
      <div class="brand">🔐 Casbin 权限实验室</div>
      <el-menu mode="horizontal" router :default-active="$route.path" :ellipsis="false" class="menu">
        <el-menu-item index="/docs">文档管理</el-menu-item>
        <el-menu-item index="/policies">策略管理</el-menu-item>
        <el-menu-item index="/playground">Playground</el-menu-item>
      </el-menu>
      <div v-if="auth.profile" class="user">
        <b>{{ auth.profile.username }}</b>
        <el-tag v-for="r in auth.profile.roles" :key="r.domain" size="small" type="info">
          {{ r.domain }}: {{ r.roles.join('/') }}
        </el-tag>
        <el-button link type="danger" @click="onLogout">退出</el-button>
      </div>
    </el-header>

    <el-main>
      <router-view />
    </el-main>
  </el-container>
</template>

<style>
body { margin: 0; background: #f5f7fa; }
.layout { min-height: 100vh; }
.header { display: flex; align-items: center; gap: 24px; background: #fff; border-bottom: 1px solid #e4e7ed; }
.brand { font-weight: 700; white-space: nowrap; }
.menu { flex: 1; border-bottom: none !important; }
.user { display: flex; align-items: center; gap: 8px; white-space: nowrap; }
</style>
