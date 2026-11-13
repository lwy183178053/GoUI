// Package httpapi 提供机器人项目自己的 Gin 接口与 Web 调试面板。
package httpapi

import (
	"context"
	"embed"
	"image"
	"net/http"

	"example.com/goui-robot-face/internal/robot"
	"github.com/gin-gonic/gin"
)

// 网页随示例编译，UI 和 GIF 仍从指定目录按需读取。
//
//go:embed web/*
var webFiles embed.FS

func New(controller *robot.Controller, capture func(context.Context) (*image.RGBA, error)) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)
	assets := map[string]struct{ file, contentType string }{
		"/":           {"web/index.html", "text/html; charset=utf-8"},
		"/styles.css": {"web/styles.css", "text/css; charset=utf-8"},
		"/display.js": {"web/display.js", "text/javascript; charset=utf-8"},
		"/preview.js": {"web/preview.js", "text/javascript; charset=utf-8"},
		"/app.js":     {"web/app.js", "text/javascript; charset=utf-8"},
		"/api.js":     {"web/api.js", "text/javascript; charset=utf-8"},
	}
	for route, asset := range assets {
		data, err := webFiles.ReadFile(asset.file)
		if err != nil {
			panic(err) // 内嵌文件缺失是程序错误，启动时立即暴露。
		}
		router.GET(route, func(c *gin.Context) {
			c.Data(http.StatusOK, asset.contentType, data)
		})
	}
	registerPreview(router, capture)
	registerEmotion(router, controller)
	registerDisplay(router, controller)
	return router
}
