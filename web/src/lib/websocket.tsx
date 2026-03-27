import {
  ReactNode,
  useState,
  createContext,
  useContext,
  useEffect,
  useRef,
  useCallback,
} from "react"
import { useKaradulStore } from "@/lib/store"

interface WebSocketContextType {
  connected: boolean
  error: string | null
}

const WebSocketContext = createContext<WebSocketContextType>({
  connected: false,
  error: null,
})

export function useWebSocket() {
  return useContext(WebSocketContext)
}

interface WebSocketProviderProps {
  children: ReactNode
  url?: string
}

export function WebSocketProvider({
  children,
  url = `ws://${window.location.host}/ws`,
}: WebSocketProviderProps) {
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mountedRef = useRef(true)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const setNodes = useKaradulStore((state) => state.setNodes)
  const setPeers = useKaradulStore((state) => state.setPeers)
  const setTopology = useKaradulStore((state) => state.setTopology)
  const setStats = useKaradulStore((state) => state.setStats)

  const connect = useCallback(() => {
    let ws: WebSocket
    try {
      ws = new WebSocket(url)
    } catch {
      if (!mountedRef.current) return null
      setError("Failed to connect to WebSocket")
      reconnectTimerRef.current = setTimeout(connect, 3000)
      return null
    }

    ws.onopen = () => {
      if (mountedRef.current) {
        setConnected(true)
        setError(null)
      }
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)

        switch (message.type) {
          case "nodes":
            setNodes(message.data)
            break
          case "peers":
            setPeers(message.data)
            break
          case "topology":
            setTopology(message.data)
            break
          case "stats":
            setStats(message.data)
            break
          default:
            console.warn("Unknown message type:", message.type)
        }
      } catch (err) {
        console.error("Failed to parse WebSocket message:", err)
      }
    }

    ws.onclose = () => {
      if (!mountedRef.current) return
      setConnected(false)
      reconnectTimerRef.current = setTimeout(connect, 3000)
    }

    ws.onerror = () => {
      if (!mountedRef.current) return
      setError("WebSocket connection failed")
      setConnected(false)
      ws.close()
    }

    return ws
  }, [url, setNodes, setPeers, setTopology, setStats])

  useEffect(() => {
    mountedRef.current = true
    const ws = connect()

    return () => {
      mountedRef.current = false
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
      }
      if (ws) ws.close()
    }
  }, [connect])

  return (
    <WebSocketContext.Provider value={{ connected, error }}>
      {children}
    </WebSocketContext.Provider>
  )
}
