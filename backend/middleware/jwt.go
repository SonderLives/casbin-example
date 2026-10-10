// Package middleware 提供两个中间件：JWT（认证）与 Authz（Casbin 授权）。
package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(envOr("JWT_SECRET", "casbin-demo-secret-change-me"))

// Claims 只放用户名；真实项目还应放用户 ID、租户等。
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// SignToken 签发 JWT。注意：签发与校验属于「认证」，与 Casbin 无关。
// Casbin 只回答「这个用户能不能做这件事」，它不关心你是怎么登录的。
func SignToken(username string) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

// JWT 中间件：校验 Authorization: Bearer <token>，通过后把用户名放进上下文。
// 它只负责「认出你是谁」；「你能不能做」由下一个中间件 Authz 判断。
func JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token := strings.TrimPrefix(h, "Bearer ")
		if token == "" || token == h {
			abort(c, http.StatusUnauthorized, "未登录：请求头需要 Authorization: Bearer <token>")
			return
		}
		claims := &Claims{}
		t, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
			return jwtSecret, nil
		})
		if err != nil || !t.Valid {
			abort(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		c.Set("user", claims.Username)
		c.Next()
	}
}

func abort(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"message": msg})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
