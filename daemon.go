package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// RunDaemon runs the warmer in daemon mode, continuously warming M3U8 streams
func (h *HLSWarmer) RunDaemon(ctx context.Context, m3u8URLs []string) error {
	fmt.Printf("🔄 Starting daemon mode with %d M3U8 streams\n", len(m3u8URLs))
	fmt.Printf("⏱️  Check interval: %v\n", h.interval)

	// Initial warming
	for _, m3u8URL := range m3u8URLs {
		h.scheduleStreamWarm(m3u8URL)
		go h.warmStreamContinuously(ctx, m3u8URL)
	}

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Println("\n🛑 Daemon mode stopped")
	return ctx.Err()
}

// warmStreamContinuously warms a single stream continuously
func (h *HLSWarmer) warmStreamContinuously(ctx context.Context, m3u8URL string) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.scheduleStreamWarm(m3u8URL)
		}
	}
}

// scheduleStreamWarm triggers a warm cycle for the given stream in the background if no other cycle is currently running.
func (h *HLSWarmer) scheduleStreamWarm(m3u8URL string) {
	if !h.beginStreamProcessing(m3u8URL) {
		if h.debug {
			fmt.Printf("⏳ Stream %s already warming, skipping this tick\n", m3u8URL)
		}
		return
	}

	go func() {
		defer h.endStreamProcessing(m3u8URL)
		h.warmStreamOnce(m3u8URL)
	}()
}

// warmStreamOnce warms a stream once, only processing new segments
func (h *HLSWarmer) warmStreamOnce(m3u8URL string) {
	segments, err := h.parseM3U8(m3u8URL)
	if err != nil {
		// Clean error message to prevent terminal corruption
		errMsg := cleanString(err.Error())
		log.Printf("⚠️ Error parsing M3U8 %s: %s", m3u8URL, errMsg)
		return
	}

	// Filter out already processed segments
	var newSegments []string
	h.mu.Lock()
	for _, segment := range segments {
		last, seen := h.processedURLs[segment]
		if !seen || time.Since(last) > h.processedTTL {
			newSegments = append(newSegments, segment)
			h.processedURLs[segment] = time.Now()
		}
	}
	h.mu.Unlock()

	// Optionally include the last N segments for re-warming even if previously seen
	if h.rewarmLast > 0 {
		h.mu.Lock()
		start := 0
		if len(segments) > h.rewarmLast {
			start = len(segments) - h.rewarmLast
		}
		// use a map to avoid duplicates
		included := make(map[string]struct{})
		for _, s := range newSegments {
			included[s] = struct{}{}
		}
		for i := start; i < len(segments); i++ {
			s := segments[i]
			if _, ok := included[s]; !ok {
				newSegments = append(newSegments, s)
				included[s] = struct{}{}
			}
			// update processed time so it won't be re-added immediately next cycle
			h.processedURLs[s] = time.Now()
		}
		h.mu.Unlock()
	}

	if len(newSegments) == 0 {
		fmt.Printf("🔍 No new segments found for %s\n", m3u8URL)
		return
	}

	fmt.Printf("🆕 Found %d new segments for %s\n", len(newSegments), m3u8URL)

	// Warm new segments
	results := h.warmSegments(newSegments)

	// Count cache hits
	hitCount := 0
	errorCount := 0
	var errorDetails []string
	for _, r := range results {
		if r.Error != nil {
			errorCount++
			if h.quiet {
				// In quiet mode, collect sanitized error messages
				cleanErr := cleanString(r.Error.Error())
				errorDetails = append(errorDetails, fmt.Sprintf("%s: %s", r.URL, cleanErr))
			}
		} else if r.Hit {
			hitCount++
		}
	}

	fmt.Printf("📊 Stream %s: %d new segments, %d hits, %d errors\n",
		m3u8URL, len(newSegments), hitCount, errorCount)

	// Show error details in quiet mode if there are errors
	if h.quiet && errorCount > 0 {
		fmt.Printf("⚠️ Error details:\n")
		for i, errDetail := range errorDetails {
			fmt.Printf("  %d. %s\n", i+1, errDetail)
		}
	}
}
