# Casbin 权限实验室（Go + Vue3 前后端分离学习示例）

一个把 **Casbin 核心概念完整跑起来** 的最小项目：多租户文档管理系统。
登录后管理文档，不同用户在不同租户里权限不同，页面里所有拦截/放行都能亲眼看到、亲手改出来。

```
浏览器 (Vue3 + Element Plus)
   │  HTTP + JWT
   ▼
Gin 中间件链：  JWT 认证（你是谁） ──►  Casbin 授权（你能不能）
   │                                   └─ e.Enforce(sub, dom, obj, act)
   ▼
业务 handler（零权限判断代码）
```

## 快速开始

```bash
# 1. 启动后端（默认策略存 policy.csv）
cd backend
go run .
# 想体验数据库适配器：
# CASBIN_ADAPTER=sqlite go run .    # 策略存 SQLite，首次启动自动播种

# 2. 启动前端（新开终端）
cd frontend
npm install
npm run dev                        # http://localhost:5173

# 3.（可选）跑授权行为矩阵测试 —— 一份「可执行的规格说明」
cd backend
go test -v
```

演示账号（密码都是 `123456`）：

| 用户 | tenant-a | tenant-b | 说明 |
|---|---|---|---|
| alice | admin | viewer | 双租户身份，体验同人在不同域权限不同 |
| bob | editor | — | 可读写文档、删不了别人的；但自己创建的可以删 |
| charlie | viewer | — | 只读；但自己的 doc-3 例外（ABAC） |
| dave | — | admin | tenant-b 的管理员 |
| eve | 无角色 | 无角色 | 什么都干不了？她拥有 doc-5，属主权限来自 ABAC |

## 建议的体验剧本

1. **登录页**点 alice 一键登录 → 文档页：能看到 tenant-a 三份文档，按钮有绿/灰提示。
2. 切到 **tenant-b**：只剩两份文档，alice 只是访客。点「新建文档」→ 被 403 拦截，
   toast 上直接显示被拦的四元组 `(alice, tenant-b, docs, write)`。
3. 用 **charlie** 登录：编辑别人的文档被拦；编辑他自己的 doc-3 却放行 —— 这就是 **ABAC**。
4. 用 **eve** 登录：几乎全灰；但她能看/改/删自己的 doc-5（属主规则），对别人的全 403。
5. **策略管理页**（alice 登录）：给 eve 挂上 viewer 角色 → 切到文档页，eve 立刻能读了。
   把角色再移除 → 立刻又读不了。**授权即时生效，无需重启。**
6. **Playground**：任选 sub/dom/obj/act，填不填 owner 各试一次，
   直观对比「纯 RBAC 判定」和「带属性的判定」。

## Casbin 核心概念 → 本项目代码地图

| 概念 | 在哪里 | 说明 |
|---|---|---|
| **Model** | `backend/model.conf` | 权限的「语法」：请求/策略长什么样、怎么匹配、命中后怎么办 |
| **Policy** | `backend/policy.csv`（file 模式）/ `data/casbin.db`（sqlite 模式） | 权限的「数据」：一行行 p / g 规则 |
| **Adapter** | `backend/casbinx/enforcer.go` | 策略存哪：CSV 或 SQLite，一行配置切换，业务代码零改动 |
| **Enforcer** | 同上，`casbin.NewSyncedEnforcer(model, adapter)` | 判定器：`Enforce(sub, dom, obj, act) → true/false` |
| **授权中间件** | `backend/middleware/authz.go` | 把「URL+HTTP 方法」翻译成 obj+act，调 Enforce，false → 403 |
| **认证中间件** | `backend/middleware/jwt.go` | 只负责认出「你是谁」，与 Casbin 无关 |
| **策略管理 API** | `backend/handler/policy.go` | `AddPolicy / RemovePolicy / AddGroupingPolicy / RemoveGroupingPolicy` |
| **权限查询 API** | `backend/handler/auth.go` | `GetRolesForUserInDomain / GetImplicitPermissionsForUser` |
| **ABAC 对象** | `backend/casbinx/object.go` | 资源用结构体传递，模型里可访问 `r.obj.Owner` 等属性 |
| **前端** | `frontend/src/` | `store.js` 的 `expectAllowed` 演示「前端提示 vs 后端强制」的关系 |

## model.conf 逐行讲解

```ini
[request_definition]
r = sub, dom, obj, act          # 一次鉴权请求的四元组：谁/在哪个域/对什么资源/做什么

[policy_definition]
p = sub, dom, obj, act          # 一条授权规则长这样（policy.csv 里 p 行的字段）

[role_definition]
g = _, _, _                     # 带域的 RBAC：g, alice, admin, tenant-a

[policy_effect]
e = some(where (p.eft == allow))   # 命中任意一条策略即放行

[matchers]
m = ((r.sub == p.sub || g(r.sub, p.sub, r.dom))   # ① 主体相等，或经角色展开后相等
     && r.dom == p.dom                             #   域相同（多租户隔离的关键）
     && (r.obj.Name == p.obj || p.obj == "*")      #   资源匹配或通配
     && (r.act == p.act || p.act == "*"))          #   操作匹配或通配
  || (p.sub == "owner" && r.dom == p.dom
      && r.obj.Owner == r.sub)                     # ② ABAC：请求者是资源属主即放行
```

