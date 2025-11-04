package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HLSWarmer handles warming of HLS streams
type HLSWarmer struct {
	client        *http.Client
	maxWorkers    int
	userAgent     string
	referer       string
	origin        string
	playbackID    string
	cacheStats    map[string]CacheStatus
	mu            sync.RWMutex
	interval      time.Duration
	daemonMode    bool
	debug         bool
	quiet         bool
	processedURLs map[string]time.Time
	processedTTL  time.Duration
	rewarmLast    int
	streamMu      sync.Mutex
	streamActive  map[string]bool
}

// NewHLSWarmer creates a new HLSWarmer instance
func NewHLSWarmer(config Config) *HLSWarmer {
	// Set defaults
	if config.Workers == 0 {
		config.Workers = defaultWorkers
	}
	if config.Interval == 0 {
		config.Interval = defaultInterval
	}
	if config.TTL == 0 {
		config.TTL = defaultTTL
	}
	if config.PlaybackID == "" {
		config.PlaybackID = generateUUID()
	}

	return &HLSWarmer{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        defaultMaxIdleConns,
				MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
				IdleConnTimeout:     defaultIdleConnTimeout,
			},
		},
		maxWorkers:    config.Workers,
		userAgent:     defaultUserAgent,
		referer:       config.Referer,
		origin:        config.Origin,
		playbackID:    config.PlaybackID,
		cacheStats:    make(map[string]CacheStatus),
		interval:      config.Interval,
		daemonMode:    config.DaemonMode,
		debug:         config.Debug,
		quiet:         config.Quiet,
		processedURLs: make(map[string]time.Time),
		processedTTL:  config.TTL,
		rewarmLast:    config.RewarmLast,
		streamActive:  make(map[string]bool),
	}
}

// GetPlaybackSessionID returns the current playback session ID
func (h *HLSWarmer) GetPlaybackSessionID() string {
	return h.playbackID
}

// WarmM3U8 warms an M3U8 playlist and its segments
func (h *HLSWarmer) WarmM3U8(m3u8URL string) (*WarmResult, error) {
	startTime := time.Now()

	// Auto-detect referer if not set
	if h.referer == "" {
		if baseReferer := extractBaseURL(m3u8URL); baseReferer != "" {
			h.referer = baseReferer
			fmt.Printf("🔗 Auto-detected Referer: %s\n", h.referer)
		}
	}

	// Auto-detect origin if not set
	if h.origin == "" {
		if baseOrigin := extractBaseURL(m3u8URL); baseOrigin != "" {
			h.origin = baseOrigin
			fmt.Printf("🌐 Auto-detected Origin: %s\n", baseOrigin)
		}
	}

	fmt.Printf("🔥 Starting to warm M3U8: %s\n", m3u8URL)

	// Download and parse M3U8 file
	segments, err := h.parseM3U8(m3u8URL)
	if err != nil {
		return nil, fmt.Errorf("M3U8 parse error: %v", err)
	}

	fmt.Printf("📋 Found %d segments\n", len(segments))

	// Warm segments in parallel
	results := h.warmSegments(segments)

	// Collect results
	result := &WarmResult{
		M3U8URL:    m3u8URL,
		TotalFiles: len(segments),
		Duration:   time.Since(startTime),
		Details:    results,
	}

	// Calculate statistics
	for _, r := range results {
		if r.Error != nil {
			result.Errors = append(result.Errors, r.Error)
		} else if r.Hit {
			result.CachedFiles++
		}
	}

	return result, nil
}

