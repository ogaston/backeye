package main

import (
	"fmt"
	"sync"
	"time"
)

type PeerInfo struct {
	PeerID    string
	Location  string
	FaceCount int
	LastSeen  time.Time
}

type PeerStore struct {
	mu                sync.RWMutex
	peers             map[string]PeerInfo
	lastSuspectLocation string
	lastSuspectTime     time.Time
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]PeerInfo),
	}
}

// UpdatePeer upserts peer info and returns the previous suspect location
// if it changed (empty string if no change or first sighting).
func (s *PeerStore) UpdatePeer(alert DetectionAlert) (prevLocation string, moved bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.lastSuspectLocation
	moved = prev != "" && prev != alert.Location

	s.peers[alert.NodeID] = PeerInfo{
		PeerID:    alert.NodeID,
		Location:  alert.Location,
		FaceCount: alert.FaceCount,
		LastSeen:  alert.Timestamp,
	}

	s.lastSuspectLocation = alert.Location
	s.lastSuspectTime = alert.Timestamp

	if moved {
		return prev, true
	}
	return "", false
}

func (s *PeerStore) GetAllPeers() []PeerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PeerInfo, 0, len(s.peers))
	for _, p := range s.peers {
		result = append(result, p)
	}
	return result
}

func (s *PeerStore) GetLastSuspectLocation() (string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSuspectLocation, s.lastSuspectTime
}

func (s *PeerStore) PrintStatus() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Println("\n=== PEER STATUS ===")
	for _, p := range s.peers {
		if p.LastSeen.IsZero() {
			fmt.Printf("  %s | %-15s | (no detections)\n", shortID(p.PeerID), p.Location)
		} else {
			fmt.Printf("  %s | %-15s | last seen: %s\n",
				shortID(p.PeerID), p.Location, p.LastSeen.Format("15:04:05"))
		}
	}
	if s.lastSuspectLocation != "" {
		fmt.Printf("  Suspect last seen at %q (%s)\n",
			s.lastSuspectLocation, s.lastSuspectTime.Format("15:04:05"))
	}
	fmt.Println("====================")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + ".."
	}
	return id
}
