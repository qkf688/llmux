package providers

import (
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type clientKey struct {
	responseHeaderTimeout time.Duration
	proxyURL              string
}

type clientCache struct {
	mu      sync.RWMutex
	clients map[clientKey]*http.Client
}

var cache = &clientCache{
	clients: make(map[clientKey]*http.Client),
}

var dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

func normalizeProxy(proxyURL string) (string, func(*http.Request) (*url.URL, error)) {
	if proxyURL == "" {
		return "", http.ProxyFromEnvironment
	}
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return "", http.ProxyFromEnvironment
	}
	return proxyURL, http.ProxyURL(parsedURL)
}

func getClient(responseHeaderTimeout time.Duration, proxyKey string, proxy func(*http.Request) (*url.URL, error)) *http.Client {
	key := clientKey{responseHeaderTimeout: responseHeaderTimeout, proxyURL: proxyKey}

	cache.mu.RLock()
	if client, exists := cache.clients[key]; exists {
		cache.mu.RUnlock()
		return client
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if client, exists := cache.clients[key]; exists {
		return client
	}

	transport := &http.Transport{
		Proxy:                 proxy,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false, // 禁用强制HTTP/2，让系统自动协商，避免HTTP/2超时问题
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
		DisableKeepAlives:     false, // 保持连接复用以提高性能
		MaxIdleConnsPerHost:   10,    // 限制每个主机的空闲连接数
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // No overall timeout, let ResponseHeaderTimeout control header timing
	}

	cache.clients[key] = client
	return client
}

// GetClient returns a cached http.Client with the specified responseHeaderTimeout.
// It reuses the underlying Transport so connections can be kept alive and pooled.
func GetClient(responseHeaderTimeout time.Duration) *http.Client {
	proxyKey, proxy := normalizeProxy("")
	return getClient(responseHeaderTimeout, proxyKey, proxy)
}

// GetClientWithProxy returns a cached http.Client with the specified responseHeaderTimeout and proxy.
// If proxyURL is empty or invalid, it falls back to environment proxy settings.
func GetClientWithProxy(responseHeaderTimeout time.Duration, proxyURL string) *http.Client {
	proxyKey, proxy := normalizeProxy(proxyURL)
	return getClient(responseHeaderTimeout, proxyKey, proxy)
}
