package web

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestID(c *gin.Context) string {
	return requestid.Get(c)
}

func ZapRequestID(c *gin.Context) zap.Field {
	return zap.String("requestid", requestid.Get(c))
}
