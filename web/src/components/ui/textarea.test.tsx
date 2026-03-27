import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { Textarea } from "./textarea"
import { AllProviders } from "@/test/utils"

describe("Textarea", () => {
  it("should render textarea component", () => {
    render(
      <AllProviders>
        <Textarea />
      </AllProviders>
    )

    expect(screen.getByRole("textbox")).toBeInTheDocument()
  })

  it("should accept placeholder", () => {
    render(
      <AllProviders>
        <Textarea placeholder="Enter text..." />
      </AllProviders>
    )

    expect(screen.getByPlaceholderText("Enter text...")).toBeInTheDocument()
  })

  it("should accept value", () => {
    render(
      <AllProviders>
        <Textarea value="Initial value" readOnly />
      </AllProviders>
    )

    expect(screen.getByDisplayValue("Initial value")).toBeInTheDocument()
  })

  it("should handle change events", () => {
    const onChange = vi.fn()

    render(
      <AllProviders>
        <Textarea onChange={onChange} />
      </AllProviders>
    )

    const textarea = screen.getByRole("textbox")
    fireEvent.change(textarea, { target: { value: "New value" } })

    expect(onChange).toHaveBeenCalled()
  })

  it("should accept custom className", () => {
    render(
      <AllProviders>
        <Textarea className="custom-textarea" />
      </AllProviders>
    )

    const textarea = screen.getByRole("textbox")
    expect(textarea).toHaveClass("custom-textarea")
  })

  it("should be disabled when disabled prop is true", () => {
    render(
      <AllProviders>
        <Textarea disabled />
      </AllProviders>
    )

    const textarea = screen.getByRole("textbox")
    expect(textarea).toBeDisabled()
  })
})
