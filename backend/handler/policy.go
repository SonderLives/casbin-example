package handler

import (
	"net/http"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
)

// 策略管理接口：运行时增删查 p / g 规则（对应 Casbin 的策略管理 API）。
// 这些接口本身也受 Casbin 保护（obj = "policies"）：只有 admin 能读写策略，
// 且只能管理自己担任 admin 的那个域 —— 多租户下权限体系也是隔离的。

// ListPolicies GET /api/policies?dom=tenant-a
// 列出某域的策略。用到两个查询 API：GetPolicy()（全部 p 规则）、
// GetGroupingPolicy()（全部 g 规则），这里按域过滤后返回。
func ListPolicies(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		dom := c.Query("dom")
		// SyncedEnforcer 的查询方法返回 (值, error) 两个返回值
		allP, _ := e.GetPolicy()
		allG, _ := e.GetGroupingPolicy()
		var pRules, gRules [][]string
		for _, r := range allP {
			if len(r) >= 4 && r[1] == dom {
				pRules = append(pRules, r)
			}
		}
		for _, r := range allG {
			if len(r) >= 3 && r[2] == dom {
				gRules = append(gRules, r)
			}
		}
		c.JSON(http.StatusOK, gin.H{"domain": dom, "p": pRules, "g": gRules})
	}
}

// policyReq 是增删策略的请求体；dom 从查询参数 ?dom= 传入，
// 这样授权中间件不用解析请求体就能拿到域。
type policyReq struct {
	Type string `json:"type"` // "p"：授权规则；"g"：角色分配
	// p 规则字段：把资源 obj 上的操作 act 授给主体 sub（用户或角色）
	Sub string `json:"sub"`
	Obj string `json:"obj"`
	Act string `json:"act"`
	// g 规则字段：把用户 user 挂到角色 role
	User string `json:"user"`
	Role string `json:"role"`
}

// AddPolicy POST /api/policies?dom=tenant-a
func AddPolicy(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		dom := c.Query("dom")
		var req policyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
			return
		}
		// 「owner」是 ABAC 占位主体（见 model.conf 匹配器分支②），是保留名，
		// 不能被当作普通策略主体或角色使用，否则等于给用户开了万能后门。
		if (req.Type == "p" && req.Sub == "owner") || (req.Type == "g" && req.Role == "owner") {
			c.JSON(http.StatusBadRequest, gin.H{"message": `"owner" 是 ABAC 占位主体（保留名），不能作为策略主体或角色使用`})
			return
		}
		var err error
		switch req.Type {
		case "p":
			_, err = e.AddPolicy(req.Sub, dom, req.Obj, req.Act)
		case "g":
			_, err = e.AddGroupingPolicy(req.User, req.Role, dom)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"message": `type 必须是 "p" 或 "g"`})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		// AddPolicy / AddGroupingPolicy 会自动写回适配器（CSV 文件或数据库），
		// 如果配置了 Watcher 还会自动广播给其他实例 —— 业务代码无感。
		c.JSON(http.StatusOK, gin.H{"message": "策略已添加（AddPolicy / AddGroupingPolicy）"})
	}
}

// RemovePolicy POST /api/policies/remove?dom=tenant-a
func RemovePolicy(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		dom := c.Query("dom")
		var req policyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
			return
		}
		// 占位规则 p, owner, dom, *, * 承载匹配器的属主分支，删掉属主就失去对
		// 自己资源的权限 —— 拒绝删除并给出解释。
		if req.Type == "p" && req.Sub == "owner" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "sub=owner 是 ABAC 占位规则，承载「属主放行」分支，不能删除"})
			return
		}
		var err error
		switch req.Type {
		case "p":
			_, err = e.RemovePolicy(req.Sub, dom, req.Obj, req.Act)
		case "g":
			_, err = e.RemoveGroupingPolicy(req.User, req.Role, dom)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"message": `type 必须是 "p" 或 "g"`})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "策略已移除（RemovePolicy / RemoveGroupingPolicy）"})
	}
}
