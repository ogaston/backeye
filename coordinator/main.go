package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/protocol"
)

// Constants for configuration
const (
	DiscoveryServiceTag = "defense-swarm-v1"
	DirectMsgProtocol   = protocol.ID("/backeye/direct/1.0.0")
	PubSubTopicName     = "detection-alerts"
)

func parseEvent(event string) (int, bool) {
	event = strings.TrimSpace(event)
	n, err := strconv.Atoi(event)
	if err != nil {
		return 0, false
	}
	return n, true
}

func main() {
	location := flag.String("location", "unknown", "physical location of this node (e.g. front-door, parking-lot)")
	flag.Parse()

	collector := NewLogsCollector()

	adapter, err := NewAdapter(8089, collector)
	if err != nil {
		log.Fatal(err)
	}
	defer adapter.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := NewLogger(collector)

	node, err := NewSwarmNode(ctx, *location, collector)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				logger.Render(node)
			case <-ctx.Done():
				return
			}
		}
	}()

	go adapter.Listen(ctx, func(event string) {
		if count, ok := parseEvent(event); ok {

			if count > 0 {
				node.PublishDetection(ctx, count)
				message := fmt.Sprintf("Published detection: %d face(s) at %q", count, node.Location)
				logger.MarkPublished(message)
			} else {
				node.PublishDetection(ctx, 0)
				logger.Update(fmt.Sprintf("No faces detected at %q", node.Location))
			}

		}
	})

	node.Start(ctx)
}
