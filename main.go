package main

import (
	"batch-connector/internal/handlers"
	"batch-connector/internal/services"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查 cookie
		authenticated, err := c.Cookie("authenticated")
		if err != nil || authenticated != "true" {
			// 如果是 API 请求，返回 JSON 错误
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
				c.Abort()
				return
			}
			// 如果是页面请求，重定向到登录页
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func main() {
	// 初始化服务
	connectorService, err := services.NewConnectorService()
	if err != nil {
		log.Fatal("初始化服务失败:", err)
	}
	handler := handlers.NewHandler(connectorService)

	// 创建 Gin 路由
	r := gin.Default()

	// 静态文件服务
	r.Static("/static", "./web/static")
	r.LoadHTMLGlob("web/templates/*")

	// 公开路由（不需要认证）
	r.GET("/login", handler.LoginPage)
	r.POST("/api/login", handler.Login)
	r.POST("/api/logout", handler.Logout)

	// 需要认证的路由
	authorized := r.Group("/")
	authorized.Use(authMiddleware())
	{
		authorized.GET("/", handler.Index)
		authorized.POST("/api/import", handler.ImportCSV)
		authorized.POST("/api/connect", handler.Connect)
		authorized.POST("/api/connect-batch", handler.ConnectBatch)
		authorized.GET("/api/connections", handler.GetConnections)
		authorized.PUT("/api/connections/:id", handler.UpdateConnection)
		authorized.DELETE("/api/connections/:id", handler.DeleteConnection)
		authorized.POST("/api/connections/delete-batch", handler.DeleteBatchConnections)
		authorized.GET("/api/settings/proxy", handler.GetProxySettings)
		authorized.PUT("/api/settings/proxy", handler.UpdateProxySettings)
	}

	// 启动服务器
	log.Println("========================================")
	log.Println("Attack_login 服务器启动")
	log.Println("公众号：知攻善防实验室")
	log.Println("开发者：ChinaRan404")
	log.Println("服务器地址: http://localhost:18921")
	log.Println("========================================")
	if err := http.ListenAndServe(":18921", r); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
