package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockDaemon is a minimal Docker Engine API fake used by the integration tests.
type mockDaemon struct {
	t   *testing.T
	srv *httptest.Server
	mu  sync.Mutex
	// actions records "action:id" pairs in order of arrival.
	actions []string
}

func newMockDaemon(t *testing.T) *mockDaemon {
	m := &mockDaemon{t: t}
	c := dockerContainer{
		ID:      "abc123def456abc123def456abc123def456",
		Names:   []string{"/web"},
		Image:   "nginx:alpine",
		Command: "nginx -g daemon off;",
		Created: 1000,
		Ports: []dockerPort{
			{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
		},
		Labels: map[string]string{"com.example.name": "web"},
		State:  "running",
		Status: "Up 2 hours",
	}
	stopped := dockerContainer{
		ID:      "ffffffbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Names:   []string{"/db"},
		Image:   "postgres:16",
		Command: "postgres",
		Created: 500,
		State:   "exited",
		Status:  "Exited (0) 5 minutes ago",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, dockerInfoResponse{
			ServerVersion:     "26.1.5",
			APIVersion:        "1.45",
			KernelVersion:     "6.8.0",
			OSType:            "linux",
			Architecture:      "x86_64",
			Name:              "mock-server",
			Containers:        2,
			ContainersRunning: 1,
			Images:            4,
		})
	})
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []dockerContainer{c, stopped})
	})
	mux.HandleFunc("/containers/", func(w http.ResponseWriter, r *http.Request) {
		// Path: /containers/{id}/logs | /containers/{id}/{action} | (DELETE) /containers/{id}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		id := parts[2]

		if r.Method == http.MethodDelete {
			m.mu.Lock()
			m.actions = append(m.actions, "remove:"+id)
			m.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if len(parts) < 4 {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		verb := parts[3]
		if verb == "logs" {
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			frame := func(stream byte, s string) {
				body := []byte(s)
				hdr := make([]byte, 8)
				hdr[0] = stream
				binary.BigEndian.PutUint32(hdr[4:], uint32(len(body)))
				_, _ = w.Write(append(hdr, body...))
			}
			frame(1, "hello from stdout\n")
			frame(2, "oops on stderr\n")
			return
		}

		m.mu.Lock()
		m.actions = append(m.actions, verb+":"+id)
		m.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockDaemon) url() string { return m.srv.URL }

func (m *mockDaemon) seenActions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.actions))
	copy(out, m.actions)
	return out
}

func parseSubnets(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	var out []*net.IPNet
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatalf("ParseCIDR(%q): %v", s, err)
		}
		out = append(out, n)
	}
	return out
}

