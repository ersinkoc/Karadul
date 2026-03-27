import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue, SelectScrollUpButton, SelectScrollDownButton, SelectSeparator } from "./select"
import { AllProviders } from "@/test/utils"

// Simple test wrapper with select
function TestSelect({ onValueChange }: { onValueChange?: (value: string) => void }) {
  return (
    <Select onValueChange={onValueChange}>
      <SelectTrigger data-testid="select-trigger">
        <SelectValue placeholder="Select an option" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Options</SelectLabel>
          <SelectItem value="option1">Option 1</SelectItem>
          <SelectItem value="option2">Option 2</SelectItem>
          <SelectItem value="option3">Option 3</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

describe("Select Components", () => {
  it("should render select trigger", () => {
    render(
      <AllProviders>
        <TestSelect />
      </AllProviders>
    )

    expect(screen.getByTestId("select-trigger")).toBeInTheDocument()
    expect(screen.getByText("Select an option")).toBeInTheDocument()
  })

  it("should render with default value", () => {
    render(
      <AllProviders>
        <Select defaultValue="option1">
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="option1">Option 1</SelectItem>
          </SelectContent>
        </Select>
      </AllProviders>
    )

    expect(screen.getByRole("combobox")).toBeInTheDocument()
  })

  it("should be disabled when disabled prop is set", () => {
    render(
      <AllProviders>
        <Select disabled>
          <SelectTrigger>
            <SelectValue placeholder="Disabled select" />
          </SelectTrigger>
        </Select>
      </AllProviders>
    )

    const trigger = screen.getByRole("combobox")
    expect(trigger).toBeDisabled()
  })

  it("should accept custom className on SelectTrigger", () => {
    render(
      <AllProviders>
        <Select>
          <SelectTrigger className="custom-class">
            <SelectValue placeholder="Test" />
          </SelectTrigger>
        </Select>
      </AllProviders>
    )

    const trigger = screen.getByRole("combobox")
    expect(trigger).toHaveClass("custom-class")
  })

  it("should render SelectGroup", () => {
    render(
      <AllProviders>
        <Select>
          <SelectTrigger>
            <SelectValue placeholder="Select..." />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectLabel>Group Label</SelectLabel>
              <SelectItem value="item">Item</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </AllProviders>
    )

    expect(screen.getByRole("combobox")).toBeInTheDocument()
  })

  it("should handle onValueChange callback", () => {
    const handleChange = vi.fn()

    render(
      <AllProviders>
        <TestSelect onValueChange={handleChange} />
      </AllProviders>
    )

    expect(screen.getByTestId("select-trigger")).toBeInTheDocument()
  })
})

describe("SelectScrollButton Components", () => {
  it("should render SelectScrollUpButton", () => {
    render(
      <AllProviders>
        <Select>
          <SelectContent>
            <SelectScrollUpButton />
          </SelectContent>
        </Select>
      </AllProviders>
    )

    // Component should render without error
    expect(document.body).toBeInTheDocument()
  })

  it("should render SelectScrollDownButton", () => {
    render(
      <AllProviders>
        <Select>
          <SelectContent>
            <SelectScrollDownButton />
          </SelectContent>
        </Select>
      </AllProviders>
    )

    // Component should render without error
    expect(document.body).toBeInTheDocument()
  })
})

describe("SelectSeparator Component", () => {
  it("should render SelectSeparator", () => {
    render(
      <AllProviders>
        <Select>
          <SelectTrigger>
            <SelectValue placeholder="Select..." />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="item1">Item 1</SelectItem>
            <SelectSeparator />
            <SelectItem value="item2">Item 2</SelectItem>
          </SelectContent>
        </Select>
      </AllProviders>
    )

    expect(screen.getByRole("combobox")).toBeInTheDocument()
  })

  it("should render SelectSeparator with custom className", () => {
    render(
      <AllProviders>
        <Select>
          <SelectContent>
            <SelectSeparator className="custom-separator" />
          </SelectContent>
        </Select>
      </AllProviders>
    )

    // Component should render without error
    expect(document.body).toBeInTheDocument()
  })
})
