# Resilient Edge Perimeter Mesh (REPM)

**An offline-first, decentralized security system for remote critical infrastructure.**

REPM is a distributed intrusion detection system designed to operate in environments where cloud connectivity is impossible or compromised. By leveraging a mesh network and Byzantine-style consensus, the system filters out false positives (animals, wind, sensor glitches) by requiring multiple nodes to verify a threat before triggering an alarm.

## 🚀 The Core Concept

Traditional security systems often suffer from two extremes: high false-alarm rates or a "single point of failure" (the central server). REPM solves this by:

- **Distributed Verification**: No single node can trigger a high-alert state alone.

- **Offline Resilience**: Uses a local mesh (libp2p) that persists even if the wider internet is jammed or unavailable.

- **Edge Intelligence**: Object detection happens locally on the node; only "verdicts" are sent over the wire.

## 🛠 Tech Stack

- Networking: Go + libp2p (P2P mesh, mTLS, signed messaging).

- Edge AI: C++ / TensorFlow Lite (Local human vs. animal classification).

- Orchestration & UI: Elixir (High-concurrency alert streaming and audit logs).

- Security: Mutual TLS (mTLS) for node-to-node authentication.

## 🎖 Tactical Use Case: Forward Operating Bases (FOB)

In military or high-security remote sites, REPM provides a Low Probability of Detection (LPD). By processing AI locally and only "gossiping" small, encrypted packets, the system minimizes its RF footprint, making it resistant to Signal Intelligence (SIGINT) and electronic warfare.

## 📐 System Architecture

The system follows a Consensus-over-Detection flow:

1. Detection: A node's PIR sensor triggers the camera.
2. Classification: Local AI determines if the object is a "Threat" (Human/Vehicle).
3. Gossip: The node broadcasts a "Detection Event" to the mesh.
4. Consensus: Nodes 1, 2, and 3 compare timestamps and classification data. If $\ge 2$ nodes agree on a threat within a specific spatial/temporal window, a Critical Alert is issued to the Elixir Dashboard.

## 💻 Lab Simulation (3-Laptop Setup)

For development and grading purposes, the physical hardware (ESP32-CAMs) is simulated using three networked laptops:

- Nodes 1 & 2 (Edge Simulators): Running the Go-based mesh client and processing webcam feeds via TensorFlow Lite.

- Node 3 (The Command Hub): Running the Elixir/Phoenix dashboard to visualize the mesh state and log consensus events.

##🚦 Quick Start

**Prerequisites**

Go 1.21+Elixir 1.15+ / Erlang OTP 26OpenCV / TensorFlow Lite C++ headers

**Installation**

Clone the mesh:
```sh
git clone https://github.com/yourusername/repm-mesh.git
cd repm-mesh
```
**Initialize the Mesh Nodes (Go)**:

```sh
# Run on Laptop A and B
go run main.go --peer [address]
```

**Launch the Dashboard (Elixir):**

```sh
mix phx.server
```

## 🛡 Security Design

- Anti-Spoofing: Every message between nodes is signed using Ed25519 keys.

- Jamming Resistance: If Node 1 is cut off, Node 2 and Node 3 re-route traffic automatically via the libp2p DHT.