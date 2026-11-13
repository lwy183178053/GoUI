package httpapi

import (
	"context"
	"net/http"
	"time"

	"example.com/goui-robot-face/internal/robot"
	"github.com/gin-gonic/gin"
)

func registerEmotion(router *gin.Engine, controller *robot.Controller) {
	reply := func(c *gin.Context, current robot.EmotionState) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"current": current})
	}
	router.GET("/api/expressions", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"expressions": robot.Expressions()})
	})
	router.GET("/api/emotion", func(c *gin.Context) {
		reply(c, controller.Emotion())
	})
	router.POST("/api/emotion", func(c *gin.Context) {
		var body struct {
			Name      string `json:"name"`
			Loop      bool   `json:"loop"`
			Interrupt *bool  `json:"interrupt"`
		}
		if err := readJSON(c, &body, 1024); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		request := robot.ExpressionRequest{Name: body.Name, Loop: body.Loop, Interrupt: true}
		if body.Interrupt != nil {
			request.Interrupt = *body.Interrupt
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		current, err := controller.SetExpression(ctx, request)
		if err != nil {
			replyError(c, err)
			return
		}
		reply(c, current)
	})
}
