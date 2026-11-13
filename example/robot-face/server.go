package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// 调试页随程序编译，运行时只需额外提供 ui 资源目录。
//
//go:embed web/*
var webFiles embed.FS

func newRouter(robot *robot, uiDirectory string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	assets := map[string]struct{ file, contentType string }{
		"/":           {"web/index.html", "text/html; charset=utf-8"},
		"/styles.css": {"web/styles.css", "text/css; charset=utf-8"},
		"/app.js":     {"web/app.js", "text/javascript; charset=utf-8"},
	}
	for route, asset := range assets {
		data, err := webFiles.ReadFile(asset.file)
		if err != nil {
			panic(err) // 内嵌文件缺失属于程序错误，在启动阶段立即暴露。
		}
		router.GET(route, func(c *gin.Context) {
			c.Data(http.StatusOK, asset.contentType, data)
		})
	}

	// 仅开放登记过的 GIF，字体、PDF 和其它项目文件不通过 HTTP 暴露。
	files := make(map[string]bool, len(expressions))
	for _, item := range expressions {
		files[item.File] = true
	}
	router.GET("/gif/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !files[name] {
			c.JSON(http.StatusNotFound, gin.H{"error": "GIF 不存在"})
			return
		}
		c.File(filepath.Join(uiDirectory, "gif", name))
	})

	reply := func(c *gin.Context, current emotionState) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"current": current, "expressions": expressions})
	}
	router.GET("/api/emotion", func(c *gin.Context) {
		reply(c, robot.state())
	})
	router.POST("/api/emotion", func(c *gin.Context) {
		input, err := readVector(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		current, err := robot.set(ctx, input)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		reply(c, current)
	})
	return router
}

func readVector(c *gin.Context) (vector, error) {
	// 指针用于区分合法的 0 和遗漏字段；整数类型同时拒绝小数和字符串。
	var body struct {
		Valence *int `json:"valence"`
		Arousal *int `json:"arousal"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return vector{}, errors.New("请求必须是包含整数 valence 和 arousal 的 JSON 对象")
	}
	if body.Valence == nil || body.Arousal == nil {
		return vector{}, errors.New("valence 和 arousal 都必须提供")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return vector{}, errors.New("请求只能包含一个 JSON 对象")
	}
	input := vector{Valence: *body.Valence, Arousal: *body.Arousal}
	return input, input.validate()
}
