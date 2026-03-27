import { useCallback, useMemo } from "react"
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  Node,
  Edge,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { Server, Globe, ArrowRightLeft } from "lucide-react"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useTopology, useNodes } from "@/lib/api"
import { useKaradulStore } from "@/lib/store"
import { Badge } from "@/components/ui/badge"
import { ErrorAlert } from "@/components/error-boundary"

type MeshNodeData = {
  label: string
  status: string
  virtualIP: string
  isExitNode?: boolean
}

function MeshNode({ data }: { data: MeshNodeData }) {
  return (
    <Card className="p-3 min-w-[150px] border-2 border-primary/20">
      <div className="flex items-center gap-2">
        {data.isExitNode ? (
          <Globe className="h-5 w-5 text-blue-500" />
        ) : (
          <Server className="h-5 w-5 text-green-500" />
        )}
        <div className="flex-1 min-w-0">
          <p className="font-medium text-sm truncate">{data.label}</p>
          <p className="text-xs text-muted-foreground">{data.virtualIP}</p>
        </div>
      </div>
      <div className="mt-2">
        <Badge
          variant={data.status === "online" ? "default" : "secondary"}
          className="text-xs"
        >
          {data.status}
        </Badge>
        {data.isExitNode && (
          <Badge variant="outline" className="text-xs ml-1">
            Exit
          </Badge>
        )}
      </div>
    </Card>
  )
}

const nodeTypes = {
  meshNode: MeshNode,
}

export function TopologyPage() {
  const { data: topology, isLoading, error, refetch } = useTopology()
  const { data: nodes } = useNodes()
  const storeTopology = useKaradulStore((state) => state.topology)

  const activeTopology = topology || storeTopology

  const initialNodes: Node<MeshNodeData>[] = useMemo(() => {
    if (!activeTopology?.nodes) return []
    return activeTopology.nodes.map((node, index) => ({
      id: node.id,
      type: "meshNode",
      position: {
        x: 200 + (index % 3) * 250,
        y: 150 + Math.floor(index / 3) * 200,
      },
      data: {
        label: node.hostname,
        status: node.status,
        virtualIP: node.virtualIP,
        isExitNode: node.isExitNode,
      },
    }))
  }, [activeTopology])

  const initialEdges: Edge[] = useMemo(() => {
    if (!activeTopology?.connections) return []
    return activeTopology.connections.map((conn) => ({
      id: `e${conn.from}-${conn.to}`,
      source: conn.from,
      target: conn.to,
      type: conn.type === "relay" ? "dashed" : "default",
      animated: true,
      style: {
        stroke: conn.type === "direct" ? "#22c55e" : "#f59e0b",
        strokeWidth: 2,
      },
      label: conn.type === "direct" ? "Direct" : "Relay",
    }))
  }, [activeTopology])

  const [flowNodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [flowEdges, setEdges, onEdgesChange] = useEdgesState(initialEdges)

  // Update nodes/edges when topology changes
  useMemo(() => {
    setNodes(initialNodes)
    setEdges(initialEdges)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialNodes, initialEdges])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    const selectedNode = nodes?.find((n) => n.id === node.id)
    if (selectedNode) {
      useKaradulStore.getState().setSelectedNode(selectedNode)
    }
  }, [nodes])

  if (error) {
    return (
      <div className="space-y-4">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Network Topology</h2>
          <p className="text-muted-foreground">
            Visual representation of your mesh network
          </p>
        </div>
        <ErrorAlert
          title="Failed to load topology"
          message={error.message}
          onRetry={() => refetch()}
        />
      </div>
    )
  }

  return (
    <div className="space-y-4 h-full">
      <div>
        <h2 className="text-3xl font-bold tracking-tight">Network Topology</h2>
        <p className="text-muted-foreground">
          Visual representation of your mesh network
        </p>
      </div>

      <Card className="h-[calc(100vh-220px)]">
        <div className="absolute top-4 left-4 z-10 flex gap-4">
          <div className="flex items-center gap-2 text-xs bg-background/80 backdrop-blur-sm px-2 py-1 rounded">
            <div className="w-3 h-3 rounded-full bg-green-500" />
            <span>Direct</span>
          </div>
          <div className="flex items-center gap-2 text-xs bg-background/80 backdrop-blur-sm px-2 py-1 rounded">
            <div className="w-3 h-3 rounded-full bg-amber-500" />
            <span>Relay</span>
          </div>
          <div className="flex items-center gap-2 text-xs bg-background/80 backdrop-blur-sm px-2 py-1 rounded">
            <ArrowRightLeft className="h-3 w-3 text-blue-500" />
            <span>Exit Node</span>
          </div>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center space-y-4">
              <Skeleton className="h-12 w-12 rounded-full mx-auto" />
              <Skeleton className="h-4 w-32 mx-auto" />
            </div>
          </div>
        ) : (
          <ReactFlow
            nodes={flowNodes}
            edges={flowEdges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onNodeClick={onNodeClick}
            nodeTypes={nodeTypes}
            fitView
            attributionPosition="bottom-right"
          >
            <Background />
            <Controls />
            <MiniMap
              nodeStrokeWidth={3}
              zoomable
              pannable
            />
          </ReactFlow>
        )}
      </Card>
    </div>
  )
}
