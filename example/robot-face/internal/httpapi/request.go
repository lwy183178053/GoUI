package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"example.com/goui-robot-face/internal/robot"
	"github.com/gin-gonic/gin"
)

// readJSON 拒绝未知字段、超大请求和多个 JSON 对象，必填字段由各接口检查。
func readJSON(c *gin.Context, value any, limit int64) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("JSON 请求无效: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("请求只能包含一个 JSON 对象，且不能超过大小限制")
	}
	return nil
}

func replyError(c *gin.Context, err error) {
	status := http.StatusServiceUnavailable
	if errors.Is(err, robot.ErrInvalidInput) {
		status = http.StatusBadRequest
	} else if errors.Is(err, context.DeadlineExceeded) {
		status = http.StatusGatewayTimeout
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
