<script setup>
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { login } from '../store'

const router = useRouter()
const form = reactive({ username: '', password: '123456' })
const loading = reactive({})

// 预置用户速查：点卡片直接登录，直观对比不同身份的权限差异
const presets = [
  { username: 'alice', desc: 'tenant-a 管理员 · tenant-b 访客' },
  { username: 'bob', desc: 'tenant-a 编辑：可读写，删不了别人的文档' },
  { username: 'charlie', desc: 'tenant-a 访客：只读，但自己的文档例外' },
  { username: 'dave', desc: 'tenant-b 管理员' },
  { username: 'eve', desc: '无任何角色，全靠 ABAC 属性（文档属主）' },
]

async function doLogin(username, password) {
  await login(username, password)
  ElMessage.success(`欢迎，${username}`)
  router.push('/docs')
}

async function quick(u) {
  loading[u] = true
  try {
    await doLogin(u, '123456')
  } catch (e) {
    ElMessage.error(e.message || '登录失败')
  } finally {
    loading[u] = false
  }
}

async function submit() {
  try {
    await doLogin(form.username, form.password)
  } catch (e) {
    ElMessage.error(e.message || '登录失败')
  }
}
</script>

<template>
  <div class="wrap">
    <el-card class="card">
      <h2>🔐 Casbin 权限实验室</h2>
      <p class="tip">
        登录只做<b>认证</b>（JWT 认出你是谁），能干什么由后端 <b>Casbin</b> 决定。
        演示账号密码均为 <el-tag size="small">123456</el-tag>，点卡片一键登录。
      </p>
      <div class="presets">
        <el-button
          v-for="p in presets"
          :key="p.username"
          class="preset"
          :loading="loading[p.username]"
          @click="quick(p.username)"
        >
          <div class="name">{{ p.username }}</div>
          <div class="desc">{{ p.desc }}</div>
        </el-button>
      </div>
      <el-divider>或手动登录</el-divider>
      <el-form label-width="70px" @submit.prevent="submit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="alice / bob / charlie / dave / eve" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-button type="primary" native-type="submit" class="go">登 录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.wrap { display: flex; justify-content: center; padding-top: 7vh; }
.card { width: 620px; }
.tip { color: #606266; font-size: 13px; line-height: 1.8; }
.presets { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.preset { height: auto; padding: 10px 12px; margin: 0 !important; text-align: left; }
.preset .name { font-weight: 700; }
.preset .desc { font-size: 12px; color: #909399; white-space: normal; line-height: 1.5; }
.go { width: 100%; }
</style>
