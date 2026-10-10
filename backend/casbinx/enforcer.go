// Package casbinx 负责 Casbin Enforcer 的初始化（模型 + 适配器）。
package casbinx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	fileadapter "github.com/casbin/casbin/v3/persist/file-adapter"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// NewEnforcer 按环境变量 CASBIN_ADAPTER 创建 Enforcer：
//
//	file（默认）: 策略存 policy.csv —— 所见即所得，适合入门
//	sqlite      : 策略存 SQLite（data/casbin.db）—— 贴近生产；首次启动自动播种 policy.csv 里的规则
//
// 重点体会：Model（怎么判）和 Adapter（存哪里）是解耦的。
// 换存储只改这里几行，model.conf 与业务代码一行不动。
func NewEnforcer() (*casbin.SyncedEnforcer, error) {
	mode := strings.ToLower(envOr("CASBIN_ADAPTER", "file"))

	m, err := model.NewModelFromFile(envOr("CASBIN_MODEL", "model.conf"))
	if err != nil {
		return nil, fmt.Errorf("加载 model.conf 失败: %w", err)
	}

	var adapter persist.Adapter
	switch mode {
	case "file":
		adapter = fileadapter.NewAdapter(envOr("CASBIN_POLICY", "policy.csv"))
	case "sqlite":
		adapter, err = newSQLiteAdapter()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("未知的 CASBIN_ADAPTER: %s（可选 file / sqlite）", mode)
	}

	// SyncedEnforcer 是线程安全的 Enforcer，适合 Web 并发场景
	e, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}
	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("加载策略失败: %w", err)
	}

	if mode == "sqlite" {
		if err := seedIfEmpty(e); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// newSQLiteAdapter 用纯 Go 的 SQLite 驱动（无需 CGO）打开数据库，
// 再交给官方 gorm-adapter 管理 casbin_rule 表。
// 真实项目把这里的驱动换成 MySQL/PostgreSQL 即可，其余代码不变。
func newSQLiteAdapter() (persist.Adapter, error) {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join("data", "casbin.db")), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	return gormadapter.NewAdapterByDBUseTableName(db, "", "casbin_rule")
}

// 与 policy.csv 相同的种子规则。sqlite 模式首次启动时写入数据库，
// 每一条都对应一次策略管理 API 调用（AddPolicy / AddGroupingPolicy）。
var seedP = [][]string{
	// ABAC 占位规则：承载 model.conf 匹配器里的「属主放行」分支
	{"owner", "tenant-a", "*", "*"},
	{"owner", "tenant-b", "*", "*"},
	{"admin", "tenant-a", "docs", "*"},
	{"editor", "tenant-a", "docs", "read"},
	{"editor", "tenant-a", "docs", "write"},
	{"viewer", "tenant-a", "docs", "read"},
	{"admin", "tenant-a", "policies", "read"},
	{"admin", "tenant-a", "policies", "write"},
	{"admin", "tenant-b", "docs", "*"},
	{"viewer", "tenant-b", "docs", "read"},
	{"admin", "tenant-b", "policies", "read"},
	{"admin", "tenant-b", "policies", "write"},
}

var seedG = [][]string{
	{"alice", "admin", "tenant-a"},
	{"alice", "viewer", "tenant-b"},
	{"bob", "editor", "tenant-a"},
	{"charlie", "viewer", "tenant-a"},
	{"dave", "admin", "tenant-b"},
}

func seedIfEmpty(e *casbin.SyncedEnforcer) error {
	// 注意：SyncedEnforcer 的查询方法返回 (值, error) 两个返回值
	pRules, err := e.GetPolicy()
	if err != nil {
		return err
	}
	gRules, err := e.GetGroupingPolicy()
	if err != nil {
		return err
	}
	if len(pRules) > 0 || len(gRules) > 0 {
		return nil // 数据库里已有策略，说明不是第一次启动
	}
	for _, r := range seedP {
		if _, err := e.AddPolicy(r[0], r[1], r[2], r[3]); err != nil {
			return err
		}
	}
	for _, r := range seedG {
		if _, err := e.AddGroupingPolicy(r[0], r[1], r[2]); err != nil {
			return err
		}
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ============================================================
// 进阶扩展：Watcher（策略变更广播）
//
// 多实例部署时，实例 A 改了策略，实例 B 的内存策略会过期。
// 官方提供 redis-watcher 等组件，把「策略变更」广播给所有实例：
//
//	import rediswatcher "github.com/casbin/redis-watcher/v2"
//
//	w, _ := rediswatcher.NewWatcher("redis://localhost:6379",
//	    rediswatcher.WatcherOptions{OptionalChannel: "casbin-policy"})
//	e.SetWatcher(w)
//	w.SetUpdateCallback(func(string) { _ = e.LoadPolicy() })
//
// 本单机示例用不到，留作扩展练习（策略管理接口里 Add/RemovePolicy
// 会自动通知 Watcher，业务代码不用改）。
// ============================================================
