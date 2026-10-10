// Command egress-proxy is a trial's only route out of its internal Docker network: an HTTP proxy
// that tunnels CONNECT to hosts on an allowlist and refuses everything else.
//
// The allowlist file holds one host per line: an exact host, or "*.suffix" for any subdomain of
// suffix (not suffix itself). Blank lines and lines starting with "#" are ignored.
//
// Each refusal prints one stderr line "egress-denied <host>" (the egress_denied metric); each
// tunnel opened prints "egress-allowed <host>". A CONNECT to an unlisted host gets 403, any other
// method 405.
//
// Usage: egress-proxy [--listen :3128] [--allowlist /etc/flow-eval/egress-allowlist.txt]
// Exit codes: 2 when the flags or the allowlist are unusable, 1 when the server stops.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	listen := flag.String("listen", ":3128", "address to listen on")
	path := flag.String("allowlist", "/etc/flow-eval/egress-allowlist.txt", "allowlist file")
	flag.Parse()
	allow, err := loadAllowlist(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "egress-proxy: %v\n", err)
		os.Exit(2)
	}
	srv := &http.Server{
		Addr:              *listen,
		Handler:           &proxy{allow: allow, log: os.Stderr},
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "egress-proxy: listening on %s\n", *listen)
	fmt.Fprintf(os.Stderr, "egress-proxy: %v\n", srv.ListenAndServe())
	os.Exit(1)
}

type allowlist struct {
	exact    map[string]bool
	suffixes []string // each with its leading ".", from "*.suffix"
}

func loadAllowlist(path string) (allowlist, error) {
	f, err := os.Open(path)
	if err != nil {
		return allowlist{}, err
	}
	defer f.Close()
	a := allowlist{exact: map[string]bool{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.ToLower(strings.TrimSpace(sc.Text()))
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "*.") && strings.Contains(line[2:], ".") &&
			!strings.ContainsAny(line[2:], "*/: \t"):
			a.suffixes = append(a.suffixes, line[1:])
		case strings.ContainsAny(line, "*/: \t"):
			return allowlist{}, fmt.Errorf("%s: %q is neither a host nor *.suffix", path, line)
		default:
			a.exact[line] = true
		}
	}
	if err := sc.Err(); err != nil {
		return allowlist{}, err
	}
	if len(a.exact)+len(a.suffixes) == 0 {
		return allowlist{}, fmt.Errorf("%s: no allowlist entries", path)
	}
	return a, nil
}

func (a allowlist) allows(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if a.exact[host] {
		return true
	}
	for _, s := range a.suffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

type proxy struct {
	allow allowlist
	log   io.Writer
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	if r.Method != http.MethodConnect {
		p.deny(host)
		http.Error(w, "only CONNECT is proxied", http.StatusMethodNotAllowed)
		return
	}
	if !p.allow.allows(host) {
		p.deny(host)
		http.Error(w, "egress to "+host+" is not allowlisted", http.StatusForbidden)
		return
	}
	upstream, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		fmt.Fprintf(p.log, "egress-upstream-error %s: %v\n", host, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer upstream.Close()
	client, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		fmt.Fprintf(p.log, "egress-proxy: hijack %s: %v\n", host, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()
	fmt.Fprintf(p.log, "egress-allowed %s\n", host)
	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	// ponytail: a tunnel lives until both sides close; no idle timeout, the proxy dies with its trial.
	var wg sync.WaitGroup
	wg.Go(func() { pipe(upstream, rw.Reader) })
	wg.Go(func() { pipe(client, upstream) })
	wg.Wait()
}

func (p *proxy) deny(host string) {
	fmt.Fprintf(p.log, "egress-denied %s\n", host)
}

// pipe copies src to dst, then half-closes dst so the far side sees EOF while the other direction
// keeps flowing. A copy error only means one side went away; the tunnel ends either way.
func pipe(dst net.Conn, src io.Reader) {
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = dst.Close()
}
