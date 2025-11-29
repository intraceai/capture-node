package api

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/intraceai/capture-node/internal/manifest"
	"github.com/intraceai/capture-node/pkg/models"
	"github.com/intraceai/capture-node/pkg/shared"
)

var browserVersionRegex = regexp.MustCompile(`Chrome/(\d+\.\d+\.\d+\.\d+)`)

func (s *Server) captureSession(c *gin.Context) {
	sessionID := c.Param("id")

	session, ok := s.orchestrator.GetSession(sessionID)
	if !ok {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}

	captureResp, err := s.orchestrator.Capture(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	screenshotData, err := base64.StdEncoding.DecodeString(captureResp.Screenshot)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to decode screenshot"})
		return
	}

	domData := []byte(captureResp.DOM)
	captureID := uuid.New().String()
	capturedAt := time.Now().UTC()

	browserVersion := extractBrowserVersion(captureResp.UserAgent)

	buildOutput, err := s.manifest.Build(manifest.BuildInput{
		CaptureID:      captureID,
		URL:            captureResp.FinalURL,
		FinalURL:       captureResp.FinalURL,
		CapturedAtUTC:  capturedAt,
		BrowserName:    "chromium",
		BrowserVersion: browserVersion,
		ViewportWidth:  captureResp.Viewport.Width,
		ViewportHeight: captureResp.Viewport.Height,
		ScreenshotData: screenshotData,
		DOMData:        domData,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to build manifest"})
		return
	}

	ctx := c.Request.Context()

	if err := s.storage.StoreScreenshot(ctx, captureID, screenshotData); err != nil {
		c.JSON(500, gin.H{"error": "failed to store screenshot"})
		return
	}

	if err := s.storage.StoreDOM(ctx, captureID, domData); err != nil {
		c.JSON(500, gin.H{"error": "failed to store DOM"})
		return
	}

	if err := s.storage.StoreManifest(ctx, captureID, buildOutput.Manifest); err != nil {
		c.JSON(500, gin.H{"error": "failed to store manifest"})
		return
	}

	event, err := s.emitEvent(c, captureID, captureResp.FinalURL, capturedAt, shared.Hashes{
		ManifestSHA256:   buildOutput.ManifestHash,
		ScreenshotSHA256: buildOutput.ScreenshotHash,
		DOMSHA256:        buildOutput.DOMHash,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to emit event"})
		return
	}

	if err := s.storage.StoreEvent(ctx, captureID, event); err != nil {
		c.JSON(500, gin.H{"error": "failed to store event"})
		return
	}

	viewURL := fmt.Sprintf("%s/capture.html?id=%s", s.viewerURL, captureID)

	resp := models.CaptureResponse{
		CaptureID: captureID,
		ViewURL:   viewURL,
	}

	_ = session
	c.JSON(201, resp)
}

func (s *Server) emitEvent(c *gin.Context, captureID, url string, capturedAt time.Time, hashes shared.Hashes) (*shared.CaptureEvent, error) {
	eventReq := shared.CreateEventRequest{
		CaptureID:     captureID,
		URL:           url,
		CapturedAtUTC: capturedAt,
		Hashes:        hashes,
	}

	body, err := json.Marshal(eventReq)
	if err != nil {
		return nil, err
	}

	eventLogURL := fmt.Sprintf("%s/events", s.eventLogURL)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, eventLogURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("event-log returned status %d", resp.StatusCode)
	}

	var event shared.CaptureEvent
	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		return nil, err
	}

	return &event, nil
}

func (s *Server) getCaptureMetadata(c *gin.Context) {
	captureID := c.Param("id")

	manifest, err := s.storage.GetManifest(c.Request.Context(), captureID)
	if err != nil {
		c.JSON(404, gin.H{"error": "capture not found"})
		return
	}

	event, _ := s.storage.GetEvent(c.Request.Context(), captureID)

	resp := models.CaptureMetadata{
		CaptureID:     manifest.CaptureID,
		URL:           manifest.URL,
		FinalURL:      manifest.FinalURL,
		CapturedAtUTC: manifest.CapturedAtUTC,
		Browser:       manifest.Browser,
		Viewport:      manifest.Viewport,
		Hashes: shared.Hashes{
			ScreenshotSHA256: manifest.Hashes.ScreenshotSHA256,
			DOMSHA256:        manifest.Hashes.DOMSHA256,
		},
	}

	if event != nil {
		resp.EventID = event.EventID
		resp.Hashes.ManifestSHA256 = event.Hashes.ManifestSHA256
	}

	c.JSON(200, resp)
}

func (s *Server) getScreenshot(c *gin.Context) {
	captureID := c.Param("id")

	data, err := s.storage.GetScreenshot(c.Request.Context(), captureID)
	if err != nil {
		c.JSON(404, gin.H{"error": "screenshot not found"})
		return
	}

	c.Data(200, "image/png", data)
}

func (s *Server) getDOM(c *gin.Context) {
	captureID := c.Param("id")

	data, err := s.storage.GetDOM(c.Request.Context(), captureID)
	if err != nil {
		c.JSON(404, gin.H{"error": "DOM not found"})
		return
	}

	c.Data(200, "text/html; charset=utf-8", data)
}

func (s *Server) getManifest(c *gin.Context) {
	captureID := c.Param("id")

	manifest, err := s.storage.GetManifest(c.Request.Context(), captureID)
	if err != nil {
		c.JSON(404, gin.H{"error": "manifest not found"})
		return
	}

	c.JSON(200, manifest)
}

func (s *Server) getBundle(c *gin.Context) {
	captureID := c.Param("id")
	ctx := c.Request.Context()

	screenshot, err := s.storage.GetScreenshot(ctx, captureID)
	if err != nil {
		c.JSON(404, gin.H{"error": "capture not found"})
		return
	}

	dom, _ := s.storage.GetDOM(ctx, captureID)
	manifestData, _ := s.storage.GetManifest(ctx, captureID)
	event, _ := s.storage.GetEvent(ctx, captureID)

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	addFile(zipWriter, "screenshot.png", screenshot)
	if dom != nil {
		addFile(zipWriter, "dom.html", dom)
	}
	if manifestData != nil {
		manifestJSON, _ := json.MarshalIndent(manifestData, "", "  ")
		addFile(zipWriter, "manifest.json", manifestJSON)
	}
	if event != nil {
		eventJSON, _ := json.MarshalIndent(event, "", "  ")
		addFile(zipWriter, "event.json", eventJSON)
	}

	keysResp, err := http.Get(fmt.Sprintf("%s/keys", s.eventLogURL))
	if err == nil {
		defer keysResp.Body.Close()
		if keysResp.StatusCode == 200 {
			var keys interface{}
			json.NewDecoder(keysResp.Body).Decode(&keys)
			keysJSON, _ := json.MarshalIndent(keys, "", "  ")
			addFile(zipWriter, "operator_keys.json", keysJSON)
		}
	}

	zipWriter.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=capture-%s.zip", captureID[:8]))
	c.Data(200, "application/zip", buf.Bytes())
}

func addFile(zw *zip.Writer, name string, data []byte) {
	if data == nil {
		return
	}
	w, err := zw.Create(name)
	if err != nil {
		return
	}
	w.Write(data)
}

func extractBrowserVersion(userAgent string) string {
	matches := browserVersionRegex.FindStringSubmatch(userAgent)
	if len(matches) > 1 {
		return matches[1]
	}
	if strings.Contains(userAgent, "Chrome") {
		return "unknown"
	}
	return "unknown"
}
