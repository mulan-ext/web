package web_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/mulan-ext/web"
)

func TestNew(t *testing.T) {
	logger, err := zap.NewDevelopment(zap.AddCaller())
	if err != nil {
		t.Fatal(err)
	}
	zap.ReplaceGlobals(logger)

	cfg := &web.Config{
		Host:  "127.0.0.1",
		Port:  3003,
		Pprof: false,
	}
	webInfo := &web.Info{
		Name:    "test",
		Version: "1.0.0",
		Commit:  "dev",
		BuildAt: time.Now().Format(time.RFC3339),
	}
	webSrv := web.New(cfg, webInfo, func(api gin.IRouter) {
		api.Handle("GET", "/aaa", func(c *gin.Context) {
			zap.L().Info("Hello, World!")
			c.String(200, "Hello, World!")
		})
	})
	// webSrv.Build()

	go func() {
		err := webSrv.Serve()
		if err != nil && err != http.ErrServerClosed {
			zap.L().Error("Failed to run http server", zap.Error(err))
		}
	}()

	r, err := http.Get("http://127.0.0.1:3003/aaa")
	if err != nil {
		t.Fatal("Failed to get /aaa", "err", err.Error())
	}
	body, err := httputil.DumpResponse(r, true)
	if err != nil {
		t.Fatal("Failed to dump response", "err", err.Error())
	}
	fmt.Println(string(body))

	<-time.After(5 * time.Second)
	webSrv.Close()
}
