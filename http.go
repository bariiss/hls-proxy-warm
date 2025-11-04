package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// makeRequest creates and executes an HTTP request with appropriate headers
func (h *HLSWarmer) makeRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", h.userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Sec-Fetch-Dest", "video")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Priority", "u=3, i")

	// Set referer header if provided
	if h.referer != "" {
		req.Header.Set("Referer", h.referer)
	}

	// Set origin header if provided
	if h.origin != "" {
		req.Header.Set("Origin", h.origin)
	}

	// Set playback session ID header
	if h.playbackID != "" {
		req.Header.Set("X-Playback-Session-Id", h.playbackID)
	}

	// Debug output
	if h.debug {
		fmt.Printf("🐛 DEBUG - Making request to: %s\n", url)
		fmt.Printf("🐛 DEBUG - Headers:\n")
		for key, values := range req.Header {
			for _, value := range values {
				fmt.Printf("🐛   %s: %s\n", key, value)
			}
		}
		fmt.Printf("🔄 Warming: %s\n", url)
	}

	return h.client.Do(req)
}

// detectCacheHit detects if a response was served from cache
func (h *HLSWarmer) detectCacheHit(resp *http.Response) bool {
	// Check various headers to detect cache status
	cacheHeaders := []string{
		"X-Cache",
		"X-Cache-Status",
		"X-Served-By",
		"CF-Cache-Status", // Cloudflare
		"X-Fastly-Cache",  // Fastly
		"X-Varnish-Cache", // Varnish
		"Age",
	}

	for _, header := range cacheHeaders {
		value := resp.Header.Get(header)
		if value != "" {
			// Cache hit indicators
			hitIndicators := []string{"hit", "HIT", "cached", "CACHED"}
			for _, indicator := range hitIndicators {
				if strings.Contains(strings.ToLower(value), strings.ToLower(indicator)) {
					return true
				}
			}
		}
	}

	// If Age header exists, it might be from cache
	if age := resp.Header.Get("Age"); age != "" && age != "0" {
		return true
	}

	return false
}

// discardBody reads and discards the response body
func discardBody(resp *http.Response) error {
	_, err := io.Copy(io.Discard, resp.Body)
	return err
}
