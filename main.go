package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	// Parse command line flags
	var (
		referer    = flag.String("referer", "", "Referer header to send with requests")
		origin     = flag.String("origin", "", "Origin header to send with requests")
		playbackID = flag.String("playback-id", "", "X-Playback-Session-Id header (auto-generated if not provided)")
		workers    = flag.Int("workers", defaultWorkers, "Number of parallel workers")
		daemon     = flag.Bool("daemon", false, "Run in daemon mode (continuously)")
		interval   = flag.Duration("interval", defaultInterval, "Check interval for daemon mode")
		rewarmLast = flag.Int("rewarm-last", 0, "Rewarm last N segments every cycle")
		ttl        = flag.Duration("ttl", defaultTTL, "How long before a processed segment is considered stale")
		debug      = flag.Bool("debug", false, "Show debug information including headers")
		quiet      = flag.Bool("quiet", false, "Suppress detailed output (only show summary)")
		help       = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	if *help || flag.NArg() < 1 {
		printHelp()
		os.Exit(0)
	}

	// Create warmer with config
	config := Config{
		Workers:    *workers,
		Referer:    *referer,
		Origin:     *origin,
		PlaybackID: *playbackID,
		Interval:   *interval,
		TTL:        *ttl,
		RewarmLast: *rewarmLast,
		DaemonMode: *daemon,
		Debug:      *debug,
		Quiet:      *quiet,
	}

	warmer := NewHLSWarmer(config)

	// Print configuration
	if *referer != "" {
		fmt.Printf("🔗 Using Referer: %s\n", *referer)
	}
	if *origin != "" {
		fmt.Printf("🌐 Using Origin: %s\n", *origin)
	}
	fmt.Printf("🎯 Playback Session ID: %s\n", warmer.GetPlaybackSessionID())

	m3u8URLs := flag.Args()

	if *daemon {
		runDaemonMode(warmer, m3u8URLs)
	} else {
		runOnceMode(warmer, m3u8URLs)
	}
}

func printHelp() {
	fmt.Println("HLS Proxy Warmer - Cache M3U8 playlists and segments")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s [options] <m3u8_url1> [m3u8_url2] ...\n", os.Args[0])
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -referer string     Referer header to send with requests")
	fmt.Println("  -origin string      Origin header to send with requests")
	fmt.Println("  -playback-id string X-Playback-Session-Id header (auto-generated if not provided)")
	fmt.Printf("  -workers int        Number of parallel workers (default %d)\n", defaultWorkers)
	fmt.Println("  -daemon             Run in daemon mode (continuously)")
	fmt.Printf("  -interval duration  Check interval for daemon mode (default %v)\n", defaultInterval)
	fmt.Println("  -rewarm-last int    Rewarm last N segments every cycle")
	fmt.Printf("  -ttl duration       How long before a processed segment is considered stale (default %v)\n", defaultTTL)
	fmt.Println("  -debug              Show debug information including headers")
	fmt.Println("  -quiet              Suppress detailed output (only show summary)")
	fmt.Println("  -help               Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s https://example.com/playlist.m3u8\n", os.Args[0])
	fmt.Printf("  %s -daemon -interval 15s https://example.com/playlist.m3u8\n", os.Args[0])
	fmt.Printf("  %s -referer \"https://example.com/\" https://example.com/playlist.m3u8\n", os.Args[0])
	fmt.Printf("  %s -workers 20 https://example.com/\n", os.Args[0])
}

func runDaemonMode(warmer *HLSWarmer, m3u8URLs []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🔄 Shutting down gracefully...")
		cancel()
	}()

	// Run daemon
	err := warmer.RunDaemon(ctx, m3u8URLs)
	if err != nil && err != context.Canceled {
		log.Printf("⚠️ Daemon error: %v", err)
	}
}

func runOnceMode(warmer *HLSWarmer, m3u8URLs []string) {
	for _, m3u8URL := range m3u8URLs {
		fmt.Printf("\n🚀 Processing %s...\n", m3u8URL)

		result, err := warmer.WarmM3U8(m3u8URL)
		if err != nil {
			log.Printf("⚠️ Error: %v", err)
			continue
		}

		warmer.PrintResults(result)
		fmt.Println("\n" + strings.Repeat("=", 50))
	}
}
