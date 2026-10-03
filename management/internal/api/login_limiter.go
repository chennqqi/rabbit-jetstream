package api

import (
	"sync"
	"time"
)

type loginAttempt struct {
	window   time.Time
	failures int
}
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]loginAttempt
	now     func() time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: make(map[string]loginAttempt), now: time.Now}
}

func (l *loginLimiter) allowed(ip, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, rule := range []struct {
		key string
		max int
	}{{"ip:" + ip, 20}, {"account:" + ip + "\x00" + username, 5}} {
		entry := l.entries[rule.key]
		if now.Sub(entry.window) >= time.Minute || now.Before(entry.window) {
			delete(l.entries, rule.key)
			continue
		}
		if entry.failures >= rule.max {
			return false, time.Minute - now.Sub(entry.window)
		}
	}
	return true, 0
}

func (l *loginLimiter) failed(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, key := range []string{"ip:" + ip, "account:" + ip + "\x00" + username} {
		entry := l.entries[key]
		if entry.window.IsZero() || now.Sub(entry.window) >= time.Minute || now.Before(entry.window) {
			entry = loginAttempt{window: now}
		}
		entry.failures++
		l.entries[key] = entry
	}
	if len(l.entries) > 10000 {
		for key, entry := range l.entries {
			if now.Sub(entry.window) >= time.Minute {
				delete(l.entries, key)
			}
		}
		if len(l.entries) > 10000 {
			l.entries = make(map[string]loginAttempt)
		}
	}
}

func (l *loginLimiter) succeeded(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, "account:"+ip+"\x00"+username)
}
