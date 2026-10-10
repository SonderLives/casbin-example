// 授权行为矩阵测试 —— 本示例的「可执行规格」。
//
// 它用 httptest 直接驱动整套「JWT 认证 → Casbin 授权」链路，
// 不需要先启动服务：在 backend 目录下执行 go test -v 即可。
// 通过读这些用例，你能精确知道每个用户在每个域能做什么、为什么。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"casbin-demo/backend/casbinx"
)

var (
	testRouter *gin.Engine
	enforcer   *casbin.SyncedEnforcer
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// 把仓库里的 policy.csv 复制到临时目录再加载：
	// 策略增删测试会通过文件适配器回写策略文件，不能污染仓库原件。
	tmp, err := os.MkdirTemp("", "casbin-smoke-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	raw, err := os.ReadFile("policy.csv")
	if err != nil {
		panic(err)
	}
	policyPath := filepath.Join(tmp, "policy.csv")
	if err := os.WriteFile(policyPath, raw, 0o644); err != nil {
		panic(err)
	}

	os.Setenv("CASBIN_ADAPTER", "file")
	os.Setenv("CASBIN_POLICY", policyPath)

	enforcer, err = casbinx.NewEnforcer()
	if err != nil {
		panic(err) // model.conf 语法错误、effect 不支持等都会在这里暴露
	}
	testRouter = newRouter(enforcer)

	os.Exit(m.Run())
}

// do 发起一个进程内 HTTP 请求，返回状态码与 JSON 响应体。
func do(t *testing.T, method, path, token, body string) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func mustLogin(t *testing.T, user string) string {
	t.Helper()
	code, out := do(t, "POST", "/api/login", "", fmt.Sprintf(`{"username":%q,"password":"123456"}`, user))
	if code != 200 {
		t.Fatalf("登录 %s 失败: %d %v", user, code, out)
	}
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatalf("登录响应缺少 token: %v", out)
	}
	return token
}

func wantCode(t *testing.T, desc string, got, exp int) {
	t.Helper()
	if got != exp {
		t.Errorf("✗ %-46s got %d, want %d", desc, got, exp)
	}
}