// warmSegments warms multiple segments in parallel
func (h *HLSWarmer) warmSegments(segments []string) []CacheStatus {
	jobs := make(chan string, len(segments))
	results := make(chan CacheStatus, len(segments))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < h.maxWorkers; i++ {
		wg.Add(1)
		go h.worker(jobs, results, &wg)
	}

	// Send jobs
	for _, segment := range segments {
		jobs <- segment
	}
	close(jobs)

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []CacheStatus
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// worker processes segment warming jobs
func (h *HLSWarmer) worker(jobs <-chan string, results chan<- CacheStatus, wg *sync.WaitGroup) {
	defer wg.Done()

	for segmentURL := range jobs {
		result := h.warmSegment(segmentURL)
		results <- result
	}
}

// warmSegment warms a single segment
func (h *HLSWarmer) warmSegment(segmentURL string) CacheStatus {
	startTime := time.Now()

	if !h.debug && !h.quiet {
		fmt.Printf("🔄 Warming: %s\n", segmentURL)
	}

	resp, err := h.makeRequest(segmentURL)
	if err != nil {
		// Clean error message to prevent terminal corruption
		errMsg := cleanString(err.Error())

		return CacheStatus{
			URL:      segmentURL,
			Error:    fmt.Errorf("%s", errMsg),
			Duration: time.Since(startTime),
		}
	}
	defer resp.Body.Close()

	// Read response (for caching)
	if err := discardBody(resp); err != nil {
		// Clean error message to prevent terminal corruption
		errMsg := cleanString(err.Error())

		return CacheStatus{
			URL:      segmentURL,
			Error:    fmt.Errorf("%s", errMsg),
			Duration: time.Since(startTime),
		}
	}

	// Check cache status
	cacheHit := h.detectCacheHit(resp)

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	status := CacheStatus{
		URL:        segmentURL,
		Hit:        cacheHit,
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Duration:   time.Since(startTime),
	}

	// Show cache status
	cacheStatus := "⚠️ MISS"
	if cacheHit {
		cacheStatus = "✅ HIT"
	}

	if !h.quiet {
		fmt.Printf("   %s (%d) - %v\n", cacheStatus, resp.StatusCode, time.Since(startTime))
	}

	h.mu.Lock()
	h.cacheStats[segmentURL] = status
	h.mu.Unlock()

	return status
}

// PrintResults prints the warming results
func (h *HLSWarmer) PrintResults(result *WarmResult) {
	fmt.Printf("\n📊 RESULTS\n")
	fmt.Printf("==========================================\n")
	fmt.Printf("M3U8 URL: %s\n", result.M3U8URL)
	fmt.Printf("Total Files: %d\n", result.TotalFiles)
	fmt.Printf("Cache Hit: %d\n", result.CachedFiles)
	fmt.Printf("Cache Miss: %d\n", result.TotalFiles-result.CachedFiles)
	fmt.Printf("Error Count: %d\n", len(result.Errors))
	fmt.Printf("Total Duration: %v\n", result.Duration)
	fmt.Printf("Cache Ratio: %.2f%%\n", float64(result.CachedFiles)/float64(result.TotalFiles)*100)

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️ ERRORS:\n")
		for i, err := range result.Errors {
			fmt.Printf("%d. %v\n", i+1, err)
		}
	}

	fmt.Printf("\n🔍 DETAILS:\n")
	for i, detail := range result.Details {
		status := "⚠️ MISS"
		if detail.Hit {
			status = "✅ HIT"
		}

		if detail.Error != nil {
			fmt.Printf("%d. ⚠️ ERROR - %s: %v\n", i+1, detail.URL, detail.Error)
		} else {
			fmt.Printf("%d. %s (%d) - %s [%v]\n", i+1, status, detail.StatusCode, detail.URL, detail.Duration)
		}
	}
}

// beginStreamProcessing marks a stream as being processed if it is not already.
// Returns true when processing should continue, false when another worker already handles it.
func (h *HLSWarmer) beginStreamProcessing(stream string) bool {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()

	if h.streamActive[stream] {
		return false
	}

	h.streamActive[stream] = true
	return true
}

// endStreamProcessing clears the processing flag for the given stream.
func (h *HLSWarmer) endStreamProcessing(stream string) {
	h.streamMu.Lock()
	delete(h.streamActive, stream)
	h.streamMu.Unlock()
}
