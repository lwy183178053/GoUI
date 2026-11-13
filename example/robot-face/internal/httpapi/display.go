package httpapi

import (
	"context"
	"net/http"
	"time"

	"example.com/goui-robot-face/internal/robot"
	"github.com/gin-gonic/gin"
)

func registerDisplay(router *gin.Engine, controller *robot.Controller) {
	reply := func(c *gin.Context, current robot.DisplayState) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"current": current})
	}
	router.GET("/api/display", func(c *gin.Context) {
		reply(c, controller.Display())
	})
	router.POST("/api/display", func(c *gin.Context) {
		var body struct {
			Text *string `json:"text"`
		}
		if err := readJSON(c, &body, 64*1024); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Text == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供 text，传空字符串可清空文字"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		current, err := controller.SetDisplay(ctx, *body.Text)
		if err != nil {
			replyError(c, err)
			return
		}
		reply(c, current)
	})
}
