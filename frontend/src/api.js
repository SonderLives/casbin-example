import axios from 'axios'

// 统一的 API 客户端：
//  - 请求拦截器：自动带上 JWT（Authorization: Bearer <token>）
//  - 响应拦截器：直接返回 data；401 清会话跳登录；错误统一转成 {message, request?}
const api = axios.create({ baseURL: '/api' })

api.interceptors.request.use((cfg) => {
  const token = localStorage.getItem('token')
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

api.interceptors.response.use(
  (resp) => resp.data,
  (err) => {
    const status = err.response?.status
    const data = err.response?.data || {}
    if (status === 401) {
      localStorage.removeItem('token')
      if (location.pathname !== '/login') location.href = '/login'
    }
    // 后端 403 时会附带回传被拦截的 Enforce 四元组 [sub, dom, obj, act]，展示出来很有教学价值
    return Promise.reject({ status, message: data.message || err.message, request: data.request })
  }
)

export default api
