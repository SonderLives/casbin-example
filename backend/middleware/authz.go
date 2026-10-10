package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"casbin-demo/backend/casbinx"
	"casbin-demo/backend/store"
)

// Authz 是本示例的核心：用 Casbin 做授权的 Gin 中间件。
//
// 每个受保护请求的处理流程：
//  1. 从上下文拿到用户名（JWT 中间件已完成认证）
//  2. 把「路由模板 + HTTP 方法」翻译成 Casbin 的 dom / obj / act
//  3. 对 /docs/:id 这类路由，把资源加载出来，让模型能访问其属性（Owner）→ ABAC
//  4. e.Enforce(...) 一次判定：false 就 403，true 才放行业务 handler
func Authz(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(string)

		// 域(dom)：优先取路径参数 :dom（文档路由），否则取查询参数 ?dom=（策略路由）
		dom := c.Param("dom")
		if dom == "" {
			dom = c.Query("dom")
		}

		objName, act := mapRequest(c.FullPath(), c.Request.Method)

		// 构造资源对象。默认只带资源名（Owner 为空，ABAC 不生效）；
		// 文档详情路由把真实文档取出来，带上属主属性，于是「属主放行」规则生效。
		obj := casbinx.Object{Name: objName}
		if objName == "docs" && c.Param("id") != "" {
			doc := store.GetDoc(c.Param("id"))
			if doc == nil {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "文档不存在: " + c.Param("id")})
				return
			}
			obj.Owner = doc.Owner
		}

		// ★ 核心：一次授权判定。Enforce 的实参顺序必须与 model.conf 的
		// [request_definition] r = sub, dom, obj, act 一致。
		ok, err := e.Enforce(user, dom, obj, act)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "授权判定出错: " + err.Error()})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": fmt.Sprintf("403：用户 %q 在域 %q 对资源 %q 执行 %q 没有权限（被 Casbin 拦截）",
					user, dom, objName, act),
				"request": []string{user, dom, objName, act},
			})
			return
		}
		c.Next()
	}
}

// mapRequest 把「路由模板 + HTTP 方法」翻译成 Casbin 的 obj 与 act。
// 前后端分离项目最常见的接法就是这种映射；具体映射关系按你的业务定义。
// 这里把「新增/修改」都归为 write；如果业务需要，也可以拆成 create / update。
func mapRequest(fullPath, method string) (obj, act string) {
	switch {
	case strings.HasPrefix(fullPath, "/api/domains"):
		obj = "docs"
	case strings.HasPrefix(fullPath, "/api/policies"):
		obj = "policies"
	}
	switch method {
	case http.MethodGet:
		act = "read"
	case http.MethodPost, http.MethodPut:
		act = "write"
	case http.MethodDelete:
		act = "delete"
	}
	return obj, act
}
