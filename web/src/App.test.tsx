import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import App from "./App"

// Mock all the page components
vi.mock("./pages/dashboard", () => ({ DashboardPage: () => <div data-testid="dashboard-page">Dashboard</div> }))
vi.mock("./pages/topology", () => ({ TopologyPage: () => <div data-testid="topology-page">Topology</div> }))
vi.mock("./pages/nodes", () => ({ NodesPage: () => <div data-testid="nodes-page">Nodes</div> }))
vi.mock("./pages/peers", () => ({ PeersPage: () => <div data-testid="peers-page">Peers</div> }))
vi.mock("./pages/settings", () => ({ SettingsPage: () => <div data-testid="settings-page">Settings</div> }))
vi.mock("./pages/not-found", () => ({ NotFoundPage: () => <div data-testid="not-found-page">Not Found</div> }))

// Mock the layout component
vi.mock("./components/layout", () => ({
  Layout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="layout">
      <div data-testid="layout-children">{children}</div>
    </div>
  ),
}))

// Mock WebSocket provider
vi.mock("./lib/websocket", () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => <div data-testid="ws-provider">{children}</div>,
}))

// Mock sonner toaster
vi.mock("@/components/ui/sonner", () => ({
  Toaster: () => <div data-testid="toaster" />,
}))

describe("App", () => {
  it("should render without crashing", () => {
    render(<App />)

    // Check that the app renders the layout
    expect(screen.getByTestId("layout")).toBeInTheDocument()
  })

  it("should render the toaster component", () => {
    render(<App />)

    expect(screen.getByTestId("toaster")).toBeInTheDocument()
  })

  it("should wrap app with WebSocket provider", () => {
    render(<App />)

    expect(screen.getByTestId("ws-provider")).toBeInTheDocument()
  })

  it("should render layout children container", () => {
    render(<App />)

    expect(screen.getByTestId("layout-children")).toBeInTheDocument()
  })
})
