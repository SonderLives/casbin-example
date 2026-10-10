<script setup>
import { onMounted, reactive, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

// 预置账号与角色（与后端 store 保持一致，仅供表单下拉用）
const USERS = ['alice', 'bob', 'charlie', 'dave', 'eve']
const ROLES = ['admin', 'editor', 'viewer']
const OBJS = ['docs', 'policies']
const ACTS = ['*', 'read', 'write', 'delete']

const st = reactive({
  domain: 'tenant-a',
  loading: false,
  pRules: [], // [[sub, dom, obj, act]]
  gRules: [], // [[user, role, dom]]
  pForm: { sub: 'eve', obj: 'docs', act: 'read' },
  gForm: { user: 'eve', role: 'viewer' },
})

async function load() {
  st.loading = true
  try {
    const data = await api.get('/policies', { params: { dom: st.domain } })
    st.pRules = data.p || []
    st.gRules = data.g || []
  } catch (e) {
    st.pRules = []
    st.gRules = []
    ElMessage.error(e.message) // 非 admin 会在这里吃到 403 —— 管理接口本身也受 Casbin 保护
  } finally {
    st.loading = false
  }
}
watch(() => st.domain, load)
onMounted(load)

async function addP() {
  try {
    await api.post('/policies', { type: 'p', ...st.pForm }, { params: { dom: st.domain } })
    ElMessage.success('p 规则已添加，权限即时生效')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function addG() {
  try {
    await api.post('/policies', { type: 'g', ...st.gForm }, { params: { dom: st.domain } })
    ElMessage.success('g 规则已添加（用户获得角色），权限即时生效')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function removeRule(type, row) {
  const body =
    type === 'p'
      ? { type: 'p', sub: row[0], obj: row[2], act: row[3] }
      : { type: 'g', user: row[0], role: row[1] }
  try {
    await ElMessageBox.confirm(`移除规则：${row.join(', ')}？移除后权限立即收回。`, '确认', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await api.post('/policies/remove', body, { params: { dom: st.domain } })
    ElMessage.success('已移除')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>

<template>
  <div>
    <el-alert type="info" :closable="false" class="note">
      <b>教学提示：</b>这个页面调用的接口本身也受 Casbin 保护（资源名 <code>policies</code>）——
      只有当前域的 <code>admin</code> 能查看/修改策略，而且只能管理自己是 admin 的域（多租户隔离）。
      用 bob 或 charlie 登录试试，会直接 403。
      <code>owner</code> 是保留主体名（承载 model.conf 匹配器里的 ABAC 属主分支），不能删除。
    </el-alert>

    <el-card>
      <div class="toolbar">
        <span>管理哪个域的策略：</span>
        <el-radio-group v-model="st.domain">
          <el-radio-button value="tenant-a">tenant-a</el-radio-button>
          <el-radio-button value="tenant-b">tenant-b</el-radio-button>
        </el-radio-group>
      </div>

      <h3>p 规则 —— 授权：主体(sub) 在本域对资源(obj) 可执行操作(act)</h3>
      <el-table :data="st.pRules" v-loading="st.loading" border size="small">
        <el-table-column label="sub" prop="0" width="110" />
        <el-table-column label="dom（当前域）" prop="1" width="130" />
        <el-table-column label="obj" prop="2" />
        <el-table-column label="act" prop="3" width="110" />
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-tooltip
              v-if="row[0] === 'owner'"
              content="ABAC 占位规则，承载「属主放行」分支，不能删除"
              placement="top"
            >
              <el-button size="small" link disabled>系统占位</el-button>
            </el-tooltip>
            <el-button v-else size="small" type="danger" link @click="removeRule('p', row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="add">
        <el-input v-model="st.pForm.sub" placeholder="主体（用户或角色，如 eve / viewer）" style="width: 230px" />
        <el-select v-model="st.pForm.obj" style="width: 120px">
          <el-option v-for="o in OBJS" :key="o" :value="o" :label="o" />
        </el-select>
        <el-select v-model="st.pForm.act" style="width: 110px">
          <el-option v-for="a in ACTS" :key="a" :value="a" :label="a" />
        </el-select>
        <el-button type="primary" @click="addP">添加 p 规则</el-button>
        <span class="tip">试试：给 eve 授 viewer 角色，或直接加一条 p 规则，切到文档页看效果。</span>
      </div>

      <h3>g 规则 —— 角色：用户(user) 在本域拥有角色(role)</h3>
      <el-table :data="st.gRules" v-loading="st.loading" border size="small">
        <el-table-column label="user" prop="0" width="110" />
        <el-table-column label="role" prop="1" width="130" />
        <el-table-column label="dom（当前域）" prop="2" />
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" type="danger" link @click="removeRule('g', row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="add">
        <el-select v-model="st.gForm.user" style="width: 130px">
          <el-option v-for="u in USERS" :key="u" :value="u" :label="u" />
        </el-select>
        <el-select v-model="st.gForm.role" style="width: 130px">
          <el-option v-for="r in ROLES" :key="r" :value="r" :label="r" />
        </el-select>
        <el-button type="primary" @click="addG">添加 g 规则</el-button>
        <span class="tip">试试：把 eve 挂到 viewer，再切到文档页 —— eve 立刻能读了。</span>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.note { margin-bottom: 14px; }
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.add { display: flex; gap: 10px; margin: 12px 0 24px; align-items: center; flex-wrap: wrap; }
.tip { color: #909399; font-size: 12px; }
</style>
