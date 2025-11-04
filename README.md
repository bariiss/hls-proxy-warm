# HLS Proxy Warmer

A Go application designed to cache M3U8 playlist files and their media segments.

## Features

- 🔥 Parses M3U8 playlist files
- 📋 Finds all media segments within
- 🚀 Warms segments in parallel (performs fake downloads)
- 📊 Detects cache status from response headers
- 📈 Detailed statistics and reporting
- ⚡ Performance optimization with configurable worker count

## Usage

```bash
# For a single M3U8 file
go run main.go https://example.com/playlist.m3u8

# For multiple M3U8 files
go run main.go https://example1.com/playlist.m3u8 https://example2.com/playlist.m3u8
```

## Sample Output

```text
🔥 Starting to warm M3U8: https://example.com/playlist.m3u8
📋 Found 120 segments
🔄 Warming: https://example.com/segment001.ts
   ✅ HIT (200) - 45ms
🔄 Warming: https://example.com/segment002.ts
   ❌ MISS (200) - 250ms

📊 RESULTS
==========================================
M3U8 URL: https://example.com/playlist.m3u8
Total Files: 120
Cache Hit: 85
Cache Miss: 35
Error Count: 0
Total Duration: 15.4s
Cache Ratio: 70.83%
```

## Cache Detection

The application detects cache status by checking the following HTTP headers:

- `X-Cache`
- `X-Cache-Status`
- `X-Served-By`
- `CF-Cache-Status` (Cloudflare)
- `X-Fastly-Cache` (Fastly)
- `X-Varnish-Cache` (Varnish)
- `Age`

## Configuration

You can modify the following parameters in the code:

- `maxWorkers`: Number of parallel workers (default: 10)
- `Timeout`: HTTP timeout duration (default: 30s)
- `userAgent`: User-Agent string

## Build

```bash
# Direct execution
go run main.go <m3u8_url>

# Create binary
go build -o hls-warmer main.go
./hls-warmer <m3u8_url>
```

## How It Works

1. Downloads and parses the M3U8 playlist file
2. Extracts all media segment URLs
3. Makes HTTP GET requests for each segment
4. Reads the response body (for caching)
5. Detects cache status from response headers
6. Collects results and generates a report

This way, media files are cached in CDN or cache servers, providing faster access for real users.
