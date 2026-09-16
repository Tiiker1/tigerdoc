package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is populated from environment variables so the dashboard is a
// drop-in component: run it with sensible defaults, tune via env.
type Config struct {
	// Addr is the bind address, e.g. ":8080" (all interfaces) or
	// "192.168.1.5:8080" (a single LAN interface).
	Addr string

	// DockerHost is the Docker daemon endpoint, e.g. "unix:///var/run/docker.sock".
	DockerHost string

	// AllowedSubnets restricts which source IPs may talk to the dashboard.
	// It defaults to private/LAN ranges so the service can never be reached
	// from the public internet, even if it is bound to a public interface.
	AllowedSubnets []*net.IPNet

	// AllowAll disables the subnet guard entirely.
	AllowAll bool

	// StopTimeout is the number of seconds to wait after SIGTERM before
	// SIGKILL when stopping/restarting a container.
	StopTimeout time.Duration

	// ListTimeout bounds each /api/containers polling request.
	ListTimeout time.Duration
}

var defaultSubnets = []string{
	"127.0.0.0/8",    // loopback
	"10.0.0.0/8",     // private
	"172.16.0.0/12",  // private
	"192.168.0.0/16", // private
	"169.254.0.0/16", // link-local
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // IPv6 unique local
	"fe80::/10",      // IPv6 link-local
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Addr:        envOr("DASHBOARD_ADDR", ":8080"),
		DockerHost:  os.Getenv("DOCKER_HOST"),
		StopTimeout: 10 * time.Second,
		ListTimeout: 10 * time.Second,
	}

	if v := os.Getenv("ALLOW_ALL"); strings.EqualFold(v, "true") || v == "1" {
		cfg.AllowAll = true
	}

	if secs := envOr("STOP_TIMEOUT", ""); secs != "" {
		n, err := strconv.Atoi(secs)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("STOP_TIMEOUT must be a non-negative integer, got %q", secs)
		}
		cfg.StopTimeout = time.Duration(n) * time.Second
	}

	list := defaultSubnets
	if v := os.Getenv("ALLOWED_SUBNETS"); v != "" {
		list = []string{}
		seen := map[string]bool{}
		for _, part := range strings.Split(v, ",") {
			s := strings.TrimSpace(part)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			list = append(list, s)
		}
	}

	for _, s := range list {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid subnet %q: %w", s, err)
		}
		cfg.AllowedSubnets = append(cfg.AllowedSubnets, ipnet)
	}

	return cfg, nil
}

// allowed reports whether ip is permitted to reach the dashboard.
func (c *Config) allowed(ip net.IP) bool {
	if c.AllowAll {
		return true
	}
	for _, n := range c.AllowedSubnets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
