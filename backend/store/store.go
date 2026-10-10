// Package store 演示用的内存数据：用户与文档。
// 真实项目里它们来自数据库；这里刻意做简单，把注意力留给 Casbin。
package store

import (
	"fmt"
	"sync"
)

// Domains 是演示用的两个租户（多租户 / domain 演示）
var Domains = []string{"tenant-a", "tenant-b"}

type User struct {
	Name     string
	Password string // 演示用明文；生产环境必须哈希存储（例如 bcrypt）
}

var users = []User{
	{"alice", "123456"},
	{"bob", "123456"},
	{"charlie", "123456"},
	{"dave", "123456"},
	{"eve", "123456"},
}

func CheckPassword(name, pwd string) bool {
	for _, u := range users {
		if u.Name == name && u.Password == pwd {
			return true
		}
	}
	return false
}

// Doc 是业务资源。Owner 字段是 ABAC 演示的关键：
// model.conf 匹配器里有「属主放行」分支（r.obj.Owner == r.sub）。
type Doc struct {
	ID      string `json:"id"`
	Domain  string `json:"domain"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Owner   string `json:"owner"`
}

var (
	mu   sync.RWMutex
	seq  = 5
	docs = []*Doc{
		{ID: "doc-1", Domain: "tenant-a", Title: "Casbin 入门指南", Owner: "alice",
			Content: "Casbin 的核心只有三样东西：模型（model.conf）、策略（policy）和执行器（Enforcer）。\n\n" +
				"模型定义「怎么判」：请求长什么样、策略长什么样、如何匹配、命中后怎么放行。\n" +
				"策略记录「谁可以」：一行行 p 规则和 g 规则。\n" +
				"Enforcer 只负责回答一个问题：Enforce(sub, dom, obj, act) => true / false。\n\n" +
				"试试用不同用户登录，观察谁能看到、谁能编辑这份文档。"},
		{ID: "doc-2", Domain: "tenant-a", Title: "RBAC 与角色继承", Owner: "bob",
			Content: "p 规则可以授权给角色，g 规则把用户挂到角色上：\n\n" +
				"  p, editor, tenant-a, docs, write\n" +
				"  g, bob, editor, tenant-a\n\n" +
				"于是 bob 在 tenant-a 里获得了 write 权限。\n" +
				"注意：bob 是这份文档的属主（Owner），即使没有任何角色，\n" +
				"model.conf 匹配器里的 ABAC 属主分支也会放行他对本文档的操作。"},
		{ID: "doc-3", Domain: "tenant-a", Title: "在 Gin 中间件里做授权", Owner: "charlie",
			Content: "前后端分离项目的推荐做法：授权统一收口到后端中间件。\n\n" +
				"1. JWT 中间件先认出「你是谁」；\n" +
				"2. Casbin 中间件再判断「你能不能」：把 HTTP 方法 + 路由模板翻译成 obj + act，\n" +
				"   然后调用 e.Enforce(...)。\n" +
				"3. 前端按钮的禁用只是体验优化，点下去被 403 拦住才是真实世界。\n\n" +
				"我是 charlie，只有 viewer 角色，但我是这份文档的属主，所以我能编辑它。"},
		{ID: "doc-4", Domain: "tenant-b", Title: "多租户：domain 维度", Owner: "dave",
			Content: "这个示例的模型是「带域的 RBAC」：请求和策略都是四元组 (sub, dom, obj, act)。\n\n" +
				"alice 在 tenant-a 是管理员，在 tenant-b 只是普通访客——\n" +
				"同一个用户在不同租户里权限完全不同，这就是 domain 的价值。\n" +
				"试试用 alice 切到 tenant-b，再试试 dave。"},
		{ID: "doc-5", Domain: "tenant-b", Title: "eve 的私人笔记", Owner: "eve",
			Content: "我（eve）没有任何角色，按 RBAC 我什么都干不了。\n\n" +
				"但我是这份笔记的属主：model.conf 匹配器里有属主分支\n" +
				"  p.sub == \"owner\" && r.dom == p.dom && r.obj.Owner == r.sub\n" +
				"（配合策略里 p, owner, tenant-b, *, * 两条占位规则），\n" +
				"所以我对自己的笔记有全部权限，而对别人的文档一律 403。\n" +
				"这就是 ABAC：不看角色，看属性。"},
	}
)

func ListDocs(dom string) []*Doc {
	mu.RLock()
	defer mu.RUnlock()
	var out []*Doc
	for _, d := range docs {
		if d.Domain == dom {
			out = append(out, d)
		}
	}
	return out
}

func GetDoc(id string) *Doc {
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range docs {
		if d.ID == id {
			return d
		}
	}
	return nil
}

func CreateDoc(dom, title, content, owner string) *Doc {
	mu.Lock()
	defer mu.Unlock()
	seq++
	d := &Doc{ID: fmt.Sprintf("doc-%d", seq), Domain: dom, Title: title, Content: content, Owner: owner}
	docs = append(docs, d)
	return d
}

func UpdateDoc(id, title, content string) (*Doc, error) {
	mu.Lock()
	defer mu.Unlock()
	for _, d := range docs {
		if d.ID == id {
			d.Title = title
			d.Content = content
			return d, nil
		}
	}
	return nil, fmt.Errorf("文档不存在")
}

func DeleteDoc(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	for i, d := range docs {
		if d.ID == id {
			docs = append(docs[:i], docs[i+1:]...)
			return true
		}
	}
	return false
}
