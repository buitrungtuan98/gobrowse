package network

import (
	"net/http"
	"net/url"
	"sync"
)

// EphemeralJar is an in-memory cookie jar designed for the Tor context to prevent disk leaks.
type EphemeralJar struct {
	mu      sync.RWMutex
	cookies map[string][]*http.Cookie
}

// NewEphemeralJar initializes a zero-footprint cookie jar.
func NewEphemeralJar() *EphemeralJar {
	return &EphemeralJar{
		cookies: make(map[string][]*http.Cookie),
	}
}

// SetCookies stores cookies in memory for the given URL context.
func (j *EphemeralJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cookies[u.Host] = append(j.cookies[u.Host], cookies...)
}

// Cookies returns the valid stored cookies for the URL.
func (j *EphemeralJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.cookies[u.Host]
}
