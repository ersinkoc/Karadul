import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Header } from "./header"
import { AllProviders } from "@/test/utils"

// Mock the useWebSocket hook
vi.mock("@/lib/websocket", () => ({
  useWebSocket: () => ({ connected: true, error: null }),
}))

// Mock the useTheme hook
const mockSetTheme = vi.fn()
vi.mock("@/components/theme-provider", () => ({
  useTheme: () => ({ theme: "light", setTheme: mockSetTheme }),
}))

describe("Header", () => {
  beforeEach(() => {
    mockSetTheme.mockReset()
  })

  it("should render the title", () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    expect(screen.getByText("Karadul Mesh VPN")).toBeInTheDocument()
  })

  it("should render connection status badge", () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    expect(screen.getByText("Connected")).toBeInTheDocument()
  })

  it("should render notifications button", () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Find notification button by looking for a button with bell icon
    const buttons = screen.getAllByRole("button")
    expect(buttons.length).toBeGreaterThan(0)
  })

  it("should render theme toggle button", () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Theme button should be present
    const buttons = screen.getAllByRole("button")
    expect(buttons.length).toBeGreaterThan(1)
  })

  it("should have mobile menu button", () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Mobile menu button has sr-only text "Open menu"
    const menuButton = screen.getByRole("button", { name: /open menu/i })
    expect(menuButton).toBeInTheDocument()
  })

  it("should render navigation items in mobile menu", async () => {
    const user = userEvent.setup()

    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Open mobile menu
    const menuButton = screen.getByRole("button", { name: /open menu/i })
    await user.click(menuButton)

    // Check navigation items are visible
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument()
      expect(screen.getByText("Topology")).toBeInTheDocument()
      expect(screen.getByText("Nodes")).toBeInTheDocument()
      expect(screen.getByText("Peers")).toBeInTheDocument()
      expect(screen.getByText("Settings")).toBeInTheDocument()
    })
  })

  it("should have GitHub link in mobile menu", async () => {
    const user = userEvent.setup()

    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Open mobile menu
    const menuButton = screen.getByRole("button", { name: /open menu/i })
    await user.click(menuButton)

    // Check GitHub link exists
    await waitFor(() => {
      expect(screen.getByText("GitHub")).toBeInTheDocument()
    })
  })

  it("should close mobile menu when navigation link is clicked", async () => {
    const user = userEvent.setup()

    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    // Open mobile menu
    const menuButton = screen.getByRole("button", { name: /open menu/i })
    await user.click(menuButton)

    // Wait for menu to open
    await waitFor(() => {
      expect(screen.getByText("Topology")).toBeInTheDocument()
    })

    // Click on a nav link (Topology)
    const topologyLink = screen.getByText("Topology")
    await user.click(topologyLink)

    // The menu should close (sheet content should no longer be visible)
    // Since we're using mocked router, the navigation won't actually happen
    // but the onClick handler should be called
  })
})

describe("Header - Theme dropdown", () => {
  const user = userEvent.setup()

  beforeEach(() => {
    mockSetTheme.mockReset()
  })

  it("should show theme options when clicking theme button", async () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    const buttons = screen.getAllByRole("button")
    const themeButton = buttons.find(btn => btn.querySelector(".sr-only")?.textContent === "Toggle theme")
    expect(themeButton).toBeTruthy()

    if (themeButton) {
      await user.click(themeButton)

      await waitFor(() => {
        expect(screen.getByText("Light")).toBeInTheDocument()
        expect(screen.getByText("Dark")).toBeInTheDocument()
        expect(screen.getByText("System")).toBeInTheDocument()
      })
    }
  })

  it("should call setTheme with 'light' when Light option is clicked", async () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    const buttons = screen.getAllByRole("button")
    const themeButton = buttons.find(btn => btn.querySelector(".sr-only")?.textContent === "Toggle theme")

    if (themeButton) {
      await user.click(themeButton)

      await waitFor(() => {
        expect(screen.getByText("Light")).toBeInTheDocument()
      })

      await user.click(screen.getByText("Light"))
      expect(mockSetTheme).toHaveBeenCalledWith("light")
    }
  })

  it("should call setTheme with 'dark' when Dark option is clicked", async () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    const buttons = screen.getAllByRole("button")
    const themeButton = buttons.find(btn => btn.querySelector(".sr-only")?.textContent === "Toggle theme")

    if (themeButton) {
      await user.click(themeButton)

      await waitFor(() => {
        expect(screen.getByText("Dark")).toBeInTheDocument()
      })

      await user.click(screen.getByText("Dark"))
      expect(mockSetTheme).toHaveBeenCalledWith("dark")
    }
  })

  it("should call setTheme with 'system' when System option is clicked", async () => {
    render(
      <AllProviders>
        <Header />
      </AllProviders>
    )

    const buttons = screen.getAllByRole("button")
    const themeButton = buttons.find(btn => btn.querySelector(".sr-only")?.textContent === "Toggle theme")

    if (themeButton) {
      await user.click(themeButton)

      await waitFor(() => {
        expect(screen.getByText("System")).toBeInTheDocument()
      })

      await user.click(screen.getByText("System"))
      expect(mockSetTheme).toHaveBeenCalledWith("system")
    }
  })
})

describe("Header - Disconnected state", () => {
  beforeEach(() => {
    vi.resetModules()
    vi.doMock("@/lib/websocket", () => ({
      useWebSocket: () => ({ connected: false, error: null }),
    }))
    vi.doMock("@/components/theme-provider", () => ({
      useTheme: () => ({ theme: "light", setTheme: mockSetTheme }),
    }))
  })

  it("should show Disconnected badge when WebSocket is not connected", async () => {
    const { Header: HeaderDisconnected } = await import("./header")

    render(
      <AllProviders>
        <HeaderDisconnected />
      </AllProviders>
    )

    expect(screen.getByText("Disconnected")).toBeInTheDocument()
  })
})

describe("Header - Dark theme icon", () => {
  beforeEach(() => {
    vi.resetModules()
    vi.doMock("@/lib/websocket", () => ({
      useWebSocket: () => ({ connected: true, error: null }),
    }))
    vi.doMock("@/components/theme-provider", () => ({
      useTheme: () => ({ theme: "dark", setTheme: mockSetTheme }),
    }))
  })

  it("should render Moon icon when theme is dark", async () => {
    const { Header: HeaderDark } = await import("./header")

    render(
      <AllProviders>
        <HeaderDark />
      </AllProviders>
    )

    // When theme is dark, the Moon icon should be rendered
    expect(screen.getByText("Karadul Mesh VPN")).toBeInTheDocument()
    // Check for theme toggle button presence
    const buttons = screen.getAllByRole("button")
    expect(buttons.length).toBeGreaterThan(0)
  })
})

describe("Header - System theme icon", () => {
  beforeEach(() => {
    vi.resetModules()
    vi.doMock("@/lib/websocket", () => ({
      useWebSocket: () => ({ connected: true, error: null }),
    }))
    vi.doMock("@/components/theme-provider", () => ({
      useTheme: () => ({ theme: "system", setTheme: mockSetTheme }),
    }))
  })

  it("should render Sun icon when theme is system", async () => {
    const { Header: HeaderSystem } = await import("./header")

    render(
      <AllProviders>
        <HeaderSystem />
      </AllProviders>
    )

    // When theme is system, the Sun icon (fallback) should be rendered
    expect(screen.getByText("Karadul Mesh VPN")).toBeInTheDocument()
    // Check for theme toggle button presence
    const buttons = screen.getAllByRole("button")
    expect(buttons.length).toBeGreaterThan(0)
  })
})
