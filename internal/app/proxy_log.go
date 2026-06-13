package app

import (
	"strings"
	"sync"
	"time"
)

const proxyLogInterval = 10 * time.Minute

type proxyLogLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (l *proxyLogLimiter) allow(purpose string, reason string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := purpose + ":" + reason
	if last, ok := l.last[key]; ok && now.Sub(last) < proxyLogInterval {
		return false
	}
	l.last[key] = now
	return true
}

func safeProxyLogToken(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			out.WriteRune(char)
		case char >= '0' && char <= '9':
			out.WriteRune(char)
		case char == '_' || char == '-':
			out.WriteRune(char)
		}
	}
	token := strings.Trim(out.String(), "_-")
	if token == "" {
		return fallback
	}
	if len(token) > 64 {
		return token[:64]
	}
	return token
}
