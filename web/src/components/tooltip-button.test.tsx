import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { TooltipButton } from "./tooltip-button"
import { AllProviders } from "@/test/utils"
import { Settings } from "lucide-react"

// Mock the tooltip component since it uses Radix UI with portals
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <div data-testid="tooltip">{children}</div>,
  TooltipTrigger: ({ children, asChild }: { children: React.ReactNode; asChild?: boolean }) => (
    <div data-testid="tooltip-trigger">{children}</div>
  ),
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="tooltip-content">{children}</div>
  ),
  TooltipProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="tooltip-provider">{children}</div>
  ),
}))

describe("TooltipButton", () => {
  it("should render the button with children", () => {
    render(
      <AllProviders>
        <TooltipButton tooltip="Test tooltip">
          <Settings className="h-4 w-4" />
        </TooltipButton>
      </AllProviders>
    )

    // The button is rendered inside the tooltip trigger
    expect(screen.getByRole("button")).toBeInTheDocument()
    expect(document.querySelector("svg")).toBeInTheDocument()
  })

  it("should pass tooltip prop to TooltipContent", () => {
    render(
      <AllProviders>
        <TooltipButton tooltip="Test tooltip text">
          <span>Button Label</span>
        </TooltipButton>
      </AllProviders>
    )

    // The tooltip text should be in the tooltip content
    expect(screen.getByText("Test tooltip text")).toBeInTheDocument()
    // The button label should also be present
    expect(screen.getByText("Button Label")).toBeInTheDocument()
  })

  it("should pass button props through", () => {
    const onClick = vi.fn()

    render(
      <AllProviders>
        <TooltipButton tooltip="Test" variant="destructive" size="lg" onClick={onClick}>
          <span>Click me</span>
        </TooltipButton>
      </AllProviders>
    )

    // The button should be clickable
    const button = screen.getByRole("button")
    expect(button).toBeInTheDocument()
    fireEvent.click(button)
    expect(onClick).toHaveBeenCalled()
  })

  it("should apply custom className", () => {
    render(
      <AllProviders>
        <TooltipButton tooltip="Test" className="custom-class">
          <span>Content</span>
        </TooltipButton>
      </AllProviders>
    )

    const button = screen.getByRole("button")
    expect(button).toHaveClass("custom-class")
  })

  it("should render tooltip provider", () => {
    render(
      <AllProviders>
        <TooltipButton tooltip="Test">
          <span>Content</span>
        </TooltipButton>
      </AllProviders>
    )

    expect(screen.getByTestId("tooltip-provider")).toBeInTheDocument()
  })
})
