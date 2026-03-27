import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { Switch } from "./switch"
import { AllProviders } from "@/test/utils"

describe("Switch", () => {
  it("should render switch component", () => {
    render(
      <AllProviders>
        <Switch />
      </AllProviders>
    )

    const switchElement = screen.getByRole("switch")
    expect(switchElement).toBeInTheDocument()
  })

  it("should be clickable", () => {
    const onCheckedChange = vi.fn()

    render(
      <AllProviders>
        <Switch onCheckedChange={onCheckedChange} />
      </AllProviders>
    )

    const switchElement = screen.getByRole("switch")
    fireEvent.click(switchElement)

    expect(onCheckedChange).toHaveBeenCalled()
  })

  it("should be disabled when disabled prop is true", () => {
    render(
      <AllProviders>
        <Switch disabled />
      </AllProviders>
    )

    const switchElement = screen.getByRole("switch")
    expect(switchElement).toBeDisabled()
  })

  it("should be checked by default when defaultChecked is true", () => {
    render(
      <AllProviders>
        <Switch defaultChecked />
      </AllProviders>
    )

    const switchElement = screen.getByRole("switch")
    expect(switchElement).toBeChecked()
  })

  it("should accept custom id", () => {
    render(
      <AllProviders>
        <Switch id="test-switch" />
      </AllProviders>
    )

    const switchElement = screen.getByRole("switch")
    expect(switchElement).toHaveAttribute("id", "test-switch")
  })
})
