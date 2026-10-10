package handler

import (
	"fmt"
	"net/http"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"casbin-demo/backend/casbinx"
)

// Playground 权限试验场：对任意 (sub, dom, obj, act, owner) 组合跑一次 Enforce。
//
// 它只要求登录、不设 Casbin 检查 —— 因为它就是「调试/教学工具」；
// 生产环境里这类接口要么不暴露，要么限制为管理员（想想为什么：任何登录用户
// 都可以借此枚举别人的权限）。
func Playground(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Sub   string `json:"sub"`
			Dom   string `json:"dom"`
			Obj   string `json:"obj"`
			Act   string `json:"act"`
			Owner string `json:"owner"` // 可选：模拟资源的属主属性（ABAC）
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
			return
		}
		ok, err := e.Enforce(req.Sub, req.Dom, casbinx.Object{Name: req.Obj, Owner: req.Owner}, req.Act)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"allowed":    ok,
			"request":    req,
			"expression": fmtEnforce(req.Sub, req.Dom, req.Obj, req.Owner, req.Act),
		})
	}
}

func fmtEnforce(sub, dom, obj, owner, act string) string {
	o := obj
	if owner != "" {
		o = fmt.Sprintf("{Name:%s, Owner:%s}", obj, owner)
	}
	return "Enforce(" + sub + ", " + dom + ", " + o + ", " + act + ")"
}
