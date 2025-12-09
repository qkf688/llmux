package providers

import (
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type clientCache struct {
	mu      sync.RWMutex
	clients map[time.Duration]*http.Client
}

var cache = &clientCache{
	clients: make(map[time.Duration]*http.Client),
}

var dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

// GetClient returns an http.Client with the specified responseHeaderTimeout.
// If a client with the same timeout already exists, it returns the cached one.
// Otherwise, it creates a new client and caches it.
func GetClient(responseHeaderTimeout time.Duration) *http.Client {
	cache.mu.RLock()
	if client, exists := cache.clients[responseHeaderTimeout]; exists {
		cache.mu.RUnlock()
		return client
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := cache.clients[responseHeaderTimeout]; exists {
		return client
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // No overall timeout, let ResponseHeaderTimeout control header timing
	}

	cache.clients[responseHeaderTimeout] = client
	return client
}

// GetClientWithProxy returns an http.Client with the specified responseHeaderTimeout and proxy.
// This creates a new client each time and does not use caching.
func GetClientWithProxy(responseHeaderTimeout time.Duration, proxyURL string) *http.Client {
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	// 如果提供了代理URL，使用它；否则使用环境变量
	if proxyURL != "" {
		if parsedURL, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsedURL)
		} else {
			// 解析失败，回退到环境变量
			transport.Proxy = http.ProxyFromEnvironment
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}
