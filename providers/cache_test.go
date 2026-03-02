package providers

import (
	"testing"
	"time"
)

func TestGetClient_CachesByTimeout(t *testing.T) {
	timeoutA := 2 * time.Second
	timeoutB := 3 * time.Second

	clientA1 := GetClient(timeoutA)
	clientA2 := GetClient(timeoutA)
	if clientA1 != clientA2 {
		t.Fatal("expected GetClient to return cached client for same timeout")
	}

	clientB := GetClient(timeoutB)
	if clientA1 == clientB {
		t.Fatal("expected GetClient to return different client for different timeout")
	}
}

func TestGetClientWithProxy_CachesByTimeoutAndProxy(t *testing.T) {
	timeout := 2 * time.Second

	proxyA := "http://127.0.0.1:8888"
	proxyB := "http://127.0.0.1:9999"

	clientA1 := GetClientWithProxy(timeout, proxyA)
	clientA2 := GetClientWithProxy(timeout, proxyA)
	if clientA1 != clientA2 {
		t.Fatal("expected GetClientWithProxy to return cached client for same timeout and proxy")
	}

	clientB := GetClientWithProxy(timeout, proxyB)
	if clientA1 == clientB {
		t.Fatal("expected GetClientWithProxy to return different client for different proxy")
	}
}

func TestGetClientWithProxy_EmptyOrInvalidProxyReusesEnvClient(t *testing.T) {
	timeout := 2 * time.Second

	clientEnv := GetClient(timeout)
	clientEmpty := GetClientWithProxy(timeout, "")
	if clientEnv != clientEmpty {
		t.Fatal("expected empty proxy to reuse env client")
	}

	clientInvalid := GetClientWithProxy(timeout, "://bad")
	if clientEnv != clientInvalid {
		t.Fatal("expected invalid proxy to fall back and reuse env client")
	}
}

