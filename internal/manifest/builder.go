package manifest

import (
	"time"

	"github.com/intraceai/capture-node/pkg/shared"
)

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

type BuildInput struct {
	CaptureID      string
	URL            string
	FinalURL       string
	CapturedAtUTC  time.Time
	BrowserName    string
	BrowserVersion string
	ViewportWidth  int
	ViewportHeight int
	ScreenshotData []byte
	DOMData        []byte
}

type BuildOutput struct {
	Manifest       *shared.Manifest
	ScreenshotHash string
	DOMHash        string
	ManifestHash   string
}

func (b *Builder) Build(input BuildInput) (*BuildOutput, error) {
	screenshotHash := shared.SHA256Hex(input.ScreenshotData)
	domHash := shared.SHA256Hex(input.DOMData)

	manifest := &shared.Manifest{
		CaptureID:     input.CaptureID,
		URL:           input.URL,
		FinalURL:      input.FinalURL,
		CapturedAtUTC: input.CapturedAtUTC,
		Browser: shared.Browser{
			Name:    input.BrowserName,
			Version: input.BrowserVersion,
		},
		Viewport: shared.Viewport{
			Width:  input.ViewportWidth,
			Height: input.ViewportHeight,
		},
		Visibility: "public",
	}
	manifest.Hashes.ScreenshotSHA256 = screenshotHash
	manifest.Hashes.DOMSHA256 = domHash

	manifestBytes, err := shared.CanonicalJSON(manifest)
	if err != nil {
		return nil, err
	}
	manifestHash := shared.SHA256Hex(manifestBytes)

	return &BuildOutput{
		Manifest:       manifest,
		ScreenshotHash: screenshotHash,
		DOMHash:        domHash,
		ManifestHash:   manifestHash,
	}, nil
}
