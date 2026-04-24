package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Client communicates with the daemon over Unix socket.
type Client struct {
	socketPath string
	httpClient *http.Client
	// Auth credentials.
	username string
	password string
	token    string
}

// NewClient creates a new IPC client.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: newHTTPClient(socketPath),
	}
}

// NewAuthenticatedClient creates a new IPC client with basic auth credentials.
func NewAuthenticatedClient(socketPath, username, password string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: newHTTPClient(socketPath),
		username:   username,
		password:   password,
	}
}

// NewTokenClient creates a new IPC client with a bearer token.
func NewTokenClient(socketPath, token string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: newHTTPClient(socketPath),
		token:      token,
	}
}

func newHTTPClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}
}

// applyAuth adds authentication headers to an HTTP request.
func (c *Client) applyAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

// IsAlive checks if the daemon is running and responsive.
func (c *Client) IsAlive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/api/ping", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Send sends a request to the daemon and returns the response.
func (c *Client) Send(ctx context.Context, req Request) (Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://unix/api/%s", req.Action)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

	log.Debug().Str("action", req.Action).Msg("sending IPC request")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("daemon returned HTTP %d", httpResp.StatusCode)
	}

	var resp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return resp, nil
}

// Close closes idle connections on the underlying transport.
func (c *Client) Close() {
	if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}
