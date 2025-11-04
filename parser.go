package main

import (
	"bufio"
	"io"
	"net/url"
	"strings"
)

// parseM3U8 parses an M3U8 playlist and returns segment URLs
func (h *HLSWarmer) parseM3U8(m3u8URL string) ([]string, error) {
	resp, err := h.makeRequest(m3u8URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var segments []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))

	baseURL, err := url.Parse(m3u8URL)
	if err != nil {
		return nil, err
	}

	// Parse M3U8 format
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Clean the line to remove any control characters
		cleanLine := cleanString(line)

		// Validate that the line looks like a valid URL segment
		if len(cleanLine) == 0 || strings.ContainsAny(cleanLine, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f") {
			continue // Skip invalid segments
		}

		// Skip segments that are too short or look like markers
		if len(cleanLine) < 5 {
			continue
		}

		// Resolve URL
		segmentURL := resolveURL(baseURL, cleanLine)

		// Validate the final URL
		if _, err := url.Parse(segmentURL); err != nil {
			continue // Skip URLs that can't be parsed
		}

		// Skip URLs that don't look like valid media segments
		// Valid segments typically have extensions like .ts, .m4s, .png, .jpg, etc.
		// or contain segment identifiers
		if !strings.Contains(segmentURL, ".") && !strings.Contains(segmentURL, "seg") && !strings.Contains(segmentURL, "chunk") {
			continue
		}

		segments = append(segments, segmentURL)
	}

	return segments, scanner.Err()
}
