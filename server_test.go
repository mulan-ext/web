package web_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
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

func TestMetricsCanBeExposedOnPublicListener(t *testing.T) {
	service := web.New(&web.Config{
		Host: "0.0.0.0", Port: 8080, Pprof: true, Metrics: true,
	}, &web.Info{Name: "test"}, nil).Build()
	metricsExposed := false
	for _, route := range service.Routes() {
		if strings.HasPrefix(route.Path, "/pprof") {
			t.Fatalf("pprof route exposed on public listener: %s", route.Path)
		}
		if route.Path == "/metrics" {
			metricsExposed = true
		}
	}
	if !metricsExposed {
		t.Fatal("metrics route is not exposed on public listener")
	}
}

func TestServeReturnsListenError(t *testing.T) {
	service := web.New(&web.Config{
		Host: "invalid host", Port: 8080,
	}, &web.Info{Name: "test"}, nil)
	if err := service.Serve(); err == nil {
		t.Fatal("Serve() error = nil")
	}
}
