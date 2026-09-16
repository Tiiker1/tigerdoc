package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	logReadChunkSize = 4096
	wsMaxMessageSize = 4096
	// maxFrameSize guards against corrupt frames from a broken stream.
	maxFrameSize = 64 << 20
)

// handleLogsWS streams a container's logs to the browser over a websocket.
// Query parameters:
//
//	id          container ID or name (required)
//	tail        "all" or line count (default 500)
//	follow      "1" to keep streaming as new lines appear (default 0)
//	timestamps  "1" to prefix lines with timestamps (default 0)
//	since       RFC3339 timestamp; only show logs after this time (optional)
func (s *Server) handleLogsWS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	tail := q.Get("tail")
	if tail == "" {
		tail = "500"
	}
	follow := q.Get("follow") == "1"
	timestamps := q.Get("timestamps") == "1"
	since := q.Get("since")

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("log ws upgrade: %v", err)
		return
	}
	conn.SetReadLimit(int64(wsMaxMessageSize))
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Detect client disconnect so the docker log stream is released.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Keepalive pings so dead peers are found and proxies stay open.
	pingDone := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-t.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer close(pingDone)

	logs, err := s.docker.LogStream(streamCtx, id, LogOptions{
		Tail:       tail,
		Follow:     follow,
		Timestamps: timestamps,
		Since:      since,
	})
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
		return
	}
	defer logs.Close()

	demux := &stdDemux{src: logs, buf: &bytes.Buffer{}}
	s.streamToWS(conn, streamCtx, demux)
}

// streamToWS copies demultiplexed container output into websocket text frames.
func (s *Server) streamToWS(conn *websocket.Conn, ctx context.Context, src io.Reader) {
	buf := make([]byte, logReadChunkSize)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if werr := conn.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
				return
			}
		}
		if rerr == io.EOF {
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
		if rerr != nil {
			select {
			case <-ctx.Done():
			default:
				log.Printf("log stream read: %v", rerr)
			}
			return
		}
	}
}

// The engine multiplexes stdout and stderr into 8-byte framed chunks when both
// are requested. stdDemux unpacks those frames and yields a single unified
// byte stream, preserving arrival order.
type stdDemux struct {
	src io.Reader
	buf *bytes.Buffer
}

func (d *stdDemux) Read(p []byte) (int, error) {
	for d.buf.Len() == 0 {
		var hdr [8]byte
		if _, err := io.ReadFull(d.src, hdr[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(hdr[4:])
		if size > maxFrameSize {
			size = maxFrameSize
		}
		if _, err := io.CopyN(d.buf, d.src, int64(size)); err != nil {
			return 0, err
		}
	}
	return d.buf.Read(p)
}
