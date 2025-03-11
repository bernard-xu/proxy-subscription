package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"proxy-subscription/api"    // 修改导入路径
	"proxy-subscription/models" // 修改导入路径
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var staticFS embed.FS

func main() {
	// 初始化数据库
	if err := models.InitDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	r := gin.Default()

	// 配置CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API路由
	apiGroup := r.Group("/api")
	{
		// 订阅相关API
		apiGroup.GET("/subscriptions", api.GetSubscriptions)
		apiGroup.POST("/subscriptions", api.AddSubscription)
		apiGroup.PUT("/subscriptions/:id", api.UpdateSubscription)
		apiGroup.DELETE("/subscriptions/:id", api.DeleteSubscription)
		apiGroup.POST("/subscriptions/:id/refresh", api.RefreshSubscription)

		// 代理节点相关API
		apiGroup.GET("/proxies", api.GetProxies)
		apiGroup.GET("/proxies/:id", api.GetProxy)

		// 合并订阅输出
		apiGroup.GET("/merged", api.GetMergedSubscription)
	}

	subFS, _ := fs.Sub(staticFS, "dist")

	// 静态文件服务
	r.StaticFS("/assets", http.FS(subFS))
	// 处理前端路由（Vue Router的history模式）
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 如果是静态资源请求，尝试直接提供文件
		if strings.HasPrefix(path, "/assets/") {
			c.FileFromFS(strings.TrimPrefix(path, "/"), http.FS(subFS))
			return
		}

		// 其他所有路由返回index.html，让Vue Router处理
		indexFile, _ := staticFS.ReadFile("dist/index.html")
		c.Data(http.StatusOK, "text/html", indexFile)
	})

	// 启动服务器
	log.Println("服务器启动在 http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
