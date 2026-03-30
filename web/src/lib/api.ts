import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { useKaradulStore, type Node, type Peer, type MeshTopology, type SystemStats } from "@/lib/store"

const API_BASE = "/api"

// Query keys
export const queryKeys = {
  nodes: ["nodes"] as const,
  peers: ["peers"] as const,
  topology: ["topology"] as const,
  stats: ["stats"] as const,
  node: (id: string) => ["nodes", id] as const,
}

// API client
async function fetchApi<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
    },
    ...options,
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(error || `HTTP ${response.status}`)
  }

  return response.json()
}

// ==================== NODES ====================

export function useNodes() {
  const setNodes = useKaradulStore((state) => state.setNodes)

  return useQuery({
    queryKey: queryKeys.nodes,
    queryFn: async () => {
      const nodes = await fetchApi<Node[]>("/v1/admin/nodes")
      setNodes(nodes)
      return nodes
    },
    refetchInterval: 5000,
  })
}

export function useDeleteNode() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await fetchApi(`/v1/admin/nodes/${id}`, { method: "DELETE" })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.nodes })
      toast.success("Node deleted successfully")
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete node: ${error.message}`)
    },
  })
}

// ==================== PEERS ====================

export function usePeers() {
  const setPeers = useKaradulStore((state) => state.setPeers)

  return useQuery({
    queryKey: queryKeys.peers,
    queryFn: async () => {
      const peers = await fetchApi<Peer[]>("/v1/peers")
      setPeers(peers)
      return peers
    },
    refetchInterval: 3000,
  })
}

// ==================== TOPOLOGY ====================

export function useTopology() {
  const setTopology = useKaradulStore((state) => state.setTopology)

  return useQuery({
    queryKey: queryKeys.topology,
    queryFn: async () => {
      const topology = await fetchApi<MeshTopology>("/v1/topology")
      setTopology(topology)
      return topology
    },
    refetchInterval: 5000,
  })
}

// ==================== STATS ====================

export function useStats() {
  const setStats = useKaradulStore((state) => state.setStats)
  const addTrafficPoint = useKaradulStore((state) => state.addTrafficPoint)
  const prevStats = useKaradulStore((s) => s.stats)
  const prevRx = prevStats?.totalRx ?? 0
  const prevTx = prevStats?.totalTx ?? 0

  return useQuery({
    queryKey: queryKeys.stats,
    queryFn: async () => {
      const stats = await fetchApi<SystemStats>("/v1/status")
      setStats(stats)
      if (prevStats) {
        const deltaRx = Math.max(0, stats.totalRx - prevRx)
        const deltaTx = Math.max(0, stats.totalTx - prevTx)
        addTrafficPoint(deltaRx, deltaTx)
      } else {
        addTrafficPoint(stats.totalRx, stats.totalTx)
      }
      return stats
    },
    refetchInterval: 2000,
  })
}

// ==================== AUTH KEYS ====================

export interface AuthKey {
  id: string
  key: string
  ephemeral: boolean
  expiresAt?: string
  createdAt: string
  usedAt?: string
  used: boolean
}

export function useAuthKeys() {
  return useQuery({
    queryKey: ["auth-keys"],
    queryFn: () => fetchApi<AuthKey[]>("/v1/admin/auth-keys"),
  })
}

/** Convert frontend expiry values (e.g. "7d") to Go time.ParseDuration format (e.g. "168h") */
function toGoDuration(val: string): string {
  const match = val.match(/^(\d+)([hdm])$/)
  if (!match) return val
  const n = parseInt(match[1], 10)
  const unit = match[2]
  if (unit === "d") return `${n * 24}h`
  if (unit === "m") return `${n}m`
  return `${n}h`
}

export function useCreateAuthKey() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (expiresIn?: string) => {
      const expiry = expiresIn ? toGoDuration(expiresIn) : ""
      return fetchApi<AuthKey>("/v1/admin/auth-keys", {
        method: "POST",
        body: JSON.stringify({ expiry }),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth-keys"] })
      toast.success("Auth key created successfully")
    },
    onError: (error: Error) => {
      toast.error(`Failed to create auth key: ${error.message}`)
    },
  })
}

export function useDeleteAuthKey() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await fetchApi(`/v1/admin/auth-keys/${id}`, { method: "DELETE" })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth-keys"] })
      toast.success("Auth key deleted successfully")
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete auth key: ${error.message}`)
    },
  })
}

// ==================== ACL ====================

export interface ACLRule {
  action: "allow" | "deny"
  src: string[]
  dst: string[]
  ports?: string[]
}

export function useACL() {
  return useQuery({
    queryKey: ["acl"],
    queryFn: () => fetchApi<{ rules: ACLRule[] }>("/v1/admin/acl"),
  })
}

export function useUpdateACL() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (rules: ACLRule[]) => {
      return fetchApi("/v1/admin/acl", {
        method: "PUT",
        body: JSON.stringify({ rules }),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["acl"] })
      toast.success("ACL rules updated successfully")
    },
    onError: (error: Error) => {
      toast.error(`Failed to update ACL: ${error.message}`)
    },
  })
}

// ==================== CONFIG ====================

export interface ServerConfig {
  addr?: string
  tls?: { enabled?: boolean; cert_file?: string; key_file?: string; self_signed?: boolean }
  approval_mode?: string
  subnet?: string
  data_dir?: string
  log_level?: string
  log_format?: string
  rate_limit?: number
}

export function useConfig() {
  return useQuery({
    queryKey: ["config"],
    queryFn: () => fetchApi<ServerConfig>("/v1/admin/config"),
  })
}

export function useUpdateConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (config: ServerConfig) => {
      return fetchApi("/v1/admin/config", {
        method: "PUT",
        body: JSON.stringify(config),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["config"] })
      toast.success("Configuration saved successfully")
    },
    onError: (error: Error) => {
      toast.error(`Failed to save config: ${error.message}`)
    },
  })
}
