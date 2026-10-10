// Casbin 学习示例 · 后端（Gin + casbin/v3）
//
// 运行（在 backend 目录下）：
//
//	go run .                          # 策略存 policy.csv（默认，推荐入门）
//	CASBIN_ADAPTER=sqlite go run .    # 策略存 SQLite（data/casbin.db，贴近生产）
//	PORT=9000 go run .                # 换端口
//
// 学习指引见根目录 README.md；授权行为矩阵见 main_test.go（go test -v 可直接验证）。
package main

import (
	"log"
	"os"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"casbin-demo/backend/casbinx"
	"casbin-demo/backend/handler"
	"casbin-demo/backend/middleware"
)

func main() {
	// 1. 初始化 Casbin：模型（model.conf）+ 适配器（CSV / SQLite）
	e, err := casbinx.NewEnforcer()
	if err != nil {
		log.Fatalf("初始化 Casbin 失败: %v", err)
	}

	r := newRouter(e)

	addr := ":" + envOr("PORT", "8080")
	log.Println("后端已启动: http://localhost" + addr + " （接口前缀 /api，登录 POST /api/login）")
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// newRouter 组装路由与中间件。抽成函数是为了让 main_test.go 能用
// httptest 直接驱动整套「JWT 认证 → Casbin 授权」链路做行为验证。
func newRouter(e *casbin.SyncedEnforcer) *gin.Engine {
	r := gin.Default()

	// 两层中间件，职责分明：
	//   JWT   —— 认证：认出「你是谁」
	//   Authz —— 授权：判断「你能不能」（Casbin）
	jwt := middleware.JWT()
	authz := middleware.Authz(e)

	// 登录接口：不需要认证
	r.POST("/api/login", handler.Login(e))

	api := r.Group("/api", jwt)
	{
		// 这两个接口只要求登录、不走 Casbin，原因见各 handler 的注释
		api.GET("/me", handler.Me(e))
		api.POST("/playground", handler.Playground(e))

		// 业务资源：全部经过 Casbin 授权中间件
		biz := api.Group("", authz)
		{
			biz.GET("/domains/:dom/docs", handler.ListDocs)
			biz.POST("/domains/:dom/docs", handler.CreateDoc)
			biz.GET("/domains/:dom/docs/:id", handler.GetDoc)
			biz.PUT("/domains/:dom/docs/:id", handler.UpdateDoc)
			biz.DELETE("/domains/:dom/docs/:id", handler.DeleteDoc)

			biz.GET("/policies", handler.ListPolicies(e))
			biz.POST("/policies", handler.AddPolicy(e))
			biz.POST("/policies/remove", handler.RemovePolicy(e))
		}
	}
	return r
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