// startDashboard wires a real Docker client against the mock daemon plus a real
// HTTP server for the dashboard API/UI, all on 127.0.0.1.
func startDashboard(t *testing.T, daemon *mockDaemon, allowed []*net.IPNet) *httptest.Server {
	t.Helper()
	cfg := &Config{
		AllowAll:       false,
		StopTimeout:    10 * time.Second,
		ListTimeout:    5 * time.Second,
		AllowedSubnets: parseSubnets(t, "127.0.0.0/8", "::1/128"),
	}
	if allowed != nil {
		cfg.AllowedSubnets = allowed
	}
	d, err := newDocker("tcp://" + strings.TrimPrefix(daemon.url(), "http://"))
	if err != nil {
		t.Fatalf("newDocker: %v", err)
	}
	ts := httptest.NewServer(newServer(cfg, d).handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestListContainers(t *testing.T) {
	daemon := newMockDaemon(t)
	ts := startDashboard(t, daemon, nil)

	res, err := http.Get(ts.URL + "/api/containers")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var rows []ContainerRow
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Name != "web" || rows[0].State != "running" || rows[0].ID != "abc123def456" {
		t.Fatalf("running container should sort first: %+v", rows[0])
	}
	if rows[1].Name != "db" || rows[1].State != "exited" {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
	if len(rows[0].Ports) != 1 || rows[0].Ports[0] != "0.0.0.0:8080->80/tcp" {
		t.Fatalf("unexpected ports: %+v", rows[0].Ports)
	}
}

func TestControlActions(t *testing.T) {
	daemon := newMockDaemon(t)
	ts := startDashboard(t, daemon, nil)

	for _, action := range []string{"start", "stop", "restart", "pause", "unpause"} {
		res, err := http.Post(ts.URL+"/api/containers/abc123/"+action, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusAccepted {
			t.Fatalf("%s: status %d", action, res.StatusCode)
		}
		res.Body.Close()
	}
	got := daemon.seenActions()
	want := []string{"start:abc123", "stop:abc123", "restart:abc123", "pause:abc123", "unpause:abc123"}
	if len(got) != len(want) {
		t.Fatalf("want %d actions, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("action %d: want %q got %q", i, want[i], got[i])
		}
	}
}

func TestRemoveContainer(t *testing.T) {
	daemon := newMockDaemon(t)
	ts := startDashboard(t, daemon, nil)

	res, err := http.Post(ts.URL+"/api/containers/abc123/remove", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", res.StatusCode)
	}
	if got := daemon.seenActions(); len(got) != 1 || got[0] != "remove:abc123" {
		t.Fatalf("unexpected actions: %v", got)
	}
}

func TestLogsOverWebSocket(t *testing.T) {
	daemon := newMockDaemon(t)
	ts := startDashboard(t, daemon, nil)

	u := url.URL{
		Scheme:   "ws",
		Host:     strings.TrimPrefix(ts.URL, "http://"),
		Path:     "/ws/logs",
		RawQuery: "id=abc123def456abc123def456abc123def456&tail=all",
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var got string
	for {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break // EOF from the mock → server closes the socket
		}
		got += string(msg)
	}
	if !strings.Contains(got, "hello from stdout") || !strings.Contains(got, "oops on stderr") {
		t.Fatalf("logs missing expected output, got: %q", got)
	}
}

func TestLanGuardBlocksLoopback(t *testing.T) {
	daemon := newMockDaemon(t)
	// Allow only a different private LAN range so the 127.0.0.1 client is refused.
	ts := startDashboard(t, daemon, parseSubnets(t, "192.168.1.0/24"))

	res, err := http.Get(ts.URL + "/api/containers")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", res.StatusCode)
	}
}

func TestStdDemux(t *testing.T) {
	var raw []byte
	frame := func(stream byte, s string) {
		hdr := make([]byte, 8)
		hdr[0] = stream
		binary.BigEndian.PutUint32(hdr[4:], uint32(len(s)))
		raw = append(raw, append(hdr, s...)...)
	}
	frame(1, "one")
	frame(2, "two")
	frame(1, "three")

	d := &stdDemux{src: bytes.NewReader(raw), buf: &bytes.Buffer{}}
	out, err := io.ReadAll(d)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(out) != "onetwothree" {
		t.Fatalf("want 'onetwothree', got %q", string(out))
	}
}

func TestStaticUI(t *testing.T) {
	daemon := newMockDaemon(t)
	ts := startDashboard(t, daemon, nil)

	for _, tc := range []struct{ path, want string }{
		{"/", "Docker Dashboard"},
		{"/app.js", "loadContainers"},
		{"/style.css", "--accent"},
	} {
		res, err := http.Get(ts.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", tc.path, res.StatusCode)
		}
		if !strings.Contains(string(body), tc.want) {
			t.Fatalf("GET %s: body missing %q", tc.path, tc.want)
		}
	}
}

func TestConfigAllowed(t *testing.T) {
	cfg := &Config{AllowedSubnets: parseSubnets(t, "10.0.0.0/8", "192.168.0.0/16", "::1/128")}
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.1.2.3", true},
		{"192.168.1.5", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"172.16.0.1", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := cfg.allowed(ip); got != c.want {
			t.Errorf("allowed(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}
