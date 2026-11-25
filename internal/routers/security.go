package routers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "csrf_token"

// csrfProtection реализует double-submit cookie для всех изменяющих запросов.
func csrfProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(csrfCookieName)
		if err != nil || len(token) != 64 {
			bytes := make([]byte, 32)
			if _, err := rand.Read(bytes); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			token = hex.EncodeToString(bytes)
		}

		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(csrfCookieName, token, 12*60*60, "/", "", c.Request.TLS != nil, false)

		method := c.Request.Method
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
			submitted := c.GetHeader("X-CSRF-Token")
			if submitted == "" {
				submitted = c.PostForm("_csrf")
			}
			if submitted == "" || subtle.ConstantTimeCompare([]byte(token), []byte(submitted)) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Проверка CSRF-токена не пройдена"})
				return
			}
		}
		c.Next()
	}
}

// uploadBodyLimit ограничивает общий размер тела запроса с файлами.
func uploadBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") || c.Request.Method == http.MethodPost {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