// TestHTTPAuthorization 走完整 HTTP 链路验证认证 + 授权矩阵。
func TestHTTPAuthorization(t *testing.T) {
	alice := mustLogin(t, "alice")
	bob := mustLogin(t, "bob")
	charlie := mustLogin(t, "charlie")
	dave := mustLogin(t, "dave")
	eve := mustLogin(t, "eve")

	cases := []struct {
		desc, method, path, token, body string
		code                            int
	}{
		// ---- 认证（JWT 中间件）----
		{"不带 token", "GET", "/api/domains/tenant-a/docs", "", "", 401},
		{"密码错误", "POST", "/api/login", "", `{"username":"alice","password":"bad"}`, 401},

		// ---- RBAC + domain 多租户 ----
		{"alice 读 tenant-a 文档（admin）", "GET", "/api/domains/tenant-a/docs", alice, "", 200},
		{"alice 读 tenant-b 文档（在 b 只是 viewer）", "GET", "/api/domains/tenant-b/docs", alice, "", 200},
		{"alice 在 tenant-b 建文档（无 write）", "POST", "/api/domains/tenant-b/docs", alice, `{"title":"x","content":"y"}`, 403},
		{"dave 读 tenant-b 文档（admin）", "GET", "/api/domains/tenant-b/docs", dave, "", 200},
		{"dave 读 tenant-a 文档（在 a 无角色）", "GET", "/api/domains/tenant-a/docs", dave, "", 403},
		{"eve 读 tenant-a 文档（无任何角色）", "GET", "/api/domains/tenant-a/docs", eve, "", 403},

		// ---- ABAC：属主分支（匹配器里的 r.obj.Owner == r.sub）----
		{"eve 读自己的 doc-5（无角色但是属主）", "GET", "/api/domains/tenant-b/docs/doc-5", eve, "", 200},
		{"eve 改自己的 doc-5", "PUT", "/api/domains/tenant-b/docs/doc-5", eve, `{"title":"eve 的私人笔记","content":"属主随意改"}`, 200},
		{"eve 读别人的 doc-2", "GET", "/api/domains/tenant-b/docs/doc-2", eve, "", 403},
		{"charlie(viewer) 改别人的 doc-1", "PUT", "/api/domains/tenant-a/docs/doc-1", charlie, `{"title":"t","content":"c"}`, 403},
		{"charlie(viewer) 改自己的 doc-3", "PUT", "/api/domains/tenant-a/docs/doc-3", charlie, `{"title":"t","content":"c"}`, 200},
		{"bob(editor) 改别人的 doc-1（editor 有 write）", "PUT", "/api/domains/tenant-a/docs/doc-1", bob, `{"title":"t","content":"c"}`, 200},
		{"bob(editor) 删别人的 doc-1（无 delete）", "DELETE", "/api/domains/tenant-a/docs/doc-1", bob, "", 403},
		{"eve 建文档（新建走 RBAC，尚无属主）", "POST", "/api/domains/tenant-b/docs", eve, `{"title":"x","content":"y"}`, 403},

		// ---- 策略管理（policies 资源本身也受 Casbin 保护）----
		{"bob 读策略（editor 无 policies:read）", "GET", "/api/policies?dom=tenant-a", bob, "", 403},
		{"alice 读 tenant-a 策略（admin）", "GET", "/api/policies?dom=tenant-a", alice, "", 200},
		{"alice 读 tenant-b 策略（在 b 不是 admin）", "GET", "/api/policies?dom=tenant-b", alice, "", 403},
		{"不带 dom 读策略（空域匹配不到规则）", "GET", "/api/policies", alice, "", 403},
		{"alice 跨域写 tenant-b 策略", "POST", "/api/policies?dom=tenant-b", alice, `{"type":"p","sub":"eve","obj":"docs","act":"read"}`, 403},
		{"bob 试图写 tenant-a 策略", "POST", "/api/policies?dom=tenant-a", bob, `{"type":"p","sub":"eve","obj":"docs","act":"read"}`, 403},

		// ---- 保留名 owner 的防护（占位主体不能被冒用/删除）----
		{"alice 把 owner 当策略主体（保留名）", "POST", "/api/policies?dom=tenant-a", alice, `{"type":"p","sub":"owner","obj":"docs","act":"read"}`, 400},
		{"alice 把 owner 当角色授予（保留名）", "POST", "/api/policies?dom=tenant-a", alice, `{"type":"g","user":"eve","role":"owner"}`, 400},
		{"alice 删除 owner 占位规则", "POST", "/api/policies/remove?dom=tenant-a", alice, `{"type":"p","sub":"owner","obj":"*","act":"*"}`, 400},
	}
	for _, c := range cases {
		code, _ := do(t, c.method, c.path, c.token, c.body)
		wantCode(t, c.desc, code, c.code)
	}
}

// TestOwnerCanDeleteOwnDoc 属主生命周期：editor 无 delete 权限，
// 但自己创建的文档自己就能删 —— 全靠 model.conf 匹配器里的 ABAC 属主分支。
func TestOwnerCanDeleteOwnDoc(t *testing.T) {
	bob := mustLogin(t, "bob")

	code, out := do(t, "POST", "/api/domains/tenant-a/docs", bob, `{"title":"临时文档","content":"冒烟测试"}`)
	wantCode(t, "bob 创建文档", code, 200)
	doc, _ := out["doc"].(map[string]any)
	id, _ := doc["id"].(string)
	if id == "" {
		t.Fatalf("创建响应缺少 doc.id: %v", out)
	}

	code, _ = do(t, "DELETE", "/api/domains/tenant-a/docs/"+id, bob, "")
	wantCode(t, "bob 删除自己刚创建的 "+id, code, 200)
}

// TestPolicyManagement 运行时改策略：授权立即生效，移除立即失效。
func TestPolicyManagement(t *testing.T) {
	alice := mustLogin(t, "alice")
	eve := mustLogin(t, "eve")

	read := func() int {
		code, _ := do(t, "GET", "/api/domains/tenant-a/docs", eve, "")
		return code
	}
	if code := read(); code != 403 {
		t.Fatalf("eve 初始应读不了 tenant-a 文档，got %d", code)
	}

	// 1) g 规则：给 eve 授 viewer 角色 → 立即可读
	code, _ := do(t, "POST", "/api/policies?dom=tenant-a", alice, `{"type":"g","user":"eve","role":"viewer"}`)
	wantCode(t, "alice 给 eve 加角色（admin 才能管策略）", code, 200)
	wantCode(t, "授权后 eve 可读（角色权限即时生效）", read(), 200)

	// 2) 移除角色 → 立即失效
	code, _ = do(t, "POST", "/api/policies/remove?dom=tenant-a", alice, `{"type":"g","user":"eve","role":"viewer"}`)
	wantCode(t, "alice 移除角色", code, 200)
	wantCode(t, "移除后 eve 又读不了", read(), 403)

	// 3) 直接 p 规则（不经过角色）：同样加 → 生效 → 移除 → 失效
	code, _ = do(t, "POST", "/api/policies?dom=tenant-a", alice, `{"type":"p","sub":"eve","obj":"docs","act":"read"}`)
	wantCode(t, "alice 加直接 p 规则", code, 200)
	wantCode(t, "p 规则生效，eve 可读", read(), 200)
	code, _ = do(t, "POST", "/api/policies/remove?dom=tenant-a", alice, `{"type":"p","sub":"eve","obj":"docs","act":"read"}`)
	wantCode(t, "alice 移除 p 规则", code, 200)
	wantCode(t, "p 规则移除后 eve 又读不了", read(), 403)
}

