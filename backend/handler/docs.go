package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"casbin-demo/backend/store"
)

// 文档 CRUD。执行到这里意味着 Casbin 中间件已经放行，
// handler 里没有任何权限判断代码 —— 这就是「授权收口到中间件」的好处。

func ListDocs(c *gin.Context) {
	dom := c.Param("dom")
	c.JSON(http.StatusOK, gin.H{"domain": dom, "docs": store.ListDocs(dom)})
}

func GetDoc(c *gin.Context) {
	doc := store.GetDoc(c.Param("id"))
	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "文档不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"doc": doc})
}

func CreateDoc(c *gin.Context) {
	user := c.MustGet("user").(string)
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "标题不能为空"})
		return
	}
	doc := store.CreateDoc(c.Param("dom"), req.Title, req.Content, user)
	c.JSON(http.StatusOK, gin.H{"message": "创建成功，你现在是这份文档的属主（Owner）", "doc": doc})
}

func UpdateDoc(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}
	doc, err := store.UpdateDoc(c.Param("id"), req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功", "doc": doc})
}

func DeleteDoc(c *gin.Context) {
	id := c.Param("id")
	if !store.DeleteDoc(id) {
		c.JSON(http.StatusNotFound, gin.H{"message": "文档不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除 " + id})
}
