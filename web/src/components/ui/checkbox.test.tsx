import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { Checkbox } from "./checkbox"
import { AllProviders } from "@/test/utils"

describe("Checkbox", () => {
  it("should render checkbox", () => {
    render(
      <AllProviders>
        <Checkbox />
      </AllProviders>
    )

    expect(screen.getByRole("checkbox")).toBeInTheDocument()
  })

  it("should be clickable", () => {
    const onCheckedChange = vi.fn()

    render(
      <AllProviders>
        <Checkbox onCheckedChange={onCheckedChange} />
      </AllProviders>
    )

    const checkbox = screen.getByRole("checkbox")
    fireEvent.click(checkbox)

    expect(onCheckedChange).toHaveBeenCalled()
  })

  it("should render with custom id", () => {
    render(
      <AllProviders>
        <Checkbox id="test-checkbox" />
      </AllProviders>
    )

    const checkbox = screen.getByRole("checkbox")
    expect(checkbox).toHaveAttribute("id", "test-checkbox")
  })

  it("should be disabled when disabled prop is true", () => {
    render(
      <AllProviders>
        <Checkbox disabled />
      </AllProviders>
    )

    const checkbox = screen.getByRole("checkbox")
    expect(checkbox).toBeDisabled()
  })

  it("should render checked by default", () => {
    render(
      <AllProviders>
        <Checkbox defaultChecked />
      </AllProviders>
    )

    const checkbox = screen.getByRole("checkbox")
    expect(checkbox).toBeChecked()
  })
})
