package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Docker is a lightweight HTTP client for the Docker Engine API.
// It talks directly to the daemon via its REST endpoint (Unix socket
// or TCP) and pulls in zero external dependencies beyond the standard
// library and gorilla/websocket.
type Docker struct {
	client   *http.Client
	baseURL  string // e.g. "http://localhost/containers"
	hostName string
	apiVer   string
}

// Parse a DOCKER_HOST value into scheme + host + path for HTTP.
func parseDockerHost(raw string) (scheme, host, path string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "unix:///var/run/docker.sock"
	}
	// Handle "unix:///var/run/docker.sock" → scheme=unix, host=/var/run/docker.sock
	// Handle "npipe:////./pipe/docker_engine" → scheme=npipe, host=\\.\pipe\docker_engine
	// Handle "tcp://host:2375" → scheme=tcp, host=host:2375
	if strings.HasPrefix(raw, "unix://") {
		p := strings.TrimPrefix(raw, "unix://")
		if p == "" {
			p = "/"
		}
		return "unix", p, ""
	}
	if strings.HasPrefix(raw, "npipe:") {
		p := strings.TrimPrefix(raw, "npipe:")
		// Normalize npipe path for Windows named pipes
		return "npipe", p, ""
	}
	if strings.HasPrefix(raw, "tcp://") {
		p := strings.TrimPrefix(raw, "tcp://")
		return "tcp", p, ""
	}
	// Bare "host:port" → assume tcp
	if strings.Contains(raw, ":") {
		return "tcp", raw, ""
	}
	// Fallback: treat as unix socket path
	return "unix", raw, ""
}

func dialContext(scheme, host string) func(ctx context.Context, _, _ string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		switch scheme {
		case "unix":
			var d net.Dialer
			return d.DialContext(ctx, "unix", host)
		case "tcp":
			var d net.Dialer
			return d.DialContext(ctx, "tcp", host)
		default:
			return nil, fmt.Errorf("unsupported docker host scheme: %s", scheme)
		}
	}
}

func newDocker(rawHost string) (*Docker, error) {
	scheme, host, _ := parseDockerHost(rawHost)
	transport := &http.Transport{
		DialContext:         dialContext(scheme, host),
		MaxIdleConns:        2,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
	}
	if scheme == "unix" || scheme == "npipe" {
		// Don't reuse connections over system sockets too aggressively
		transport.DisableKeepAlives = false
	}
	httpClient := &http.Client{
		Transport: transport,
		// No client-wide timeout: /logs?follow=true must stay open for as
		// long as the container runs. Individual API calls are bounded by
		// their request context instead.
	}
	d := &Docker{
		client:  httpClient,
		baseURL: "http://localhost",
	}
	// Verify connection with a ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := d.EngineInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Docker daemon at %s: %w", rawHost, err)
	}
	d.hostName = info.Hostname
	d.apiVer = info.APIVersion
	return d, nil
}

/* ---------- Engine info ---------- */

type EngineInfo struct {
	ServerVersion string `json:"serverVersion"`
	APIVersion    string `json:"apiVersion"`
	KernelVersion string `json:"kernelVersion"`
	OSType        string `json:"osType"`
	Architecture  string `json:"architecture"`
	Hostname      string `json:"hostname"`
	DaemonHost    string `json:"daemonHost"`
	Containers    int    `json:"containers"`
	Running       int    `json:"running"`
	Paused        int    `json:"paused"`
	Images        int    `json:"images"`
}

type dockerInfoResponse struct {
	ServerVersion     string `json:"ServerVersion"`
	APIVersion        string `json:"ApiVersion"`
	KernelVersion     string `json:"KernelVersion"`
	OSType            string `json:"OSType"`
	Architecture      string `json:"Architecture"`
	Name              string `json:"Name"`
	Containers        int    `json:"Containers"`
	ContainersRunning int    `json:"ContainersRunning"`
	ContainersPaused  int    `json:"ContainersPaused"`
	Images            int    `json:"Images"`
}

