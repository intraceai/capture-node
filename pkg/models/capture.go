package models

import (
	"time"

	"github.com/intraceai/capture-node/pkg/shared"
)

type Capture struct {
	CaptureID     string          `json:"capture_id"`
	SessionID     string          `json:"session_id,omitempty"`
	URL           string          `json:"url"`
	FinalURL      string          `json:"final_url"`
	CapturedAtUTC time.Time       `json:"captured_at_utc"`
	Browser       shared.Browser  `json:"browser"`
	Viewport      shared.Viewport `json:"viewport"`
	Hashes        shared.Hashes   `json:"hashes"`
	EventID       string          `json:"event_id,omitempty"`
	ViewURL       string          `json:"view_url"`
	CreatedAt     time.Time       `json:"created_at"`
}

type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	VNCURL    string    `json:"vnc_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type OpenURLRequest struct {
	URL string `json:"url" binding:"required"`
}

type CaptureResponse struct {
	CaptureID string `json:"capture_id"`
	ViewURL   string `json:"view_url"`
}

type CaptureMetadata struct {
	CaptureID     string          `json:"capture_id"`
	URL           string          `json:"url"`
	FinalURL      string          `json:"final_url"`
	CapturedAtUTC time.Time       `json:"captured_at_utc"`
	Browser       shared.Browser  `json:"browser"`
	Viewport      shared.Viewport `json:"viewport"`
	Hashes        shared.Hashes   `json:"hashes"`
	EventID       string          `json:"event_id"`
}
