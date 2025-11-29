package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/intraceai/capture-node/pkg/shared"
)

const (
	sessionTimeout     = 15 * time.Minute
	cleanupInterval    = 1 * time.Minute
	browserImage       = "intraceai/remote-browser:latest"
	browserAPIPort     = 8082
	browserVNCPort     = 5900
	browserNoVNCPort   = 6080
)

type Orchestrator struct {
	docker       *client.Client
	httpClient   *http.Client
	sessions     map[string]*shared.Session
	mu           sync.RWMutex
	networkName  string
	stopChan     chan struct{}
}

func NewOrchestrator(networkName string) (*Orchestrator, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Orchestrator{
		docker:      docker,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		sessions:    make(map[string]*shared.Session),
		networkName: networkName,
		stopChan:    make(chan struct{}),
	}, nil
}

func (o *Orchestrator) Start(ctx context.Context) {
	go o.cleanupLoop(ctx)
}

func (o *Orchestrator) Stop() {
	close(o.stopChan)
}

func (o *Orchestrator) CreateSession(ctx context.Context) (*shared.Session, error) {
	sessionID := uuid.New().String()
	containerName := fmt.Sprintf("intrace-browser-%s", sessionID[:8])

	exposedPorts := nat.PortSet{
		nat.Port(fmt.Sprintf("%d/tcp", browserAPIPort)):   struct{}{},
		nat.Port(fmt.Sprintf("%d/tcp", browserNoVNCPort)): struct{}{},
	}

	config := &container.Config{
		Image: browserImage,
		Env: []string{
			fmt.Sprintf("SESSION_ID=%s", sessionID),
		},
		ExposedPorts: exposedPorts,
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
		Resources: container.Resources{
			Memory:   2 * 1024 * 1024 * 1024,
			NanoCPUs: 2 * 1000000000,
		},
	}

	var networkConfig *network.NetworkingConfig
	if o.networkName != "" {
		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				o.networkName: {},
			},
		}
	}

	resp, err := o.docker.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := o.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	inspect, err := o.docker.ContainerInspect(ctx, resp.ID)
	if err != nil {
		o.docker.ContainerStop(ctx, resp.ID, container.StopOptions{})
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var containerIP string
	if o.networkName != "" && inspect.NetworkSettings.Networks[o.networkName] != nil {
		containerIP = inspect.NetworkSettings.Networks[o.networkName].IPAddress
	} else if inspect.NetworkSettings.IPAddress != "" {
		containerIP = inspect.NetworkSettings.IPAddress
	} else {
		for _, net := range inspect.NetworkSettings.Networks {
			if net.IPAddress != "" {
				containerIP = net.IPAddress
				break
			}
		}
	}

	now := time.Now().UTC()
	session := &shared.Session{
		SessionID:     sessionID,
		ContainerID:   resp.ID,
		ContainerIP:   containerIP,
		VNCPort:       browserVNCPort,
		WebsocketPort: browserNoVNCPort,
		APIPort:       browserAPIPort,
		CreatedAt:     now,
		ExpiresAt:     now.Add(sessionTimeout),
	}

	o.mu.Lock()
	o.sessions[sessionID] = session
	o.mu.Unlock()

	if err := o.waitForReady(ctx, session); err != nil {
		o.DestroySession(ctx, sessionID)
		return nil, fmt.Errorf("browser not ready: %w", err)
	}

	return session, nil
}

func (o *Orchestrator) waitForReady(ctx context.Context, session *shared.Session) error {
	healthURL := fmt.Sprintf("http://%s:%d/health", session.ContainerIP, session.APIPort)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := o.httpClient.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for browser to be ready")
}

func (o *Orchestrator) GetSession(sessionID string) (*shared.Session, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	session, ok := o.sessions[sessionID]
	return session, ok
}

func (o *Orchestrator) DestroySession(ctx context.Context, sessionID string) error {
	o.mu.Lock()
	session, ok := o.sessions[sessionID]
	if ok {
		delete(o.sessions, sessionID)
	}
	o.mu.Unlock()

	if !ok {
		return nil
	}

	stopTimeout := 5
	err := o.docker.ContainerStop(ctx, session.ContainerID, container.StopOptions{Timeout: &stopTimeout})
	if err != nil {
		log.Printf("failed to stop container %s: %v", session.ContainerID, err)
	}

	return nil
}

func (o *Orchestrator) OpenURL(ctx context.Context, sessionID, url string) error {
	session, ok := o.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	apiURL := fmt.Sprintf("http://%s:%d/open", session.ContainerIP, session.APIPort)
	body, _ := json.Marshal(map[string]string{"url": url})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to open URL: %s", string(respBody))
	}

	return nil
}

func (o *Orchestrator) Capture(ctx context.Context, sessionID string) (*shared.BrowserCaptureResponse, error) {
	session, ok := o.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	apiURL := fmt.Sprintf("http://%s:%d/capture", session.ContainerIP, session.APIPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("capture failed: %s", string(respBody))
	}

	var result shared.BrowserCaptureResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (o *Orchestrator) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopChan:
			return
		case <-ticker.C:
			o.cleanupExpiredSessions(ctx)
		}
	}
}

func (o *Orchestrator) cleanupExpiredSessions(ctx context.Context) {
	now := time.Now().UTC()

	o.mu.RLock()
	var expired []string
	for id, session := range o.sessions {
		if now.After(session.ExpiresAt) {
			expired = append(expired, id)
		}
	}
	o.mu.RUnlock()

	for _, id := range expired {
		log.Printf("cleaning up expired session: %s", id)
		o.DestroySession(ctx, id)
	}
}

func (o *Orchestrator) GetVNCURL(session *shared.Session, publicHost string) string {
	return fmt.Sprintf("ws://%s:%d/websockify", publicHost, session.WebsocketPort)
}
