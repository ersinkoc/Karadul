import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import { Separator } from "./separator"
import { AllProviders } from "@/test/utils"

describe("Separator", () => {
  it("should render horizontal separator by default", () => {
    const { container } = render(
      <AllProviders>
        <Separator />
      </AllProviders>
    )

    const separator = container.querySelector('[role="none"]') || container.querySelector('[role="separator"]')
    expect(separator).toBeInTheDocument()
  })

  it("should render vertical separator when orientation is vertical", () => {
    const { container } = render(
      <AllProviders>
        <Separator orientation="vertical" />
      </AllProviders>
    )

    const separator = container.querySelector('[role="none"]') || container.querySelector('[role="separator"]')
    expect(separator).toBeInTheDocument()
  })

  it("should render with custom className", () => {
    const { container } = render(
      <AllProviders>
        <Separator className="custom-separator" />
      </AllProviders>
    )

    expect(container.querySelector(".custom-separator")).toBeInTheDocument()
  })

  it("should render as decorative by default", () => {
    const { container } = render(
      <AllProviders>
        <Separator />
      </AllProviders>
    )

    // Decorative separators have role="none"
    const separator = container.querySelector('[role="none"]')
    expect(separator).toBeInTheDocument()
  })

  it("should render as non-decorative when decorative is false", () => {
    const { container } = render(
      <AllProviders>
        <Separator decorative={false} />
      </AllProviders>
    )

    // Non-decorative separators have role="separator"
    const separator = container.querySelector('[role="separator"]')
    expect(separator).toBeInTheDocument()
  })
})
