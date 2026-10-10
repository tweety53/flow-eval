package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// syncBuffer is the proxy's log sink; the handler writes from server goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// echoServer is a TLS-less TCP upstream that echoes what it reads and counts accepted connections.
func echoServer(t *testing.T) (addr string, accepted *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	accepted = new(atomic.Int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return ln.Addr().String(), accepted
}

// startProxy writes allowlist lines to a real file, loads it the way main does and serves the proxy.
func startProxy(t *testing.T, lines ...string) (proxyAddr string, log *syncBuffer) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "allowlist.txt")
	body := "# comment\n\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	allow, err := loadAllowlist(path)
	if err != nil {
		t.Fatal(err)
	}
	log = new(syncBuffer)
	srv := httptest.NewServer(&proxy{allow: allow, log: log})
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String(), log
}

// connect sends CONNECT target through the proxy and returns the connection and the response.
func connect(t *testing.T, proxyAddr, target string) (net.Conn, *bufio.Reader, *http.Response) {
	t.Helper()
	c, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	return c, br, resp
}

func TestProxyAllowsListedHost(t *testing.T) {
	upstream, _ := echoServer(t)
	proxyAddr, log := startProxy(t, "*.example.invalid", "127.0.0.1")
	c, br, resp := connect(t, proxyAddr, upstream)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT %s: status %d, want 200", upstream, resp.StatusCode)
	}
	if _, err := c.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	line, err := br.ReadString('\n')
	if err != nil || line != "ping\n" {
		t.Fatalf("tunnel echoed %q, %v; want \"ping\\n\"", line, err)
	}
	if strings.Contains(log.String(), "egress-denied") {
		t.Errorf("allowed host logged as denied: %s", log)
	}
}

func TestProxyRefusesUnlistedHost(t *testing.T) {
	upstream, accepted := echoServer(t)
	proxyAddr, log := startProxy(t, "api.anthropic.com", "*.127.0.0.1")
	_, _, resp := connect(t, proxyAddr, upstream)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("CONNECT %s: status %d, want 403", upstream, resp.StatusCode)
	}
	if got := log.String(); got != "egress-denied 127.0.0.1\n" {
		t.Errorf("log = %q, want one egress-denied line", got)
	}
	if n := accepted.Load(); n != 0 {
		t.Errorf("upstream accepted %d connections, want 0", n)
	}
}

func TestProxyRefusesPlainHTTP(t *testing.T) {
	upstream, accepted := echoServer(t)
	proxyAddr, log := startProxy(t, "127.0.0.1")
	c, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(c, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\n\r\n", upstream, upstream)
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET via proxy: status %d, want 405", resp.StatusCode)
	}
	if got := log.String(); got != "egress-denied 127.0.0.1\n" {
		t.Errorf("log = %q, want one egress-denied line", got)
	}
	if n := accepted.Load(); n != 0 {
		t.Errorf("upstream accepted %d connections, want 0", n)
	}
}

func TestShippedAllowlist(t *testing.T) {
	a, err := loadAllowlist("../../docker/egress-allowlist.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"api.anthropic.com", "statsig.anthropic.com", "api.openai.com", "chatgpt.com",
		"repo.maven.apache.org", "plugins.gradle.org", "plugins-artifacts.gradle.org", "services.gradle.org", "registry.npmjs.org", "registry-1.docker.io", "production.cloudfront.docker.com",
		"api.meta.ai", "auth.meta.com"} {
		if !a.allows(h) {
			t.Errorf("shipped allowlist refuses %s", h)
		}
	}
	for _, h := range []string{"example.com", "anthropic.com.evil.example", "github.com", "docker.io",
		"lookaside.facebook.com"} {
		if a.allows(h) {
			t.Errorf("shipped allowlist allows %s", h)
		}
	}
	for _, s := range a.suffixes {
		if strings.Count(s, ".") < 2 {
			t.Errorf("wildcard *%s spans more than one provider's domain", s)
		}
	}
}

func TestAllowlistRejectsMalformedLine(t *testing.T) {
	for _, body := range []string{"*\n", "*.\n", "*.com\n", "*.com:443\n", "api.example.com:443\n", "a/b\n", "# only a comment\n", ""} {
		path := filepath.Join(t.TempDir(), "allowlist.txt")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadAllowlist(path); err == nil {
			t.Errorf("loadAllowlist(%q) succeeded, want an error", body)
		}
	}
}
