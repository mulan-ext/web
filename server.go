package web

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/virzz/mulan/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 默认超时配置
const (
	defaultReadTimeout    = 30 * time.Second
	defaultWriteTimeout   = 30 * time.Second
	defaultIdleTimeout    = 120 * time.Second
	defaultReadHeaderTime = 10 * time.Second
	defaultMaxHeaderBytes = 1 << 20 // 1MB
)

var _ service.Servicer = (*Service)(nil)

type Info struct {
	Name    string
	Version string
	Commit  string
	BuildAt string
}

type Service struct {
	conf     *Config
	info     *Info
	routerFn func(gin.IRouter)
	engine   *gin.Engine
	server   *http.Server
	isBuild  bool
}

func (s *Service) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Service) Close() error {
	err := s.server.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Service) Server() *http.Server    { return s.server }
func (s *Service) Routes() []gin.RouteInfo { return s.engine.Routes() }
func (s *Service) Engine() *gin.Engine     { return s.engine }

func (s *Service) Serve() error {
	if !s.isBuild {
		s.Build()
	}
	errCh := make(chan error, 1)
	go func() {
		err := s.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Second):
		zap.L().Info("HTTP Server Listening on",
			zap.String("host", s.conf.Host),
			zap.Int("port", s.conf.Port),
		)
		return nil
	}
}

var (
	loggerSkipPaths   = []string{"/health", "/version", "/metrics", "/pprof"}
	loggerSkipMethods = []string{http.MethodOptions, http.MethodHead, http.MethodTrace}
)

func New(conf *Config, info *Info, fn func(gin.IRouter)) *Service {
	return &Service{conf: conf, info: info, routerFn: fn}
}

func (s *Service) Build() *Service {
	if s.conf.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	ginLog := zap.L().Named("gin")

	s.engine = gin.New()

	s.engine.Use(
		// Recovery
		ginzap.RecoveryWithZap(ginLog, true),
		// RequestID
		requestid.New(),
		// Logger
		ginzap.GinzapWithConfig(ginLog, &ginzap.Config{
			TimeFormat: time.RFC3339,
			Skipper: func(c *gin.Context) bool {
				if slices.Contains(loggerSkipMethods, c.Request.Method) {
					return true
				}
				if c.Request.Response != nil &&
					c.Request.Response.StatusCode == 404 {
					return true
				}
				for _, path := range loggerSkipPaths {
					if strings.HasSuffix(c.Request.URL.Path, path) {
						return true
					}
				}
				return false
			},
			Context: func(c *gin.Context) []zap.Field {
				fields := []zapcore.Field{
					zap.String("referer", c.Request.Referer()),
					zap.String("requestid", requestid.Get(c)),
				}
				return fields
			},
		}),
	)

	versionHandler := VersionHandler(s.info)
	s.engine.GET("/version", versionHandler)
	s.engine.GET("/health", versionHandler)

	if s.conf.Pprof {
		pprof.RouteRegister(s.engine, "/pprof")
	}

	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	m.SetExcludePaths(loggerSkipPaths)
	m.SetSlowTime(10)
	m.Use(s.engine)

	// Register Router
	if s.routerFn != nil {
		api := s.engine.Group(s.conf.Prefix)
		s.routerFn(api)
	}
	// 配置 HTTP Server 超时防止慢速连接攻击
	s.server = &http.Server{
		Addr:              s.conf.Addr(),
		Handler:           s.engine,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		ReadHeaderTimeout: defaultReadHeaderTime,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
	s.isBuild = true
	return s
}
