# Casbin 权限控制示例（Go + Vue 3）

一个前后端分离的访问控制演示项目，用于完整展示 Casbin 的核心能力：**RBAC（基于角色的访问控制）**、**多租户 Domain 隔离**、**ABAC（基于属性的访问控制）** 以及 **运行时策略管理**。

项目以一个多租户文档管理系统为载体：用户登录后在指定租户下管理文档，所有请求均经过统一的授权中间件判定，放行与拦截结果在接口响应中可直接观测。

---

## 目录

- [设计目标](#设计目标)
- [技术栈](#技术栈)
- [系统架构](#系统架构)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [配置项](#配置项)
- [接口清单](#接口清单)
- [授权模型说明](#授权模型说明)
- [初始策略与演示数据](#初始策略与演示数据)
- [Casbin 概念与代码对应关系](#casbin-概念与代码对应关系)
- [测试与验证](#测试与验证)
- [演示边界与安全声明](#演示边界与安全声明)
- [扩展方向](#扩展方向)
- [参考资料](#参考资料)

---

## 设计目标

1. **认证与授权分离。** JWT 中间件仅负责识别调用者身份；权限判定完全交由 Casbin，二者互不耦合。
2. **授权逻辑收口于中间件。** 业务 handler 中不出现任何角色判断语句，新增接口只需扩展一条「路由 → 资源/操作」映射。
3. **模型与存储分离。** 策略后端可在 CSV 文件与数据库之间通过单个环境变量切换，`model.conf` 与业务代码无需改动。
4. **策略可运行时变更。** 通过策略管理接口增删 `p` / `g` 规则，权限即时生效，无需重启服务。
5. **前端提示与后端强制的边界可观测。** 前端按钮状态仅作体验优化，最终判定始终由后端 `Enforce` 决定。

---

## 技术栈

| 层 | 组件 | 版本 |
|---|---|---|
| 后端 | Go | 1.27+ |
| 后端 | [gin-gonic/gin](https://github.com/gin-gonic/gin) | v1.12.0 |
| 后端 | [casbin/casbin/v3](https://github.com/casbin/casbin) | v3.11.0 |
| 后端 | [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | v5.3.1 |
| 后端 | [casbin/gorm-adapter/v3](https://github.com/casbin/gorm-adapter) + [glebarez/sqlite](https://github.com/glebarez/sqlite)（纯 Go，无需 CGO） | v3.41.0 / v1.11.0 |
| 前端 | Vue | ^3.5.13 |
| 前端 | Vue Router / Element Plus / Axios | ^4.5.0 / ^2.9.3 / ^1.7.9 |
| 前端 | Vite | ^6.0.7 |

前端状态管理使用 Vue 原生 `reactive` 单例（`frontend/src/store.js`），未引入 Pinia 等额外依赖。

---

## 系统架构

```
浏览器（Vue 3 + Element Plus，:5173）
   │  HTTP + Authorization: Bearer <JWT>
   ▼
Vite 开发代理：/api → http://localhost:8080
   │
   ▼
Gin 路由
   ├── 中间件 1  JWT   —— 认证：解析 token，注入调用者身份，失败返回 401
   └── 中间件 2  Authz —— 授权：路由模板 + HTTP 方法 → (dom, obj, act)
                            e.Enforce(sub, dom, obj, act) → false 则 403
   │
   ▼
业务 handler（不含任何权限判断代码）
   │
   ├── store（内存数据：用户、文档）
   └── Casbin SyncedEnforcer
            ├── model.conf   —— 判定规则（语法）
            └── Adapter      —— 策略数据（policy.csv 或 SQLite casbin_rule 表）
```

---

## 目录结构

```
.
├── backend/
│   ├── main.go                 路由与中间件装配（newRouter 供测试复用）
│   ├── model.conf              Casbin 模型：请求/策略/角色/效果/匹配器定义
│   ├── policy.csv              初始策略数据（file 模式下同时作为运行时存储）
│   ├── main_test.go            授权行为矩阵测试
│   ├── casbinx/
│   │   ├── enforcer.go         Enforcer 初始化、适配器切换、SQLite 播种、Watcher 扩展说明
│   │   └── object.go           ABAC 资源对象（Name + Owner）
│   ├── middleware/
│   │   ├── jwt.go              认证：token 签发与校验
│   │   └── authz.go            授权：Casbin 判定与请求映射
│   ├── handler/
│   │   ├── auth.go             登录、当前用户信息与权限查询
│   │   ├── docs.go             文档 CRUD
│   │   ├── policy.go           策略查询与增删
│   │   └── playground.go       Enforce 试验接口
│   └── store/store.go          演示用内存数据层
├── frontend/
│   ├── vite.config.js          开发端口与 /api 代理
│   └── src/
│       ├── api.js              Axios 实例与拦截器
│       ├── store.js            登录态与 expectAllowed 前端预估
│       ├── router/index.js     路由与登录守卫
│       └── views/              登录、文档、策略、Playground
└── scripts/smoke.sh            手工冒烟脚本
```

---

## 快速开始

### 环境要求

- Go 1.27 及以上
- Node.js 18 及以上（建议 20+）
- 如需运行冒烟脚本：Bash 与 curl

### 1. 启动后端

默认以 CSV 文件作为策略存储：

```bash
cd backend
go mod download
go run .
# 服务监听 http://localhost:8080，接口前缀 /api
```

改用 SQLite 作为策略存储（策略写入 `backend/data/casbin.db` 的 `casbin_rule` 表，首次启动自动播种）：

```bash
cd backend
CASBIN_ADAPTER=sqlite go run .
```

### 2. 启动前端

```bash
cd frontend
npm install
npm run dev
# 访问 http://localhost:5173
```

### 3. 运行授权行为测试

```bash
cd backend
go test -v
```

### 4. 手工冒烟验证（可选）

后端启动后执行：

```bash
bash scripts/smoke.sh
```

脚本以 alice / bob / eve 三个账号发起六组请求，逐条打印 HTTP 状态码与预期结果，覆盖 RBAC、域隔离与 ABAC 三类判定。

---

## 配置项

后端通过环境变量配置，均有默认值：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | HTTP 监听端口 |
| `CASBIN_ADAPTER` | `file` | 策略存储后端，可选 `file` / `sqlite`；传入其他值时启动报错 |
| `CASBIN_MODEL` | `model.conf` | 模型文件路径 |
| `CASBIN_POLICY` | `policy.csv` | CSV 模式下的策略文件路径 |
| `JWT_SECRET` | `casbin-demo-secret-change-me` | JWT 签名密钥，**部署时必须覆盖** |

---

## 接口清单

所有接口前缀为 `/api`。「授权」列表示该接口是否经过 Casbin 判定。

| 方法 | 路径 | 认证 | 授权 | obj | act | 说明 |
|---|---|---|---|---|---|---|
| POST | `/login` | 否 | 否 | — | — | 校验用户名密码，签发 24 小时有效的 JWT，同时返回用户角色与隐式权限 |
| GET | `/me` | 是 | 否 | — | — | 当前用户的身份、各域角色与权限列表；属认证范畴，不走 Casbin |
| POST | `/playground` | 是 | 否 | — | — | 对任意 `(sub, dom, obj, act, owner)` 组合执行一次 `Enforce`，用于调试与教学 |
| GET | `/domains/:dom/docs` | 是 | 是 | `docs` | `read` | 列出指定租户的文档 |
| POST | `/domains/:dom/docs` | 是 | 是 | `docs` | `write` | 新建文档，创建者自动成为属主 |
| GET | `/domains/:dom/docs/:id` | 是 | 是 | `docs` | `read` | 读取单个文档；文档不存在时返回 404 |
| PUT | `/domains/:dom/docs/:id` | 是 | 是 | `docs` | `write` | 更新文档标题与内容 |
| DELETE | `/domains/:dom/docs/:id` | 是 | 是 | `docs` | `delete` | 删除文档 |
| GET | `/policies?dom=` | 是 | 是 | `policies` | `read` | 按域列出 `p` / `g` 规则 |
| POST | `/policies?dom=` | 是 | 是 | `policies` | `write` | 新增 `p` 规则或 `g` 规则 |
| POST | `/policies/remove?dom=` | 是 | 是 | `policies` | `write` | 移除 `p` 规则或 `g` 规则 |

**请求到 Casbin 参数的映射规则**（见 `backend/middleware/authz.go`）：

- `sub`：由 JWT 中间件注入的用户名；
- `dom`：优先取路径参数 `:dom`，为空时取查询参数 `?dom=`；
- `obj`：路径前缀 `/api/domains` 映射为 `docs`，`/api/policies` 映射为 `policies`；
- `act`：`GET` → `read`，`POST` / `PUT` → `write`，`DELETE` → `delete`。

**拒绝响应示例**（403）：

```json
{
  "message": "403：用户 \"alice\" 在域 \"tenant-b\" 对资源 \"docs\" 执行 \"write\" 没有权限（被 Casbin 拦截）",
  "request": ["alice", "tenant-b", "docs", "write"]
}
```

`request` 字段回显本次参与判定的四元组，便于对照模型逐参数排查。

---

## 授权模型说明

### 模型定义

`backend/model.conf` 采用「带域的多角色 RBAC + ABAC 属主扩展」：

```ini
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = ((r.sub == p.sub || g(r.sub, p.sub, r.dom)) && r.dom == p.dom && (r.obj.Name == p.obj || p.obj == "*") && (r.act == p.act || p.act == "*")) || (p.sub == "owner" && r.dom == p.dom && r.obj.Owner == r.sub)
```

各段含义：

| 段 | 作用 |
|---|---|
| `[request_definition]` | 声明一次鉴权请求的参数结构：谁（sub）、在哪个租户（dom）、对什么资源（obj）、做什么操作（act） |
| `[policy_definition]` | 声明每条 `p` 规则的字段结构，与策略文件中的列一一对应 |
| `[role_definition]` | 三元 `g` 表示「用户在指定域内拥有指定角色」，即带域的 RBAC |
| `[policy_effect]` | `some(where (p.eft == allow))` 为 allow-override 语义：任意一条规则命中即放行 |
| `[matchers]` | 请求与规则的匹配条件，包含下述两个分支 |

匹配器分为两个分支：

- **分支 ①（RBAC）**：主体直接相等，或经 `g` 规则展开角色继承后相等；同时要求域一致、资源名一致或策略为 `*`、操作一致或策略为 `*`。
- **分支 ②（ABAC）**：当规则主体为保留名 `owner`、域一致，且资源属主等于请求者（`r.obj.Owner == r.sub`）时放行。该分支不依赖任何角色，仅依赖资源属性。

### ABAC 为什么写在匹配器而不是策略效果中

Casbin 的 `[policy_effect]` 仅支持固定模板（allow-override、deny-override、priority 等），不接受任意布尔表达式；将其写为 `... || r.obj.Owner == r.sub` 会返回 `unsupported effect`。因此属性判定必须放在 `[matchers]` 中，这也是官方 ABAC 示例的标准实现方式。

由于匹配器分支必须与至少一条策略行共同成立才能命中，`policy.csv` 中保留两条占位规则：

```csv
p, owner, tenant-a, *, *
p, owner, tenant-b, *, *
```

`owner` 是**保留主体名**。策略管理接口会拒绝将其作为普通主体或角色使用，也拒绝删除这两条占位规则（见 `backend/handler/policy.go`），以避免因误操作导致属主权限整体失效，或被用于构造越权通道。

### 资源对象与属主属性的注入

`obj` 在常规情况下是字符串资源名，但 ABAC 分支需要访问资源属性。因此传入 `Enforce` 的是结构体（`backend/casbinx/object.go`）：

```go
type Object struct {
    Name  string // 资源名，如 docs / policies
    Owner string // 属主用户名；无具体资源实例时为空
}
```

注入策略如下：

- 命中 `/api/domains/:dom/docs/:id` 类路由时，中间件先加载该文档实例，将其 `Owner` 填入对象，ABAC 分支生效；
- 列表与创建类路由（如 `GET /domains/:dom/docs`、`POST /domains/:dom/docs`）不指向具体资源实例，`Owner` 为空，ABAC 分支必然不命中。该行为准确表达了「属主权限只针对已存在的资源实例」这一语义。

若文档实例不存在，中间件在判定之前返回 404。

### 策略规则示例

```csv
p, admin,  tenant-a, docs, *       # admin 角色在 tenant-a 对 docs 拥有全部操作
p, editor, tenant-a, docs, read    # editor 在 tenant-a 可读 docs
p, editor, tenant-a, docs, write   # 追加 write 后即可新建与编辑
g, bob,    editor,    tenant-a     # 将 bob 在 tenant-a 挂载为 editor 角色
```

要点：

- `p` 规则表达**权限**，`g` 规则表达**身份归属**；`p` 可直接授予用户，也可授予角色后再用 `g` 关联用户；
- 角色继承通过 `g` 实现，例如 `g, admin, editor, tenant-a` 表示 admin 继承 editor 的权限；
- SQLite 模式下首次启动会调用 `AddPolicy` / `AddGroupingPolicy` 将规则写入数据库（`seedIfEmpty`），该过程本身即演示了策略管理 API 的调用方式。

---

## 初始策略与演示数据

### 演示账号

密码统一为 `123456`。

| 用户名 | tenant-a | tenant-b | 行为特征 |
|---|---|---|---|
| alice | admin | viewer | 同一用户在不同域下权限不同；在 tenant-b 仅可读取文档 |
| bob | editor | — | 可读取与编辑，无 delete 权限；但可删除自己创建的 doc-2 |
| charlie | viewer | — | 仅可读；但其属主的 doc-3 可编辑、可删除（ABAC） |
| dave | — | admin | tenant-b 管理员，可管理 tenant-b 的策略 |
| eve | 无角色 | 无角色 | 无任何 RBAC 权限；仅可读写删自己的 doc-5（ABAC） |

### 初始文档

| ID | 域 | 属主 | 标题 |
|---|---|---|---|
| doc-1 | tenant-a | alice | Casbin 入门指南 |
| doc-2 | tenant-a | bob | RBAC 与角色继承 |
| doc-3 | tenant-a | charlie | 在 Gin 中间件里做授权 |
| doc-4 | tenant-b | dave | 多租户：domain 维度 |
| doc-5 | tenant-b | eve | eve 的私人笔记 |

角色体系为 `admin` / `editor` / `viewer`，受保护资源为 `docs` 与 `policies`，操作为 `read` / `write` / `delete`，策略中支持 `*` 通配。完整规则见 `backend/policy.csv`。

### 建议验证路径

1. 以 alice 登录，进入文档页并将租户切换至 tenant-b：可见文档减少，点击「新建文档」返回 403，响应中的 `request` 字段显示被拦截的四元组。
2. 以 charlie 登录：编辑他人文档被拦截，编辑 doc-3 放行（ABAC 属主分支）。
3. 以 eve 登录：除 doc-5 外，全部操作被拦截。
4. 以 alice 进入策略管理页，为 eve 添加 `g` 规则（viewer 角色），回到文档页后即可读取；移除该规则后读取立即失败。全过程无需重启服务。
5. 使用 Playground 页面对任意四元组执行 `Enforce`，对比填写与留空 `owner` 时的判定差异。

---

## Casbin 概念与代码对应关系

| Casbin 概念 | 代码位置 | 说明 |
|---|---|---|
| Model（模型） | `backend/model.conf` | 定义请求与策略结构、匹配规则、命中效果 |
| Policy（策略） | `backend/policy.csv` / `backend/data/casbin.db` | 授权数据，即 `p` 与 `g` 规则集合 |
| Adapter（适配器） | `backend/casbinx/enforcer.go` | 策略持久化层，`file` 与 `sqlite` 两种实现 |
| Enforcer（执行器） | `backend/casbinx/enforcer.go` | `casbin.NewSyncedEnforcer(model, adapter)`，提供线程安全的 `Enforce` |
| Request（请求） | `backend/middleware/authz.go` | 由 HTTP 上下文构造 `(sub, dom, obj, act)` |
| 认证（Authentication） | `backend/middleware/jwt.go` | 与 Casbin 无关，仅解析身份 |
| 授权（Authorization） | `backend/middleware/authz.go` | 统一判定与拦截，业务层无权限代码 |
| 域（Domain） | 路径参数 `:dom` / 查询参数 `?dom=` | 多租户隔离维度 |
| ABAC 属性 | `backend/casbinx/object.go` | 资源以结构体传入，匹配器可访问其属性 |
| 策略管理 API | `backend/handler/policy.go` | `AddPolicy` / `RemovePolicy` / `AddGroupingPolicy` / `RemoveGroupingPolicy` |
| 权限查询 API | `backend/handler/auth.go` | `GetRolesForUserInDomain` / `GetImplicitPermissionsForUser` |
| 前端预估逻辑 | `frontend/src/store.js` | `expectAllowed` 基于 `/api/me` 结果估算按钮状态，仅作提示 |
| 策略变更广播（进阶） | `backend/casbinx/enforcer.go` 末尾注释 | 多实例部署时接入 `redis-watcher` 的示例代码 |

---

## 测试与验证

`backend/main_test.go` 是一份可执行的授权行为规格说明。测试通过 `TestMain` 装配与生产一致的路由树，使用 `httptest` 直接驱动完整的「JWT 认证 → Casbin 授权」链路。

| 测试函数 | 覆盖范围 |
|---|---|
| `TestHTTPAuthorization` | 无 token 与错误密码返回 401；每个用户在每个域下的读取/写入/删除放行与拦截矩阵 |
| `TestOwnerCanDeleteOwnDoc` | 属主可删除自有文档；非属主的同角色用户删除失败 |
| `TestPolicyManagement` | 授予角色后立即生效、移除后立即失效；跨域管理被拦截；保留名 `owner` 防护 |
| `TestPlayground` | `/api/playground` 对 RBAC 与 ABAC 组合的判定结果 |
| `TestEnforceSemantics` | 绕过 HTTP 层，直接验证 Enforcer 的模型语义 |

```bash
cd backend && go test -v
```

---

## 演示边界与安全声明

本项目为教学示例，以下内容不可直接用于生产环境：

1. **用户凭据为明文常量。** `backend/store/store.go` 中密码为 `123456`，且数据存储在内存；生产环境须使用数据库并以 bcrypt 等算法哈希存储凭据。
2. **JWT 密钥存在内置默认值。** 部署时必须通过 `JWT_SECRET` 注入强度足够的密钥。
3. **`POST /api/playground` 仅要求登录，不受 Casbin 保护。** 该接口允许任意登录用户对 `(sub, dom, obj, act)` 组合发起判定请求，存在权限枚举风险；生产环境不应暴露，或应限制为管理员专用。
4. **`GET /api/me` 返回的权限列表包含角色展开结果。** 前端仅可将其用于界面提示，任何安全相关的判断都必须在后端通过 `Enforce` 完成。
5. **CSV 适配器在策略增删时会重写 `policy.csv`。** 文件中的注释与排版将被规范化覆盖；如需长期保留说明文字，建议改用数据库适配器。
6. **业务数据为内存存储。** 用户与文档数据在进程重启后恢复为初始状态。

---

## 实现要点

1. **授权收口。** 路由到资源的翻译逻辑集中在单个 `mapRequest` 函数中，业务 handler 不含权限判断；新增接口只需扩展一条映射关系。
2. **`Enforce` 实参顺序** 必须与 `[request_definition]` 完全一致，否则匹配器将读取错误列，判定结果整体失真。
3. **`SyncedEnforcer` 与 `Enforcer` 的 API 差异。** `SyncedEnforcer` 的多数查询方法返回 `(值, error)` 两个返回值，而 casbin/v3 中部分查询方法仅返回单个值；`middleware`、`handler` 与 `casbinx` 中的相应调用处已按此约束编写。
4. **域来源的两种约定。** 文档类路由使用路径参数 `:dom`，策略类路由使用查询参数 `?dom=`，后者使中间件无需解析请求体即可获得域信息。
5. **策略隔离。** 策略管理接口同样受 Casbin 保护（`obj = policies`），用户仅能管理自己担任 admin 的域，因此管理员身份不会跨租户生效。

---

## 扩展方向

1. **模型演进**：引入 `keyMatch2` / `regexMatch` 实现路径级通配授权；改用 `priority` 模型实现 deny-override（拒绝优先）。
2. **ABAC 属性扩展**：在 `Object` 上增加部门、密级、状态等属性，在匹配器中组合「属主或同部门」等规则。
3. **更换数据库**：将 `newSQLiteAdapter` 中的驱动替换为 MySQL / PostgreSQL，其余代码不变，可直观体现 Adapter 层的解耦价值。
4. **多实例策略一致性**：接入 `redis-watcher`（`casbinx/enforcer.go` 末尾已给出接入代码），策略变更后自动广播至所有实例。
5. **审计日志**：在授权中间件中记录判定结果与四元组，形成访问审计。

---

## 参考资料

- Casbin 官网与文档（中文）：<https://casbin.org/zh/docs/overview>
- casbin/casbin 源码：<https://github.com/casbin/casbin>
- casbin/gorm-adapter 源码：<https://github.com/casbin/gorm-adapter>
- Gin 文档（中文）：<https://gin-gonic.com/zh-cn/docs/>
- Element Plus 文档（中文）：<https://element-plus.org/zh-CN/>
