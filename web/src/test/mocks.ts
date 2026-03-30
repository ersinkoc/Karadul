import type { Node, Peer, MeshTopology, SystemStats } from "@/lib/store"
import type { AuthKey } from "@/lib/api"

export const mockNodes: Node[] = [
  {
    id: "node-1",
    hostname: "server-1",
    virtualIP: "10.0.0.1",
    publicKey: "abc123publickey1",
    status: "online",
    endpoint: "192.168.1.100:51820",
    lastSeen: "2026-03-26T10:00:00Z",
    isExitNode: true,
    rxBytes: 1024000,
    txBytes: 2048000,
    routes: ["10.0.0.0/24", "192.168.1.0/24"],
  },
  {
    id: "node-2",
    hostname: "server-2",
    virtualIP: "10.0.0.2",
    publicKey: "def456publickey2",
    status: "online",
    endpoint: "192.168.1.101:51820",
    lastSeen: "2026-03-26T10:05:00Z",
    isExitNode: false,
    rxBytes: 512000,
    txBytes: 1024000,
  },
  {
    id: "node-3",
    hostname: "server-3",
    virtualIP: "10.0.0.3",
    publicKey: "ghi789publickey3",
    status: "offline",
    endpoint: undefined,
    lastSeen: "2026-03-25T10:00:00Z",
    isExitNode: false,
    rxBytes: 0,
    txBytes: 0,
  },
]

export const mockPeers: Peer[] = [
  {
    id: "peer-1",
    hostname: "peer-1",
    virtualIP: "10.0.0.10",
    state: "Direct",
    endpoint: "192.168.1.200:51820",
    lastHandshake: "2026-03-26T09:55:00Z",
    rxBytes: 1024000,
    txBytes: 512000,
    latency: 15,
  },
  {
    id: "peer-2",
    hostname: "peer-2",
    virtualIP: "10.0.0.11",
    state: "Relayed",
    endpoint: undefined,
    lastHandshake: "2026-03-26T09:50:00Z",
    rxBytes: 256000,
    txBytes: 128000,
    latency: 45,
  },
  {
    id: "peer-3",
    hostname: "peer-3",
    virtualIP: "10.0.0.12",
    state: "Idle",
    endpoint: undefined,
    lastHandshake: undefined,
    rxBytes: 0,
    txBytes: 0,
    latency: undefined,
  },
]

export const mockTopology: MeshTopology = {
  nodes: [
    {
      id: "node-1",
      hostname: "server-1",
      virtualIP: "10.0.0.1",
      status: "online",
      isExitNode: true,
    },
    {
      id: "node-2",
      hostname: "server-2",
      virtualIP: "10.0.0.2",
      status: "online",
      isExitNode: false,
    },
  ],
  connections: [
    {
      from: "node-1",
      to: "node-2",
      type: "direct",
    },
  ],
}

export const mockStats: SystemStats = {
  uptime: 86400,
  memoryUsage: 524288000,
  cpuUsage: 25.5,
  goroutines: 50,
  peersConnected: 2,
  totalRx: 1792000,
  totalTx: 3172000,
}

export const mockAuthKeys: AuthKey[] = [
  {
    id: "key-1",
    key: "k1_aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890",
    ephemeral: false,
    createdAt: "2026-03-25T10:00:00Z",
    expiresAt: undefined,
    used: false,
  },
  {
    id: "key-2",
    key: "k2_aBcDeFgHiJkLmNoPqRsTuVwXyZ0987654321",
    ephemeral: true,
    createdAt: "2026-03-24T10:00:00Z",
    expiresAt: "2026-04-24T10:00:00Z",
    usedAt: "2026-03-24T11:00:00Z",
    used: true,
  },
]
