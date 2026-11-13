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
		c.JSON(http.StatusOK, gin.H{"current": current, "expressions": robot.Expressions(), "grid_size": robot.GridSize})
	}
	router.GET("/api/emotion", func(c *gin.Context) {
		reply(c, controller.Emotion())
	})
	router.POST("/api/emotion", func(c *gin.Context) {
		var body struct {
			Valence *int `json:"valence"`
			Arousal *int `json:"arousal"`
		}
		if err := readJSON(c, &body, 1024); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Valence == nil || body.Arousal == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valence 和 arousal 都必须提供"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		current, err := controller.SetEmotion(ctx, robot.Vector{Valence: *body.Valence, Arousal: *body.Arousal})
		if err != nil {
			replyError(c, err)
			return
		}
		reply(c, current)
	})
}
