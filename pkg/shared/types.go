package shared

import "time"

type Hashes struct {
	ManifestSHA256   string `json:"manifest_sha256"`
	ScreenshotSHA256 string `json:"screenshot_sha256"`
	DOMSHA256        string `json:"dom_sha256"`
}

type Browser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Manifest struct {
	CaptureID     string    `json:"capture_id"`
	URL           string    `json:"url"`
	FinalURL      string    `json:"final_url"`
	CapturedAtUTC time.Time `json:"captured_at_utc"`
	Browser       Browser   `json:"browser"`
	Viewport      Viewport  `json:"viewport"`
	Hashes        struct {
		ScreenshotSHA256 string `json:"screenshot_sha256"`
		DOMSHA256        string `json:"dom_sha256"`
	} `json:"hashes"`
	Visibility string `json:"visibility"`
}

type CaptureEvent struct {
	EventID       string    `json:"event_id"`
	PrevEventHash *string   `json:"prev_event_hash"`
	EventHash     string    `json:"event_hash"`
	CaptureID     string    `json:"capture_id"`
	URL           string    `json:"url"`
	CapturedAtUTC time.Time `json:"captured_at_utc"`
	Hashes        Hashes    `json:"hashes"`
	OperatorKeyID string    `json:"operator_key_id"`
	Signature     string    `json:"signature"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateEventRequest struct {
	CaptureID     string    `json:"capture_id"`
	URL           string    `json:"url"`
	CapturedAtUTC time.Time `json:"captured_at_utc"`
	Hashes        Hashes    `json:"hashes"`
}

type BrowserCaptureResponse struct {
	Screenshot string   `json:"screenshot"`
	DOM        string   `json:"dom"`
	FinalURL   string   `json:"final_url"`
	Viewport   Viewport `json:"viewport"`
	UserAgent  string   `json:"user_agent"`
}

type Session struct {
	SessionID     string    `json:"session_id"`
	ContainerID   string    `json:"container_id"`
	ContainerIP   string    `json:"container_ip"`
	VNCPort       int       `json:"vnc_port"`
	WebsocketPort int       `json:"websocket_port"`
	APIPort       int       `json:"api_port"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}
