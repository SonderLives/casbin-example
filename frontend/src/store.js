import { reactive } from 'vue'
import api from './api'

// 极简全局状态（演示项目不引 Pinia，reactive 单例足够清晰）
export const auth = reactive({
  token: localStorage.getItem('token') || '',
  profile: null, // { username, roles: [{domain, roles}], permissions: [{domain, permissions}] }
})

export async function login(username, password) {
  const data = await api.post('/login', { username, password })
  auth.token = data.token
  auth.profile = data.profile
  localStorage.setItem('token', data.token)
}

// 刷新页面后恢复用户信息（token 还在 localStorage 里）
export async function fetchMe() {
  if (!auth.token || auth.profile) return
  try {
    const data = await api.get('/me')
    auth.profile = data.profile
  } catch {
    /* 401 已由拦截器处理（清除并跳登录） */
  }
}

export function logout() {
  auth.token = ''
  auth.profile = null
  localStorage.removeItem('token')
}

// expectAllowed 估算「当前用户对某资源某操作是否预计放行」。
// ⚠️ 它只是给按钮上色的前端提示（依据 /api/me 返回的权限列表），
//    真正的判定永远在后端 Casbin 中间件 —— 这正是本示例要传达的重点。
export function expectAllowed(dom, obj, act, owner = '') {
  const p = auth.profile
  if (!p) return false
  // ABAC：文档属主对自己的文档有全部权限（对应 model.conf 匹配器的属主分支）
  if (obj === 'docs' && owner && owner === p.username) return true
  const entry = (p.permissions || []).find((x) => x.domain === dom)
  if (!entry) return false
  // 后端返回的每条权限是 [sub, dom, obj, act] 四元组（已含角色展开）
  return entry.permissions.some(
    ([, d, o, a]) => d === dom && (o === obj || o === '*') && (a === act || a === '*')
  )
}
