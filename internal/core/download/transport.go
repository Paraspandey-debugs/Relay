package download

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Pool tuning constants (mirrors Surge's engine/types values).
const (
	poolMaxConnsPerHost     = 32
	poolMaxIdleConns        = 100
	poolMaxIdleConnsPerHost = 16
	poolIdleConnTimeout     = 90 * time.Second
	poolTLSHandshakeTimeout = 10 * time.Second
	poolResponseHeaderTO    = 30 * time.Second
	poolExpectContinueTO    = 1 * time.Second
	poolIdleEvictAfter      = 10 * time.Second
)

type poolKey struct {
	proxyURL  string
	maxConns  int
}

type transportLease struct {
	transport *http.Transport
	refs      int
	idleTimer *time.Timer
	timerGen  int
	key       poolKey
}

// TransportPool manages shared HTTP transports for TCP connection reuse across
// workers within the same download.
type TransportPool struct {
	mu           sync.Mutex
	configMap    map[poolKey]*transportLease
	transportMap map[*http.Transport]*transportLease
}

// DefaultTransportPool is the package-level singleton.
var DefaultTransportPool = newTransportPool()

func newTransportPool() *TransportPool {
	return &TransportPool{
		configMap:    make(map[poolKey]*transportLease),
		transportMap: make(map[*http.Transport]*transportLease),
	}
}

// AcquireTransport returns a shared *http.Transport for the given proxy URL
// and connection limit. Callers must call ReleaseTransport when done.
func (p *TransportPool) AcquireTransport(proxyURL string, maxConns int) *http.Transport {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := poolKey{proxyURL, maxConns}
	lease, ok := p.configMap[key]
	if !ok {
		t := p.createTransport(proxyURL, maxConns)
		lease = &transportLease{transport: t, key: key}
		p.configMap[key] = lease
		p.transportMap[t] = lease
	}

	// Cancel any pending idle-eviction timer.
	if lease.idleTimer != nil {
		lease.idleTimer.Stop()
		lease.idleTimer = nil
		lease.timerGen++
	}

	lease.refs++
	return lease.transport
}

// ReleaseTransport decrements the ref-count and schedules idle eviction when
// the transport is no longer in use.
func (p *TransportPool) ReleaseTransport(t *http.Transport) {
	if t == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	lease, ok := p.transportMap[t]
	if !ok {
		return
	}

	lease.refs--
	if lease.refs < 0 {
		lease.refs = 0
	}

	if lease.refs == 0 {
		lease.timerGen++
		gen := lease.timerGen
		lease.idleTimer = time.AfterFunc(poolIdleEvictAfter, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if lease.refs == 0 && lease.timerGen == gen {
				lease.transport.CloseIdleConnections()
				delete(p.configMap, lease.key)
				delete(p.transportMap, lease.transport)
			}
		})
	}
}

func (p *TransportPool) createTransport(proxyURL string, maxConns int) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	proxyFunc := http.ProxyFromEnvironment
	if proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			proxyFunc = http.ProxyURL(parsed)
		}
	}

	if maxConns <= 0 {
		maxConns = poolMaxConnsPerHost
	}

	return &http.Transport{
		Proxy: proxyFunc,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},

		MaxIdleConns:        poolMaxIdleConns,
		MaxIdleConnsPerHost: poolMaxIdleConnsPerHost,
		MaxConnsPerHost:     maxConns,

		IdleConnTimeout:       poolIdleConnTimeout,
		TLSHandshakeTimeout:   poolTLSHandshakeTimeout,
		ResponseHeaderTimeout: poolResponseHeaderTO,
		ExpectContinueTimeout: poolExpectContinueTO,

		// Disable compression so we receive raw bytes and can seek correctly.
		DisableCompression: true,
		// Force HTTP/1.1 — HTTP/2 multiplexing interferes with byte-range
		// parallelism because all streams share one connection.
		ForceAttemptHTTP2: false,
		TLSNextProto:      make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
}