// TestPlayground 调试接口：任意登录用户可以对任意组合跑 Enforce。
func TestPlayground(t *testing.T) {
	eve := mustLogin(t, "eve")

	code, out := do(t, "POST", "/api/playground", eve,
		`{"sub":"bob","dom":"tenant-a","obj":"docs","act":"delete","owner":"bob"}`)
	wantCode(t, "playground 请求", code, 200)
	if out["allowed"] != true {
		t.Errorf("✗ bob 删「属主是 bob」的文档应 allowed=true，got %v", out["allowed"])
	}

	code, out = do(t, "POST", "/api/playground", eve,
		`{"sub":"bob","dom":"tenant-a","obj":"docs","act":"delete"}`)
	wantCode(t, "playground 请求", code, 200)
	if out["allowed"] != false {
		t.Errorf("✗ bob 删无属主信息的文档应 allowed=false，got %v", out["allowed"])
	}
}

// TestEnforceSemantics 绕过 HTTP，直接测 Enforcer 的判定语义 —— 最纯粹的 Casbin。
func TestEnforceSemantics(t *testing.T) {
	cases := []struct {
		desc     string
		sub, dom string
		obj      casbinx.Object
		act      string
		want     bool
	}{
		{"alice(admin) 在 tenant-a 删文档", "alice", "tenant-a", casbinx.Object{Name: "docs"}, "delete", true},
		{"bob(editor) 在 tenant-a 删文档", "bob", "tenant-a", casbinx.Object{Name: "docs"}, "delete", false},
		{"bob 删「属主是 bob」的文档（ABAC）", "bob", "tenant-a", casbinx.Object{Name: "docs", Owner: "bob"}, "delete", true},
		{"bob 删「属主是 alice」的文档", "bob", "tenant-a", casbinx.Object{Name: "docs", Owner: "alice"}, "delete", false},
		{"charlie(viewer) 写「属主是 charlie」的文档", "charlie", "tenant-a", casbinx.Object{Name: "docs", Owner: "charlie"}, "write", true},
		{"charlie(viewer) 写「属主是 alice」的文档", "charlie", "tenant-a", casbinx.Object{Name: "docs", Owner: "alice"}, "write", false},
		{"eve 读 tenant-b 文档", "eve", "tenant-b", casbinx.Object{Name: "docs"}, "read", false},
		{"eve 读「属主是 eve」的笔记（ABAC）", "eve", "tenant-b", casbinx.Object{Name: "docs", Owner: "eve"}, "read", true},
		{"跨域：dave 读 tenant-a 文档", "dave", "tenant-a", casbinx.Object{Name: "docs"}, "read", false},
		{"alice 写 tenant-b 策略（在 b 只是 viewer）", "alice", "tenant-b", casbinx.Object{Name: "policies"}, "write", false},
		{"dave 写 tenant-b 策略（admin）", "dave", "tenant-b", casbinx.Object{Name: "policies"}, "write", true},
		{"通配 act：alice(admin) 对 tenant-a docs 任意操作", "alice", "tenant-a", casbinx.Object{Name: "docs"}, "anything", true},
		{"obj 不在授权范围：alice 读 tenant-a「其它资源」", "alice", "tenant-a", casbinx.Object{Name: "anything"}, "read", false},
	}
	for _, c := range cases {
		got, err := enforcer.Enforce(c.sub, c.dom, c.obj, c.act)
		if err != nil {
			t.Errorf("✗ %s: Enforce 出错: %v", c.desc, err)
			continue
		}
		if got != c.want {
			t.Errorf("✗ %-44s got %v, want %v", c.desc, got, c.want)
		}
	}
}
