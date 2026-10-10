import { createRouter, createWebHistory } from 'vue-router'

import { auth } from '../store'
import Docs from '../views/Docs.vue'
import Login from '../views/Login.vue'
import Playground from '../views/Playground.vue'
import Policies from '../views/Policies.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', redirect: '/docs' },
    { path: '/docs', component: Docs, meta: { auth: true } },
    { path: '/policies', component: Policies, meta: { auth: true } },
    { path: '/playground', component: Playground, meta: { auth: true } },
  ],
})

// 前端路由守卫只是「别让没登录的人看到界面」；
// 就算绕过守卫直接调接口，后端 JWT/Casbin 中间件也会拦下 —— 安全线永远在后端。
router.beforeEach((to) => {
  if (to.meta.auth && !auth.token) return '/login'
})

export default router
