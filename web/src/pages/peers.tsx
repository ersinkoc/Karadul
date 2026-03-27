import { useState } from "react"
import {
  Users,
  MoreHorizontal,
  Zap,
  ZapOff,
  Router,
  Activity,
  Search,
  RefreshCw,
  Download,
  FileJson,
  FileSpreadsheet,
} from "lucide-react"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { usePeers } from "@/lib/api"
import { formatBytes, formatDuration, cn } from "@/lib/utils"
import { ErrorAlert } from "@/components/error-boundary"
import { EmptyState } from "@/components/empty-state"
import { exportPeersCSV, exportPeersJSON } from "@/lib/export"
import type { Peer } from "@/lib/store"

function ConnectionBadge({ state }: { state: Peer["state"] }) {
  const variants: Record<string, { variant: "default" | "secondary" | "outline" | "destructive"; icon: React.ReactNode }> = {
    Direct: { variant: "default", icon: <Zap className="h-3 w-3" /> },
    Relayed: { variant: "secondary", icon: <Router className="h-3 w-3" /> },
    Connecting: { variant: "outline", icon: <Activity className="h-3 w-3" /> },
    Discovered: { variant: "outline", icon: null },
    Idle: { variant: "secondary", icon: null },
    Expired: { variant: "destructive", icon: <ZapOff className="h-3 w-3" /> },
  }

  const config = variants[state] || { variant: "secondary", icon: null }

  return (
    <Badge variant={config.variant} className="gap-1">
      {config.icon}
      {state}
    </Badge>
  )
}

function PeerSkeleton() {
  return (
    <TableRow>
      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
      <TableCell><Skeleton className="h-4 w-12" /></TableCell>
      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
      <TableCell><Skeleton className="h-8 w-8" /></TableCell>
    </TableRow>
  )
}

function StatCardSkeleton() {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <Skeleton className="h-4 w-[80px]" />
        <Skeleton className="h-4 w-4" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-8 w-[40px]" />
      </CardContent>
    </Card>
  )
}