**为什么 ABAC 放在 matcher 里而不是 effect 里？**
Casbin 的 `[policy_effect]` 只支持几种固定模板（`some(where ...)`、`!some(...)`、
`priority(...)` 等），不能写任意表达式（写成 `... || r.obj.Owner == r.sub` 会报
`unsupported effect`）。官方 ABAC 的标准做法就是在 **matcher** 里访问对象属性。
matcher 分支②需要一条可命中的策略行，所以 policy.csv 里有两条**占位规则**
`p, owner, tenant-a, *, *` 和 `p, owner, tenant-b, *, *`；`owner` 是保留主体名，
策略管理接口会拒绝把它当角色/主体使用，也拒绝删除这两行。

## policy.csv 的两种规则

```csv
p, admin, tenant-a, docs, *     # p 规则：admin 这个角色在 tenant-a 对 docs 可做任何操作
p, editor, tenant-a, docs, read # 粒度到单个操作；再给 write 就能新建/编辑
g, bob, editor, tenant-a        # g 规则：把 bob 挂到 editor 角色上（带域）
```

- **p 是「权限」，g 是「身份」**：p 直接对用户授也行（如 `p, eve, tenant-a, docs, read`），
  也可以对角色授再用 g 挂人。角色支持继承（`g, admin, editor, tenant-a` 即 admin 继承 editor）。
- 用 sqlite 模式时，首次启动会把这里的规则作为种子写入 `casbin_rule` 表
  （见 `casbinx/enforcer.go` 的 `seedIfEmpty`，顺便演示了策略管理 API 的调用方式）。

## 值得细看的实现要点

1. **授权收口在中间件**（`middleware/authz.go`）：
   `路由模板 + HTTP 方法 → obj + act` 的映射表就一个 `mapRequest` 函数。
   业务 handler 里**没有任何** `if user.Role == ...` —— 加新接口只是加一行映射。
2. **ABAC 需要资源属性**：`/docs/:id` 类路由在中间件里把文档加载出来，
   连同 `Owner` 一起传给 Enforce；而列表/创建路由只有 URL 信息，`Owner` 为空，
   属主分支自然不命中（这就是「eve 不能新建文档，但能管理自己的旧文档」的原因）。
3. **前端按钮只是提示**：`store.js` 里的 `expectAllowed` 依据 `/api/me` 返回的
   隐式权限列表给按钮上色。页面明确告诉你：**点灰色按钮后端会 403，点绿色按钮后端才放行**
   —— 前端做的是体验，安全线永远在后端。
4. **多租户策略隔离**：策略管理接口以 `?dom=` 声明操作的域，alice 是 tenant-a 的
   admin 也改不了 tenant-b 的策略（她只是 tenant-b 的 viewer）。
5. **Watcher（进阶）**：多实例部署时策略变更要广播。看 `casbinx/enforcer.go`
   末尾的注释块 —— 接入 `redis-watcher` 只需三行，`Add/RemovePolicy` 会自动通知。

## 测试即文档

`backend/main_test.go` 是一份可执行的授权行为矩阵，覆盖：

- 认证：无 token 401、密码错误 401；
- RBAC + domain：每个用户在每个域的放行/拦截；
- ABAC：属主对自有资源的放行、对他人资源的拦截；
- 策略管理：授角色 → 立即生效 → 移除 → 立即失效；跨域管理被拦；保留名 `owner` 防护；
- `TestEnforceSemantics`：绕过 HTTP 直接验证 Enforcer 语义（最纯粹的 Casbin 用法）。

```bash
cd backend && go test -v
```

## 常见的坑（本项目都替你踩过了）

1. **Enforce 实参顺序必须与 `[request_definition]` 完全一致**，否则匹配器拿错列，判定全乱。
2. **`SyncedEnforcer` 的查询方法返回 `(值, error)` 两个值**（v3 里部分查询只返回值），
   `Enforcer` 与 `SyncedEnforcer` 的 API 细节有差异，写代码时留意编译器提示。
3. **file adapter 会在增删策略时回写 policy.csv**：注释和排版会被重写为标准格式，
   想保留说明就写进 README，或改用数据库适配器。
4. **casbin v3 的 effect 只支持固定模板**，任意表达式请写进 matcher。
5. **多实例部署必须配 Watcher**，否则各实例内存里的策略会不一致。

## 下一步学习路线

- 换/加模型：试 `keyMatch2` 做路径通配（`/docs/*`）、`priority` 模型做拒绝优先；
- 把 ABAC 扩展成「属主 OR 部门匹配」等更复杂的属性规则；
- 接 MySQL/PostgreSQL：把 sqlite 驱动换掉即可，体会 Adapter 解耦；
- 接入 redis-watcher 体验策略广播（enforcer.go 里有现成注释代码）；
- 读官方文档 <https://casbin.org/zh/docs/overview>，对照本项目的 model 理解每一节。
