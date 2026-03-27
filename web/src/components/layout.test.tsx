import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { Layout } from "./layout"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

// Mock the Sidebar and Header components
vi.mock("./sidebar", () => ({
  Sidebar: () => <div data-testid="sidebar">Sidebar</div>,
}))

vi.mock("./header", () => ({
  Header: () => <div data-testid="header">Header</div>,
}))

function renderWithRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<div data-testid="outlet-content">Page Content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe("Layout", () => {
  it("should render sidebar", () => {
    renderWithRouter()

    expect(screen.getByTestId("sidebar")).toBeInTheDocument()
  })

  it("should render header", () => {
    renderWithRouter()

    expect(screen.getByTestId("header")).toBeInTheDocument()
  })

  it("should render outlet content", () => {
    renderWithRouter()

    expect(screen.getByTestId("outlet-content")).toBeInTheDocument()
    expect(screen.getByText("Page Content")).toBeInTheDocument()
  })

  it("should have proper layout structure", () => {
    const { container } = renderWithRouter()

    // Main container should have flex and full screen height
    const mainContainer = container.firstChild as HTMLElement
    expect(mainContainer).toHaveClass("flex", "h-screen", "w-full")
  })
})
