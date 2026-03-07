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
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// Constants for configuration
const (
	DiscoveryServiceTag = "defense-swarm-v1"
	DirectMsgProtocol   = protocol.ID("/backeye/direct/1.0.0")
	PubSubTopicName     = "detection-alerts"
)

// SwarmNode encapsulates the P2P logic
type SwarmNode struct {
	Host     host.Host
	PubSub   *pubsub.PubSub
	Topic    *pubsub.Topic
	Location string
	Store    *PeerStore
	wg       sync.WaitGroup
}

// NewSwarmNode initializes a new libp2p host and pubsub system
func NewSwarmNode(ctx context.Context, location string) (*SwarmNode, error) {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to setup pubsub: %w", err)
	}

	topic, err := ps.Join(PubSubTopicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	return &SwarmNode{
		Host:     h,
		PubSub:   ps,
		Topic:    topic,
		Location: location,
		Store:    NewPeerStore(),
	}, nil
}

// Start manages the lifecycle of background workers
func (n *SwarmNode) Start(ctx context.Context) {
	fmt.Printf("[*] Node Online: %s (location: %s)\n", n.Host.ID(), n.Location)

	ser := mdns.NewMdnsService(n.Host, DiscoveryServiceTag, &discoveryNotifee{h: n.Host})
	if err := ser.Start(); err != nil {
		log.Printf("Discovery failed to start: %v", err)
	}

	n.Host.SetStreamHandler(DirectMsgProtocol, n.handleIncomingStream)

	n.wg.Add(2)
	go n.alertListener(ctx)
	go n.statusPrinter(ctx)

	<-ctx.Done()
	fmt.Println("\n[*] Shutting down gracefully...")
	n.Host.Close()
	n.wg.Wait()
	fmt.Println("[*] Shutdown complete.")
}

func (n *SwarmNode) alertListener(ctx context.Context) {
	defer n.wg.Done()
	sub, _ := n.Topic.Subscribe()
	defer sub.Cancel()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == n.Host.ID() {
			continue
		}

		alert, err := UnmarshalAlert(msg.Data)
		if err != nil {
			log.Printf("[ALERT] bad payload from %s: %v", msg.ReceivedFrom.String()[:8], err)
			continue
		}

		prevLoc, moved := n.Store.UpdatePeer(alert)
		if moved {
			fmt.Printf("\n[MOVE] Suspect moved from %q to %q - reported by %s at %s\n",
				prevLoc, alert.Location, shortID(alert.NodeID), alert.Timestamp.Format("15:04:05"))
		} else {
			fmt.Printf("\n[ALERT] Suspect spotted at %q (%d faces) - reported by %s at %s\n",
				alert.Location, alert.FaceCount, shortID(alert.NodeID), alert.Timestamp.Format("15:04:05"))
		}
	}
}

// PublishDetection broadcasts a detection event to the mesh.
func (n *SwarmNode) PublishDetection(ctx context.Context, faceCount int) {
	alert := DetectionAlert{
		NodeID:    n.Host.ID().String(),
		Location:  n.Location,
		FaceCount: faceCount,
		Timestamp: time.Now(),
	}

	data, err := MarshalAlert(alert)
	if err != nil {
		log.Printf("Failed to marshal alert: %v", err)
		return
	}
	if err := n.Topic.Publish(ctx, data); err != nil {
		log.Printf("Failed to publish alert: %v", err)
		return
	}

	n.Store.UpdatePeer(alert)
	fmt.Printf("[LOCAL] Published detection: %d face(s) at %q\n", faceCount, n.Location)
}

func (n *SwarmNode) statusPrinter(ctx context.Context) {
	defer n.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.Store.PrintStatus()
		}
	}
}

func (n *SwarmNode) sendDirectMessage(ctx context.Context, peerID peer.ID, msg string) error {
	s, err := n.Host.NewStream(ctx, peerID, DirectMsgProtocol)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = s.Write([]byte(msg))
	return err
}

func (n *SwarmNode) handleIncomingStream(s network.Stream) {
	defer s.Close()
	buf := make([]byte, 256)
	nBytes, err := s.Read(buf)
	if err != nil {
		return
	}
	fmt.Printf("\n[DIRECT] From %s: %s\n", s.Conn().RemotePeer().String()[:8], string(buf[:nBytes]))
}

func parseFaceCount(event string) (int, bool) {
	event = strings.TrimSpace(event)
	if !strings.HasPrefix(event, "FACE_DETECTED count=") {
		return 0, false
	}
	parts := strings.SplitN(event, "=", 2)
	if len(parts) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, false
	}
	return n, true
}

func main() {
	location := flag.String("location", "unknown", "physical location of this node (e.g. front-door, parking-lot)")
	flag.Parse()

	adapter, err := NewAdapter(8089)
	if err != nil {
		log.Fatal(err)
	}
	defer adapter.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	node, err := NewSwarmNode(ctx, *location)
	if err != nil {
		log.Fatal(err)
	}

	go adapter.Listen(ctx, func(event string) {
		if count, ok := parseFaceCount(event); ok {
			node.PublishDetection(ctx, count)
		}
	})

	node.Start(ctx)
}

// Discovery Logic
type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if err := n.h.Connect(context.Background(), pi); err == nil {
		fmt.Printf("[+] Connected to: %s\n", pi.ID.String()[:8])
	}
}