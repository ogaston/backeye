package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// SwarmNode encapsulates the P2P logic
type SwarmNode struct {
	Host      host.Host
	PubSub    *pubsub.PubSub
	Topic     *pubsub.Topic
	Location  string
	Store     *PeerStore
	wg        sync.WaitGroup
	collector *LogsCollector
}

// NewSwarmNode initializes a new libp2p host and pubsub system
func NewSwarmNode(ctx context.Context, location string, collector *LogsCollector) (*SwarmNode, error) {
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
		Host:      h,
		PubSub:    ps,
		Topic:     topic,
		Location:  location,
		Store:     NewPeerStore(),
		collector: collector,
	}, nil
}

// Start manages the lifecycle of background workers
func (n *SwarmNode) Start(ctx context.Context) {
	n.collector.AddLog(fmt.Sprintf("[*] Node Online: %s (location: %s)", n.Host.ID(), n.Location))

	ser := mdns.NewMdnsService(n.Host, DiscoveryServiceTag, &discoveryNotifee{h: n.Host, collector: n.collector})
	if err := ser.Start(); err != nil {
		n.collector.AddLog(fmt.Sprintf("Discovery failed to start: %v", err))
	}

	n.Host.SetStreamHandler(DirectMsgProtocol, n.handleIncomingStream)

	n.wg.Add(2)
	go n.alertListener(ctx)
	go n.statusPrinter(ctx)

	<-ctx.Done()
	n.collector.AddLog("[*] Shutting down gracefully...")
	n.Host.Close()
	n.wg.Wait()
	n.collector.AddLog("[*] Shutdown complete.")
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
			n.collector.AddLog(fmt.Sprintf("[ALERT] bad payload from %s: %v", msg.ReceivedFrom.String()[:8], err))
			continue
		}

		prevLoc, moved := n.Store.UpdatePeer(alert)
		if moved {
			n.collector.AddLog(fmt.Sprintf("[MOVE] Suspect moved from %q to %q - reported by %s at %s",
				prevLoc, alert.Location, shortID(alert.NodeID), alert.Timestamp.Format("15:04:05")))
		} else {
			n.collector.AddLog(fmt.Sprintf("[ALERT] Suspect spotted at %q (%d faces) - reported by %s at %s",
				alert.Location, alert.FaceCount, shortID(alert.NodeID), alert.Timestamp.Format("15:04:05")))
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
		n.collector.AddLog(fmt.Sprintf("Failed to marshal alert: %v", err))
		return
	}
	if err := n.Topic.Publish(ctx, data); err != nil {
		n.collector.AddLog(fmt.Sprintf("Failed to publish alert: %v", err))
		return
	}

	n.Store.UpdatePeer(alert)
}

func (n *SwarmNode) statusPrinter(ctx context.Context) {
	defer n.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.collector.AddLog(n.Store.GetStatusString())
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
	n.collector.AddLog(fmt.Sprintf("[DIRECT] From %s: %s", s.Conn().RemotePeer().String()[:8], string(buf[:nBytes])))
}

// Discovery Logic
type discoveryNotifee struct {
	h         host.Host
	collector *LogsCollector
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if err := n.h.Connect(context.Background(), pi); err == nil {
		n.collector.AddLog(fmt.Sprintf("[+] Connected to: %s", pi.ID.String()[:8]))
	}
}
