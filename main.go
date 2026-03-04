package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

const DiscoveryServiceTag = "defense-swarm-v1"
const DirectMsgProtocol = protocol.ID("/backeye/direct/1.0.0")

var randomMessages = []string{
	"Sector clear.",
	"Target acquired.",
	"Movement detected at grid 7.",
	"Perimeter breach — north flank.",
	"All units hold position.",
}

func main() {
	ctx := context.Background()
	
	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}
	defer host.Close()


	fmt.Printf("[*] Node Online: %s\n", host.ID())

	ser := mdns.NewMdnsService(host, DiscoveryServiceTag, &discoveryNotifee{h: host})
	if err := ser.Start(); err != nil {
		panic(err)
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		panic(err)
	}

	topic, err := ps.Join("detection-alerts")
	if err != nil {
		panic(err)
	}

	sub, _ := topic.Subscribe()

	// 4. Goroutine: Listen for messages from the swarm
	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				return
			}
			// In a real app, this would be a JSON struct
			fmt.Printf("\n[ALERT] Received from %s: %s\n", msg.ReceivedFrom, string(msg.Data))
		}
	}()

	// Handle incoming direct peer messages
	host.SetStreamHandler(DirectMsgProtocol, func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 256)
		n, _ := s.Read(buf)
		fmt.Printf("\n[DIRECT] From %s: %s\n", s.Conn().RemotePeer(), string(buf[:n]))
	})

	// 5. Main Loop: Periodically send a random message to a random peer
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			peers := host.Network().Peers()
			if len(peers) == 0 {
				fmt.Print(".")
				continue
			}
			target := peers[rand.Intn(len(peers))]
			msg := randomMessages[rand.Intn(len(randomMessages))]
			s, err := host.NewStream(ctx, target, DirectMsgProtocol)
			if err != nil {
				fmt.Printf("\n[!] Stream to %s failed: %v\n", target, err)
				continue
			}
			s.Write([]byte(msg))
			s.Close()
			fmt.Printf("\n[->] Sent to %s: %s\n", target, msg)
		}
	}()

	fmt.Println("[*] Listening for swarm alerts... (Ctrl+C to exit)")
	select {}
}

// discoveryNotifee handles finding new peers
type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("[+] Found peer: %s. Connecting...\n", pi.ID)
	n.h.Connect(context.Background(), pi)
	fmt.Printf("[+] Connected to peer: %s\n", pi.ID)
}