package main

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"strings"
)

// generateUUID generates a random UUID v4
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)

	// Set version (4) and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08X-%04X-%04X-%04X-%012X",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// extractBaseURL extracts the base URL (scheme + host) from a given URL
func extractBaseURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Return scheme + host (e.g., "https://example.com")
	return parsedURL.Scheme + "://" + parsedURL.Host
}

// cleanString removes non-printable characters that might corrupt terminal output
func cleanString(s string) string {
	// Remove null bytes and other problematic characters
	s = strings.ReplaceAll(s, "\x00", "")

	// Replace non-printable characters except for tab, newline, and carriage return
	return strings.Map(func(r rune) rune {
		// Allow normal printable ASCII characters and some basic whitespace
		if (r >= 32 && r <= 126) || r == 9 || r == 10 || r == 13 {
			return r
		}
		// Skip/remove problematic characters completely for URLs
		return -1
	}, s)
}

// resolveURL resolves a relative URL against a base URL
func resolveURL(baseURL *url.URL, segment string) string {
	// If segment is already a full URL, use it directly
	if strings.HasPrefix(segment, "http://") || strings.HasPrefix(segment, "https://") {
		return segment
	}

	// Resolve relative URL
	segmentURL, err := url.Parse(segment)
	if err != nil {
		return segment
	}

	return baseURL.ResolveReference(segmentURL).String()
}
