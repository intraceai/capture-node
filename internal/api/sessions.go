package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/intraceai/capture-node/pkg/models"
	"github.com/intraceai/capture-node/pkg/shared"
)

func (s *Server) createSession(c *gin.Context) {
	session, err := s.orchestrator.CreateSession(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	vncURL := fmt.Sprintf("ws://%s:%d/websockify", s.publicHost, session.WebsocketPort)

	resp := models.CreateSessionResponse{
		SessionID: session.SessionID,
		VNCURL:    vncURL,
		ExpiresAt: session.ExpiresAt,
	}

	c.JSON(201, resp)
}

func (s *Server) deleteSession(c *gin.Context) {
	sessionID := c.Param("id")

	if err := s.orchestrator.DestroySession(c.Request.Context(), sessionID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(204, nil)
}

func (s *Server) openURL(c *gin.Context) {
	sessionID := c.Param("id")

	var req models.OpenURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	url := shared.SanitizeURL(req.URL)
	if err := shared.ValidateURL(url); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := s.orchestrator.OpenURL(c.Request.Context(), sessionID, url); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "ok"})
}
