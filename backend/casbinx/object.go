package casbinx

// Object 是传给 Enforce 的资源对象。
//
// 用结构体而不是字符串，是为了演示 ABAC：匹配器和策略效果里可以访问
// 资源的属性（例如 Owner —— 资源属主）。这是 Casbin 官方推荐的 ABAC 用法。
type Object struct {
	Name  string `json:"name"`  // 资源名，例如 docs / policies
	Owner string `json:"owner"` // 属主用户名；URL 级检查（无具体资源实例）时为空
}
