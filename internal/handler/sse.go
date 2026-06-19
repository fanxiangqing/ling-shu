package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func writeSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(http.StatusOK)
}

func writeStreamEvent(c *gin.Context, name string, data any) error {
	c.SSEvent(name, data)
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeSSEvent(c *gin.Context, name string, data any) error {
	return writeStreamEvent(c, name, data)
}
