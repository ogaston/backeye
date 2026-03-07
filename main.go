package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
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
	Host   host.Host
	PubSub *pubsub.PubSub
	Topic  *pubsub.Topic
	wg     sync.WaitGroup
}

// NewSwarmNode initializes a new libp2p host and pubsub system
func NewSwarmNode(ctx context.Context) (*SwarmNode, error) {
	// 1. Initialize Host
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	// 2. Initialize PubSub (GossipSub)
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to setup pubsub: %w", err)
	}

	// 3. Join Topic
	topic, err := ps.Join(PubSubTopicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	return &SwarmNode{
		Host:   h,
		PubSub: ps,
		Topic:  topic,
	}, nil
}

// Start manages the lifecycle of background workers
func (n *SwarmNode) Start(ctx context.Context) {
	fmt.Printf("[*] Node Online: %s\n", n.Host.ID())

	// Setup Discovery
	ser := mdns.NewMdnsService(n.Host, DiscoveryServiceTag, &discoveryNotifee{h: n.Host})
	if err := ser.Start(); err != nil {
		log.Printf("Discovery failed to start: %v", err)
	}

	// Setup Direct Message Handler
	n.Host.SetStreamHandler(DirectMsgProtocol, n.handleIncomingStream)

	// Launch Background Workers
	n.wg.Add(2)
	go n.alertListener(ctx)
	go n.periodicSender(ctx)

	// Wait for context cancellation (Ctrl+C)
	<-ctx.Done()
	fmt.Println("\n[*] Shutting down gracefully...")
	n.Host.Close()
	n.wg.Wait() // Wait for workers to exit
	fmt.Println("[*] Shutdown complete.")
}

func (n *SwarmNode) alertListener(ctx context.Context) {
	defer n.wg.Done()
	sub, _ := n.Topic.Subscribe()
	defer sub.Cancel()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			return // Context cancelled or subscription closed
		}
		fmt.Printf("\n[ALERT] From %s: %s\n", msg.ReceivedFrom.String()[:8], string(msg.Data))
	}
}

func (n *SwarmNode) periodicSender(ctx context.Context) {
	defer n.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	messages := []string{"Sector clear.", "Target acquired.", "Movement detected."}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			peers := n.Host.Network().Peers()
			if len(peers) == 0 {
				continue
			}

			target := peers[rand.Intn(len(peers))]
			msg := messages[rand.Intn(len(messages))]
			
			if err := n.sendDirectMessage(ctx, target, msg); err != nil {
				log.Printf("Failed to send to %s: %v", target, err)
			}
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
	n_bytes, err := s.Read(buf)
	if err != nil {
		return
	}
	fmt.Printf("\n[DIRECT] From %s: %s\n", s.Conn().RemotePeer().String()[:8], string(buf[:n_bytes]))
}

// Main entry point
func main() {

	adapter, err := NewAdapter(8089)
	if err != nil {
		log.Fatal(err)
	}
	defer adapter.Close()
	
	// Create a context that listens for the interrupt signal from the OS
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	node, err := NewSwarmNode(ctx)
	if err != nil {
		log.Fatal(err)
	}

	go adapter.Listen(ctx)
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