func (d *Docker) EngineInfo(ctx context.Context) (*EngineInfo, error) {
	var raw dockerInfoResponse
	if err := d.get(ctx, "/info", &raw); err != nil {
		return nil, err
	}
	return &EngineInfo{
		ServerVersion: raw.ServerVersion,
		APIVersion:    raw.APIVersion,
		KernelVersion: raw.KernelVersion,
		OSType:        raw.OSType,
		Architecture:  raw.Architecture,
		Hostname:      raw.Name,
		DaemonHost:    d.hostName,
		Containers:    raw.Containers,
		Running:       raw.ContainersRunning,
		Paused:        raw.ContainersPaused,
		Images:        raw.Images,
	}, nil
}

/* ---------- Containers ---------- */

type ContainerRow struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Command string            `json:"command"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Created int64             `json:"created"`
	Ports   []string          `json:"ports"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type dockerPort struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	Ports   []dockerPort      `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
}

func (d *Docker) ListContainers(ctx context.Context) ([]ContainerRow, error) {
	var raw []dockerContainer
	if err := d.get(ctx, "/containers/json?all=true", &raw); err != nil {
		return nil, err
	}
	rows := make([]ContainerRow, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		var ports []string
		for _, p := range c.Ports {
			proto := p.Type
			if proto == "" {
				proto = "tcp"
			}
			if p.PublicPort > 0 {
				ip := p.IP
				if ip == "" {
					ip = "0.0.0.0"
				}
				ports = append(ports, fmt.Sprintf("%s:%d->%d/%s", ip, p.PublicPort, p.PrivatePort, proto))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, proto))
			}
		}
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		rows = append(rows, ContainerRow{
			ID:      shortID,
			Name:    name,
			Image:   c.Image,
			Command: c.Command,
			State:   c.State,
			Status:  c.Status,
			Created: c.Created,
			Ports:   ports,
			Labels:  c.Labels,
		})
	}
	// Sort: running first, then by name
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].State != rows[j].State {
			return rows[i].State == "running"
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

/* ---------- Control ---------- */

func (d *Docker) Control(ctx context.Context, id, action string, stopTimeout time.Duration) error {
	secs := int(stopTimeout.Seconds())
	pathID := url.PathEscape(id)
	switch action {
	case "start":
		return d.post(ctx, "/containers/"+pathID+"/start", nil, nil)
	case "stop":
		return d.post(ctx, "/containers/"+pathID+"/stop?t="+fmt.Sprint(secs), nil, nil)
	case "restart":
		return d.post(ctx, "/containers/"+pathID+"/restart?t="+fmt.Sprint(secs), nil, nil)
	case "pause":
		return d.post(ctx, "/containers/"+pathID+"/pause", nil, nil)
	case "unpause":
		return d.post(ctx, "/containers/"+pathID+"/unpause", nil, nil)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

func (d *Docker) Remove(ctx context.Context, id string, force bool) error {
	path := "/containers/" + url.PathEscape(id) + "?force=" + fmt.Sprint(force)
	return d.delete(ctx, path)
}

/* ---------- Logs ---------- */

type LogOptions struct {
	Tail       string
	Follow     bool
	Timestamps bool
	Since      string
}

func (d *Docker) LogStream(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	params := url.Values{}
	params.Set("stdout", "true")
	params.Set("stderr", "true")
	tail := opts.Tail
	if tail == "" {
		tail = "500"
	}
	params.Set("tail", tail)
	if opts.Follow {
		params.Set("follow", "true")
	}
	if opts.Timestamps {
		params.Set("timestamps", "true")
	}
	if opts.Since != "" {
		params.Set("since", opts.Since)
	}
	path := "/containers/" + url.PathEscape(id) + "/logs?" + params.Encode()

	// Logs endpoint returns chunked transfer encoding — use a fresh connection.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("logs %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

/* ---------- HTTP helpers ---------- */

type apiError struct {
	Message string `json:"message"`
}

func (d *Docker) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+path, nil)
	if err != nil {
		return err
	}
	return d.do(req, out)
}

func (d *Docker) post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return d.do(req, out)
}

func (d *Docker) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, d.baseURL+path, nil)
	if err != nil {
		return err
	}
	return d.do(req, nil)
}

func (d *Docker) do(req *http.Request, out any) error {
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Message != "" {
			return fmt.Errorf("docker %s %s → %d: %s", req.Method, req.URL.Path, resp.StatusCode, apiErr.Message)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("docker %s %s → %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
