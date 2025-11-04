package main

import "time"

const (
	// HTTP Client configuration
	defaultHTTPTimeout         = 30 * time.Second
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 10
	defaultIdleConnTimeout     = 90 * time.Second

	// User Agent
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"

	// Default configuration values
	defaultWorkers  = 10
	defaultInterval = 1 * time.Second
	defaultTTL      = 5 * time.Minute
)

// Config holds the configuration for HLSWarmer
type Config struct {
	Workers    int
	Referer    string
	Origin     string
	PlaybackID string
	Interval   time.Duration
	TTL        time.Duration
	RewarmLast int
	DaemonMode bool
	Debug      bool
	Quiet      bool
}

// CacheStatus represents the status of a segment request
type CacheStatus struct {
	URL        string
	Hit        bool
	StatusCode int
	Headers    map[string]string
	Error      error
	Duration   time.Duration
}

// WarmResult represents the result of warming an M3U8 playlist
type WarmResult struct {
	M3U8URL     string
	TotalFiles  int
	CachedFiles int
	Errors      []error
	Duration    time.Duration
	Details     []CacheStatus
}
