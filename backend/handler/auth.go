// Package handler：HTTP 接口层。
// 能执行到 handler 的请求都已经通过 Casbin 中间件，所以这里不需要
// 出现任何 if user.Role == ... 之类的判断 —— 授权已统一收口。
package handler

import (
	"net/http"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"casbin-demo/backend/middleware"
	"casbin-demo/backend/store"
)

// Login 登录：只做认证（校验用户名密码 + 签发 JWT），不做任何授权。
// 「登录成功」≠「有权限」——权限由每个请求经过的 Casbin 中间件判断。
func Login(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
			return
		}
		if !store.CheckPassword(req.Username, req.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户名或密码错误（演示账号密码都是 123456）"})
			return
		}
		token, err := middleware.SignToken(req.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "签发 token 失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "profile": profile(e, req.Username)})
	}
}

// Me 读取当前登录用户自己的信息与权限。
// 「看自己的资料」属于认证范畴，不走 Casbin（对比：docs / policies 都要走）。
func Me(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(string)
		c.JSON(http.StatusOK, gin.H{"profile": profile(e, user)})
	}
}

// profile 汇总某用户的角色与隐式权限（含角色展开）。
// 用到两个常用的 Casbin 查询 API：
//   - GetRolesForUserInDomain(user, dom)：用户在某域里的角色
//   - GetImplicitPermissionsForUser(user, dom)：用户在某域里的全部权限（含通过角色继承的）
//
// 前端拿它「提示」按钮是否可点——记住：这只是提示，最终判定永远在后端 Enforce。
func profile(e *casbin.SyncedEnforcer, user string) gin.H {
	roles := []gin.H{}
	perms := []gin.H{}
	for _, dom := range store.Domains {
		// casbin/v3 里这类查询只返回一个值（不再带 error）
		if rs := e.GetRolesForUserInDomain(user, dom); len(rs) > 0 {
			roles = append(roles, gin.H{"domain": dom, "roles": rs})
		}
		if ps, err := e.GetImplicitPermissionsForUser(user, dom); err == nil {
			perms = append(perms, gin.H{"domain": dom, "permissions": ps})
		}
	}
	return gin.H{"username": user, "roles": roles, "permissions": perms}
}
