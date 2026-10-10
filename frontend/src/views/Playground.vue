<script setup>
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'

const USERS = ['alice', 'bob', 'charlie', 'dave', 'eve']
const DOMAINS = ['tenant-a', 'tenant-b']
const OBJS = ['docs', 'policies']
const ACTS = ['read', 'write', 'delete']

const st = reactive({
  form: { sub: 'bob', dom: 'tenant-a', obj: 'docs', act: 'delete', owner: '' },
  result: null, // { allowed, expression, request }
  loading: false,
})

// 几个精心挑选的试验，覆盖 RBAC / domain / ABAC 三种机制
const examples = [
  { label: 'alice 删 tenant-a 文档（admin）→ 应 true', form: { sub: 'alice', dom: 'tenant-a', obj: 'docs', act: 'delete', owner: '' } },
  { label: 'bob 删 tenant-a 文档（editor 无 delete）→ 应 false', form: { sub: 'bob', dom: 'tenant-a', obj: 'docs', act: 'delete', owner: '' } },
  { label: 'bob 删「属主是 bob」的文档（ABAC）→ 应 true', form: { sub: 'bob', dom: 'tenant-a', obj: 'docs', act: 'delete', owner: 'bob' } },
  { label: 'eve 读 tenant-b 文档（无角色）→ 应 false', form: { sub: 'eve', dom: 'tenant-b', obj: 'docs', act: 'read', owner: '' } },
  { label: 'eve 读「属主是 eve」的笔记（ABAC）→ 应 true', form: { sub: 'eve', dom: 'tenant-b', obj: 'docs', act: 'read', owner: 'eve' } },
  { label: 'alice 写 tenant-b 策略（跨域）→ 应 false', form: { sub: 'alice', dom: 'tenant-b', obj: 'policies', act: 'write', owner: '' } },
]

async function run(form) {
  Object.assign(st.form, form)
  st.loading = true
  try {
    st.result = await api.post('/playground', st.form)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    st.loading = false
  }
}
</script>

<template>
  <el-card>
    <h3>Enforce 试验场</h3>
    <p class="tip">
      对任意「用户(sub) - 租户(dom) - 资源(obj) - 操作(act)」组合直接调用一次
      <code>Enforce</code>，即时返回 true / false。
      <b>owner</b> 用来模拟资源的属主属性：留空等价于「只有 RBAC 判定」；
      填上某人名字再看结果变化 —— 这就是 ABAC（基于属性的访问控制）。
    </p>

    <el-form inline>
      <el-form-item label="sub">
        <el-select v-model="st.form.sub" style="width: 110px">
          <el-option v-for="u in USERS" :key="u" :value="u" />
        </el-select>
      </el-form-item>
      <el-form-item label="dom">
        <el-select v-model="st.form.dom" style="width: 130px">
          <el-option v-for="d in DOMAINS" :key="d" :value="d" />
        </el-select>
      </el-form-item>
      <el-form-item label="obj">
        <el-select v-model="st.form.obj" style="width: 120px">
          <el-option v-for="o in OBJS" :key="o" :value="o" />
        </el-select>
      </el-form-item>
      <el-form-item label="act">
        <el-select v-model="st.form.act" style="width: 110px">
          <el-option v-for="a in ACTS" :key="a" :value="a" />
        </el-select>
      </el-form-item>
      <el-form-item label="owner">
        <el-input v-model="st.form.owner" placeholder="属主（可空）" style="width: 130px" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="st.loading" @click="run({})">开始判定</el-button>
      </el-form-item>
    </el-form>

    <el-alert
      v-if="st.result"
      :type="st.result.allowed ? 'success' : 'error'"
      :closable="false"
      class="result"
    >
      <template #title>
        {{ st.result.expression }} ⇒ {{ st.result.allowed ? '✅ true —— 放行' : '❌ false —— 拒绝' }}
      </template>
    </el-alert>

    <el-divider>快速试验（点击即判定）</el-divider>
    <div class="examples">
      <el-button v-for="ex in examples" :key="ex.label" plain size="small" @click="run(ex.form)">
        {{ ex.label }}
      </el-button>
    </div>
  </el-card>
</template>

<style scoped>
.tip { color: #606266; font-size: 13px; line-height: 1.8; }
.result { margin-top: 10px; }
.examples { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
</style>
