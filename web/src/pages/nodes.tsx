import { useState } from "react"
import {
  Server,
  MoreHorizontal,
  Trash2,
  Power,
  PowerOff,
  ExternalLink,
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
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useNodes, useDeleteNode } from "@/lib/api"
import { useKaradulStore } from "@/lib/store"
import { formatBytes, formatDate, cn } from "@/lib/utils"
import { ErrorAlert } from "@/components/error-boundary"
import { EmptyState } from "@/components/empty-state"
import { toast } from "sonner"
import { exportNodesCSV, exportNodesJSON } from "@/lib/export"
import type { Node } from "@/lib/store"

function StatusBadge({ status }: { status: Node["status"] }) {
  const variants = {
    online: "default",
    offline: "secondary",
    pending: "outline",
  } as const

  return (
    <Badge variant={variants[status]} className="gap-1">
      {status === "online" ? (
        <Power className="h-3 w-3" />
      ) : status === "offline" ? (
        <PowerOff className="h-3 w-3" />
      ) : null}
      {status}
    </Badge>
  )
}

function NodeSkeleton() {
  return (
    <TableRow>
      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
      <TableCell><Skeleton className="h-4 w-12" /></TableCell>
      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
      <TableCell><Skeleton className="h-8 w-8" /></TableCell>
    </TableRow>
  )
}

export function NodesPage() {
  const { data: nodes, isLoading, error, refetch } = useNodes()
  const deleteNode = useDeleteNode()
  const [nodeToDelete, setNodeToDelete] = useState<Node | null>(null)
  const [searchQuery, setSearchQuery] = useState("")
  const setSelectedNode = useKaradulStore((state) => state.setSelectedNode)
  const selectedNode = useKaradulStore((state) => state.selectedNode)

  const filteredNodes = nodes?.filter((node) =>
    node.hostname.toLowerCase().includes(searchQuery.toLowerCase()) ||
    node.virtualIP.includes(searchQuery) ||
    node.publicKey.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const handleDelete = async () => {
    if (nodeToDelete) {
      try {
        await deleteNode.mutateAsync(nodeToDelete.id)
        toast.success(`Node "${nodeToDelete.hostname}" deleted successfully`)
        setNodeToDelete(null)
      } catch (err) {
        toast.error(`Failed to delete node: ${err instanceof Error ? err.message : "Unknown error"}`)
      }
    }
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Nodes</h2>
          <p className="text-muted-foreground">
            Manage your mesh network nodes
          </p>
        </div>
        <ErrorAlert
          title="Failed to load nodes"
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
          <h2 className="text-3xl font-bold tracking-tight">Nodes</h2>
          <p className="text-muted-foreground">
            Manage your mesh network nodes
          </p>
        </div>
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" disabled={!nodes || nodes.length === 0}>
                <Download className="h-4 w-4 mr-2" />
                Export
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem onClick={() => exportNodesCSV(nodes || [])}>
                <FileSpreadsheet className="h-4 w-4 mr-2" />
                Export as CSV
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => exportNodesJSON(nodes || [])}>
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

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Node List</CardTitle>
                <CardDescription>All nodes in your mesh network</CardDescription>
              </div>
              <div className="relative w-64">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search nodes..."
                  className="pl-8"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Hostname</TableHead>
                    <TableHead>Virtual IP</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Exit Node</TableHead>
                    <TableHead>Last Seen</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {Array.from({ length: 5 }).map((_, i) => (
                    <NodeSkeleton key={i} />
                  ))}
                </TableBody>
              </Table>
            ) : filteredNodes?.length === 0 ? (
              <EmptyState
                icon={Server}
                title="No nodes found"
                description={
                  searchQuery
                    ? "No nodes match your search criteria. Try a different query."
                    : "Your mesh network doesn't have any nodes yet. Add a node to get started."
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
                    <TableHead>Status</TableHead>
                    <TableHead>Version</TableHead>
                    <TableHead>Last Seen</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(filteredNodes ?? []).map((node) => (
                    <TableRow
                      key={node.id}
                      className={
                        selectedNode?.id === node.id ? "bg-muted" : ""
                      }
                    >
                      <TableCell className="font-medium">
                        <div className="flex items-center gap-2">
                          <Server className="h-4 w-4 text-muted-foreground" />
                          {node.hostname}
                          {node.isExitNode && (
                            <Badge variant="outline" className="text-xs">
                              Exit
                            </Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>{node.virtualIP}</TableCell>
                      <TableCell>
                        <StatusBadge status={node.status} />
                      </TableCell>
                      <TableCell>{node.isExitNode ? "Yes" : "No"}</TableCell>
                      <TableCell>
                        {node.lastSeen
                          ? formatDate(node.lastSeen)
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
                            <DropdownMenuLabel>Actions</DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              onClick={() => setSelectedNode(node)}
                            >
                              <ExternalLink className="mr-2 h-4 w-4" />
                              View Details
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => setNodeToDelete(node)}
                              className="text-red-600"
                            >
                              <Trash2 className="mr-2 h-4 w-4" />
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Node Details</CardTitle>
            <CardDescription>
              {selectedNode
                ? selectedNode.hostname
                : "Select a node to view details"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {selectedNode ? (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div className="text-muted-foreground">ID</div>
                  <div className="font-mono text-xs truncate">
                    {selectedNode.id}
                  </div>

                  <div className="text-muted-foreground">Public Key</div>
                  <div className="font-mono text-xs truncate">
                    {selectedNode.publicKey}
                  </div>

                  <div className="text-muted-foreground">Endpoint</div>
                  <div>{selectedNode.endpoint || "N/A"}</div>

                  <div className="text-muted-foreground">Exit Node</div>
                  <div>{selectedNode.isExitNode ? "Yes" : "No"}</div>

                  <div className="text-muted-foreground">Data Received</div>
                  <div>{formatBytes(selectedNode.rxBytes || 0)}</div>

                  <div className="text-muted-foreground">Data Sent</div>
                  <div>{formatBytes(selectedNode.txBytes || 0)}</div>

                  {selectedNode.routes &&
                    selectedNode.routes.length > 0 && (
                      <>
                        <div className="text-muted-foreground">
                          Advertised Routes
                        </div>
                        <div>
                          {selectedNode.routes.map((route) => (
                            <Badge
                              key={route}
                              variant="outline"
                              className="mr-1"
                            >
                              {route}
                            </Badge>
                          ))}
                        </div>
                      </>
                    )}
                </div>
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                Click on a node to view its details
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <Dialog open={!!nodeToDelete} onOpenChange={() => setNodeToDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Node</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete {nodeToDelete?.hostname}? This
              action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNodeToDelete(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteNode.isPending}
            >
              {deleteNode.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}