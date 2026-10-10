<script setup>
import { onMounted, reactive, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { expectAllowed } from '../store'

// ---- 页面状态 ----
const st = reactive({
  domain: 'tenant-a',
  docs: [],
  loading: false,
  view: { visible: false, mode: 'view', form: {} }, // 查看 / 编辑共用对话框
  create: { visible: false, form: { title: '', content: '' } },
})

async function load() {
  st.loading = true
  try {
    const data = await api.get(`/domains/${st.domain}/docs`)
    st.docs = data.docs || []
  } catch (e) {
    st.docs = []
    showErr(e)
  } finally {
    st.loading = false
  }
}
watch(() => st.domain, load)
onMounted(load)

// 统一错误展示：把后端 403 附带的 Enforce 四元组一起显示，直观看「是谁被哪条规则拦了」
function showErr(e) {
  const req = e.request ? `（Enforce: ${e.request.join(', ')}）` : ''
  ElMessage.error(`${e.message}${req}`)
}

function hint(row, act) {
  return expectAllowed(st.domain, 'docs', act, row.owner)
    ? '预计允许（点击验证后端判定）'
    : '预计无权限 —— 试试点击，会被后端 Casbin 拦下'
}

function btnType(row, act) {
  return expectAllowed(st.domain, 'docs', act, row.owner) ? 'primary' : 'info'
}

function openView(row) {
  st.view = { visible: true, mode: 'view', form: { ...row } }
}
function openEdit(row) {
  st.view = { visible: true, mode: 'edit', form: { ...row } }
}

async function save() {
  try {
    await api.put(`/domains/${st.domain}/docs/${st.view.form.id}`, {
      title: st.view.form.title,
      content: st.view.form.content,
    })
    ElMessage.success('保存成功')
    st.view.visible = false
    load()
  } catch (e) {
    showErr(e)
  }
}

async function create() {
  try {
    await api.post(`/domains/${st.domain}/docs`, st.create.form)
    ElMessage.success('创建成功，你是它的属主（Owner）')
    st.create.visible = false
    st.create.form = { title: '', content: '' }
    load()
  } catch (e) {
    showErr(e)
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.title}」？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.delete(`/domains/${st.domain}/docs/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    showErr(e)
  }
}
</script>

<template>
  <div>
    <el-alert type="info" :closable="false" class="note">
      <b>教学提示：</b>按钮颜色只是前端依据 /api/me 的「预估提示」；
      真正的判定在每个请求的后端 Casbin 中间件里。点一个灰色按钮，观察 403 与被拦截的四元组。
      属主（owner）对自己的文档有全部权限 —— 这就是 ABAC。
    </el-alert>

    <el-card>
      <div class="toolbar">
        <span>租户 domain：</span>
        <el-radio-group v-model="st.domain">
          <el-radio-button value="tenant-a">tenant-a</el-radio-button>
          <el-radio-button value="tenant-b">tenant-b</el-radio-button>
        </el-radio-group>
        <el-button type="primary" @click="st.create.visible = true">新建文档</el-button>
        <span class="tip">换租户试试：同一用户在不同域的权限不同（RBAC + domain）。</span>
      </div>

      <el-table :data="st.docs" v-loading="st.loading" border>
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="owner" label="属主" width="110" />
        <el-table-column label="操作" width="240">
          <template #default="{ row }">
            <el-tooltip :content="hint(row, 'read')" placement="top">
              <el-button size="small" @click="openView(row)">查看</el-button>
            </el-tooltip>
            <el-tooltip :content="hint(row, 'write')" placement="top">
              <el-button size="small" :type="btnType(row, 'write')" @click="openEdit(row)">编辑</el-button>
            </el-tooltip>
            <el-tooltip :content="hint(row, 'delete')" placement="top">
              <el-button size="small" :type="btnType(row, 'delete')" plain @click="remove(row)">删除</el-button>
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 查看 / 编辑 -->
    <el-dialog v-model="st.view.visible" :title="st.view.mode === 'edit' ? '编辑文档' : '查看文档'" width="640">
      <el-form label-width="60px">
        <el-form-item label="标题">
          <el-input v-model="st.view.form.title" :disabled="st.view.mode === 'view'" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input
            v-model="st.view.form.content"
            type="textarea"
            :rows="10"
            :disabled="st.view.mode === 'view'"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="st.view.visible = false">关闭</el-button>
        <el-button v-if="st.view.mode === 'edit'" type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 新建 -->
    <el-dialog v-model="st.create.visible" title="新建文档" width="640">
      <el-form label-width="60px">
        <el-form-item label="标题"><el-input v-model="st.create.form.title" /></el-form-item>
        <el-form-item label="内容">
          <el-input v-model="st.create.form.content" type="textarea" :rows="8" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="st.create.visible = false">取消</el-button>
        <el-button type="primary" @click="create">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.note { margin-bottom: 14px; }
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.tip { color: #909399; font-size: 12px; }
</style>
