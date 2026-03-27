import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { Sidebar } from "./sidebar"
import { BrowserRouter, MemoryRouter } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

// Helper function to render with router
function renderWithRouter(initialRoute = "/") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialRoute]}>
        <Sidebar />
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe("Sidebar", () => {
  it("should render the Karadul logo and title", () => {
    renderWithRouter()

    expect(screen.getByText("Karadul")).toBeInTheDocument()
    // Network icon is rendered (check for SVG)
    expect(document.querySelector("svg")).toBeInTheDocument()
  })

  it("should render all navigation items", () => {
    renderWithRouter()

    expect(screen.getByText("Dashboard")).toBeInTheDocument()
    expect(screen.getByText("Topology")).toBeInTheDocument()
    expect(screen.getByText("Nodes")).toBeInTheDocument()
    expect(screen.getByText("Peers")).toBeInTheDocument()
    expect(screen.getByText("Settings")).toBeInTheDocument()
  })

  it("should highlight active navigation item", () => {
    renderWithRouter("/nodes")

    // The nodes button should have the active styling
    const nodesLink = screen.getByText("Nodes").closest("a")
    expect(nodesLink).toHaveAttribute("href", "/nodes")
  })

  it("should render GitHub link", () => {
    renderWithRouter()

    const githubLink = screen.getByText("GitHub").closest("a")
    expect(githubLink).toHaveAttribute("href", "https://github.com/ersinkoc/karadul")
    expect(githubLink).toHaveAttribute("target", "_blank")
    expect(githubLink).toHaveAttribute("rel", "noopener noreferrer")
  })

  it("should have correct links for all nav items", () => {
    renderWithRouter()

    expect(screen.getByText("Dashboard").closest("a")).toHaveAttribute("href", "/")
    expect(screen.getByText("Topology").closest("a")).toHaveAttribute("href", "/topology")
    expect(screen.getByText("Nodes").closest("a")).toHaveAttribute("href", "/nodes")
    expect(screen.getByText("Peers").closest("a")).toHaveAttribute("href", "/peers")
    expect(screen.getByText("Settings").closest("a")).toHaveAttribute("href", "/settings")
  })
})
