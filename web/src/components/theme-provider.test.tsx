import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, renderHook } from "@testing-library/react"
import { ThemeProvider, useTheme } from "./theme-provider"


// Helper component to test the hook
function ThemeConsumer() {
  const { theme, setTheme } = useTheme()
  return (
    <div>
      <span data-testid="current-theme">{theme}</span>
      <button onClick={() => setTheme("dark")}>Set Dark</button>
      <button onClick={() => setTheme("light")}>Set Light</button>
      <button onClick={() => setTheme("system")}>Set System</button>
    </div>
  )
}

function renderWithProvider(defaultTheme?: "dark" | "light" | "system") {
  return render(
    <ThemeProvider defaultTheme={defaultTheme}>
      <ThemeConsumer />
    </ThemeProvider>
  )
}

describe("ThemeProvider", () => {
  const localStorageMock = {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
    length: 0,
    key: vi.fn(),
  }

  beforeEach(() => {
    vi.stubGlobal("localStorage", localStorageMock)
    localStorageMock.getItem.mockReturnValue(null)
    document.documentElement.classList.remove("light", "dark")
  })

  afterEach(() => {
    vi.clearAllMocks()
    document.documentElement.classList.remove("light", "dark")
  })

  it("should provide default theme as system when not specified", () => {
    renderWithProvider()

    expect(screen.getByTestId("current-theme")).toHaveTextContent("system")
  })

  it("should use provided default theme", () => {
    renderWithProvider("dark")

    expect(screen.getByTestId("current-theme")).toHaveTextContent("dark")
  })

  it("should read theme from localStorage if available", () => {
    localStorageMock.getItem.mockReturnValue("light")

    renderWithProvider()

    expect(screen.getByTestId("current-theme")).toHaveTextContent("light")
  })

  it("should set theme and save to localStorage", () => {
    renderWithProvider()

    act(() => {
      screen.getByText("Set Dark").click()
    })

    expect(screen.getByTestId("current-theme")).toHaveTextContent("dark")
    expect(localStorageMock.setItem).toHaveBeenCalledWith("vite-ui-theme", "dark")
  })

  it("should apply dark class to document when theme is dark", () => {
    renderWithProvider()

    act(() => {
      screen.getByText("Set Dark").click()
    })

    expect(document.documentElement.classList.contains("dark")).toBe(true)
  })

  it("should apply light class to document when theme is light", () => {
    renderWithProvider()

    act(() => {
      screen.getByText("Set Light").click()
    })

    expect(document.documentElement.classList.contains("light")).toBe(true)
  })

  it("should remove previous theme class when changing theme", () => {
    renderWithProvider()

    act(() => {
      screen.getByText("Set Dark").click()
    })
    expect(document.documentElement.classList.contains("dark")).toBe(true)

    act(() => {
      screen.getByText("Set Light").click()
    })
    expect(document.documentElement.classList.contains("dark")).toBe(false)
    expect(document.documentElement.classList.contains("light")).toBe(true)
  })

  it("should use custom storage key when provided", () => {
    const customKey = "custom-theme-key"

    render(
      <ThemeProvider storageKey={customKey}>
        <ThemeConsumer />
      </ThemeProvider>
    )

    act(() => {
      screen.getByText("Set Dark").click()
    })

    expect(localStorageMock.setItem).toHaveBeenCalledWith(customKey, "dark")
  })
})

describe("useTheme error handling", () => {
  it("should return initial state when used without provider", () => {
    // Since ThemeProviderContext has a default initial state,
    // useTheme won't throw but will return the default state
    const { result } = renderHook(() => useTheme())

    expect(result.current.theme).toBe("system")
  })
})
