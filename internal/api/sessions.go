package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/intraceai/capture-node/pkg/models"
	"github.com/intraceai/capture-node/pkg/shared"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) createSession(c *gin.Context) {
	session, err := s.orchestrator.CreateSession(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	streamURL := s.orchestrator.GetStreamURL(session, s.publicHost)

	resp := models.CreateSessionResponse{
		SessionID: session.SessionID,
		StreamURL: streamURL,
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

func (s *Server) startStream(c *gin.Context) {
	sessionID := c.Param("id")

	if err := s.orchestrator.StartStream(c.Request.Context(), sessionID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "streaming"})
}

func (s *Server) stopStream(c *gin.Context) {
	sessionID := c.Param("id")

	if err := s.orchestrator.StopStream(c.Request.Context(), sessionID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "stopped"})
}

func (s *Server) proxyWebSocket(c *gin.Context) {
	sessionID := c.Param("id")

	session, ok := s.orchestrator.GetSession(sessionID)
	if !ok {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}

	// Upgrade client connection
	clientConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade client connection: %v", err)
		return
	}
	defer clientConn.Close()

	// Connect to browser container WebSocket
	browserWSURL := fmt.Sprintf("ws://%s:%d/ws", session.ContainerIP, session.APIPort)
	browserConn, _, err := websocket.DefaultDialer.Dial(browserWSURL, nil)
	if err != nil {
		log.Printf("Failed to connect to browser WebSocket: %v", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to connect to browser"))
		return
	}
	defer browserConn.Close()

	// Start streaming
	if err := s.orchestrator.StartStream(c.Request.Context(), sessionID); err != nil {
		log.Printf("Failed to start stream: %v", err)
	}

	done := make(chan struct{})

	// Browser -> Client
	go func() {
		defer close(done)
		for {
			messageType, message, err := browserConn.ReadMessage()
			if err != nil {
				return
			}
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				return
			}
		}
	}()

	// Client -> Browser
	go func() {
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				browserConn.Close()
				return
			}
			if err := browserConn.WriteMessage(messageType, message); err != nil {
				return
			}
		}
	}()

	<-done

	// Stop streaming when done
	s.orchestrator.StopStream(c.Request.Context(), sessionID)
}