export function PeersPage() {
  const { data: peers, isLoading, error, refetch } = usePeers()
  const [filter, setFilter] = useState<string>("all")
  const [searchQuery, setSearchQuery] = useState("")

  const filteredPeers = peers?.filter((peer) => {
    const matchesFilter =
      filter === "all"
        ? true
        : filter === "active"
        ? ["Direct", "Relayed"].includes(peer.state)
        : !["Direct", "Relayed"].includes(peer.state)

    const matchesSearch =
      peer.hostname.toLowerCase().includes(searchQuery.toLowerCase()) ||
      peer.virtualIP.includes(searchQuery)

    return matchesFilter && matchesSearch
  })

  const activePeers =
    peers?.filter((p) => ["Direct", "Relayed"].includes(p.state)).length || 0

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Peers</h2>
          <p className="text-muted-foreground">
            Manage peer connections in your mesh network
          </p>
        </div>
        <ErrorAlert
          title="Failed to load peers"
          message={error.message}
          onRetry={() => refetch()}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Peers</h2>
          <p className="text-muted-foreground">
            Manage peer connections in your mesh network
          </p>
        </div>
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" disabled={!peers || peers.length === 0}>
                <Download className="h-4 w-4 mr-2" />
                Export
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem onClick={() => exportPeersCSV(peers || [])}>
                <FileSpreadsheet className="h-4 w-4 mr-2" />
                Export as CSV
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => exportPeersJSON(peers || [])}>
                <FileJson className="h-4 w-4 mr-2" />
                Export as JSON
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            variant="outline"
            size="icon"
            onClick={() => refetch()}
            disabled={isLoading}
          >
            <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        {isLoading ? (
          <>
            <StatCardSkeleton />
            <StatCardSkeleton />
            <StatCardSkeleton />
            <StatCardSkeleton />
          </>
        ) : (
          <>
            <Card className="card-hover">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Total Peers</CardTitle>
                <Users className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{peers?.length || 0}</div>
              </CardContent>
            </Card>

            <Card className="card-hover">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Direct</CardTitle>
                <Zap className="h-4 w-4 text-green-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {peers?.filter((p) => p.state === "Direct").length || 0}
                </div>
              </CardContent>
            </Card>

            <Card className="card-hover">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Relayed</CardTitle>
                <Router className="h-4 w-4 text-amber-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {peers?.filter((p) => p.state === "Relayed").length || 0}
                </div>
              </CardContent>
            </Card>

            <Card className="card-hover">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">Active</CardTitle>
                <Activity className="h-4 w-4 text-blue-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{activePeers}</div>
              </CardContent>
            </Card>
          </>
        )}
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <CardTitle>Peer Connections</CardTitle>
              <CardDescription>All peer connections in the network</CardDescription>
            </div>
            <div className="relative w-full sm:w-64">
              <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search peers..."
                className="pl-8"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="all" className="w-full" onValueChange={setFilter}>
            <TabsList className="mb-4">
              <TabsTrigger value="all">All</TabsTrigger>
              <TabsTrigger value="active">Active</TabsTrigger>
              <TabsTrigger value="inactive">Inactive</TabsTrigger>
            </TabsList>

            <TabsContent value={filter}>
              {isLoading ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Hostname</TableHead>
                      <TableHead>Virtual IP</TableHead>
                      <TableHead>State</TableHead>
                      <TableHead>Endpoint</TableHead>
                      <TableHead>Latency</TableHead>
                      <TableHead>Data (RX/TX)</TableHead>
                      <TableHead>Last Handshake</TableHead>
                      <TableHead className="w-[50px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {Array.from({ length: 5 }).map((_, i) => (
                      <PeerSkeleton key={i} />
                    ))}
                  </TableBody>
                </Table>
              ) : filteredPeers?.length === 0 ? (
                <EmptyState
                  icon={Users}
                  title="No peers found"
                  description={
                    searchQuery
                      ? "No peers match your search criteria. Try a different query."
                      : filter === "active"
                      ? "No active peer connections. Peers will appear here when they connect."
                      : filter === "inactive"
                      ? "No inactive peers."
                      : "Your mesh network doesn't have any peers yet."
                  }
                  action={
                    searchQuery
                      ? { label: "Clear Search", onClick: () => setSearchQuery("") }
                      : undefined
                  }
                />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Hostname</TableHead>
                      <TableHead>Virtual IP</TableHead>
                      <TableHead>State</TableHead>
                      <TableHead>Endpoint</TableHead>
                      <TableHead>Latency</TableHead>
                      <TableHead>Data (RX/TX)</TableHead>
                      <TableHead>Last Handshake</TableHead>
                      <TableHead className="w-[50px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(filteredPeers ?? []).map((peer) => (
                      <TableRow key={peer.id}>
                        <TableCell className="font-medium">
                          {peer.hostname}
                        </TableCell>
                        <TableCell>{peer.virtualIP}</TableCell>
                        <TableCell>
                          <ConnectionBadge state={peer.state} />
                        </TableCell>
                        <TableCell>{peer.endpoint || "N/A"}</TableCell>
                        <TableCell>
                          {peer.latency ? `${peer.latency}ms` : "N/A"}
                        </TableCell>
                        <TableCell>
                          <div className="text-xs">
                            <div className="text-green-600">
                              ↓ {formatBytes(peer.rxBytes)}
                            </div>
                            <div className="text-blue-600">
                              ↑ {formatBytes(peer.txBytes)}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          {peer.lastHandshake
                            ? formatDuration(
                                Date.now() -
                                  new Date(peer.lastHandshake).getTime()
                              ) + " ago"
                            : "Never"}
                        </TableCell>
                        <TableCell>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon">
                                <MoreHorizontal className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem>View Details</DropdownMenuItem>
                              <DropdownMenuItem>Reconnect</DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  )
}
