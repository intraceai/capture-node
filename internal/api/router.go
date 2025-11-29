package api

import (
	"github.com/gin-gonic/gin"
	"github.com/intraceai/capture-node/internal/manifest"
	"github.com/intraceai/capture-node/internal/orchestrator"
	"github.com/intraceai/capture-node/internal/storage"
)

type Server struct {
	router       *gin.Engine
	storage      *storage.MinIOStorage
	orchestrator *orchestrator.Orchestrator
	manifest     *manifest.Builder
	eventLogURL  string
	publicHost   string
	viewerURL    string
}

type ServerConfig struct {
	Storage      *storage.MinIOStorage
	Orchestrator *orchestrator.Orchestrator
	Manifest     *manifest.Builder
	EventLogURL  string
	PublicHost   string
	ViewerURL    string
}

func NewServer(cfg ServerConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	s := &Server{
		router:       router,
		storage:      cfg.Storage,
		orchestrator: cfg.Orchestrator,
		manifest:     cfg.Manifest,
		eventLogURL:  cfg.EventLogURL,
		publicHost:   cfg.PublicHost,
		viewerURL:    cfg.ViewerURL,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.GET("/health", s.healthCheck)

	sessions := s.router.Group("/sessions")
	{
		sessions.POST("", s.createSession)
		sessions.DELETE("/:id", s.deleteSession)
		sessions.POST("/:id/open", s.openURL)
		sessions.POST("/:id/capture", s.captureSession)
		sessions.POST("/:id/start-stream", s.startStream)
		sessions.POST("/:id/stop-stream", s.stopStream)
		sessions.GET("/:id/ws", s.proxyWebSocket)
	}

	captures := s.router.Group("/captures")
	{
		captures.GET("/:id", s.getCaptureMetadata)
		captures.GET("/:id/screenshot", s.getScreenshot)
		captures.GET("/:id/dom", s.getDOM)
		captures.GET("/:id/manifest", s.getManifest)
		captures.GET("/:id/bundle", s.getBundle)
	}
}

func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
