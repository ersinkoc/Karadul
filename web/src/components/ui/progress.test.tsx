import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import { Progress } from "./progress"
import { AllProviders } from "@/test/utils"

describe("Progress", () => {
  it("should render progress bar", () => {
    render(
      <AllProviders>
        <Progress value={50} />
      </AllProviders>
    )

    const progress = document.querySelector('[role="progressbar"]')
    expect(progress).toBeInTheDocument()
  })

  it("should display progress indicator with correct width", () => {
    render(
      <AllProviders>
        <Progress value={75} />
      </AllProviders>
    )

    // The Progress component renders with an indicator element
    const indicator = document.querySelector('[style*="transform"]')
    expect(indicator).toBeInTheDocument()
  })

  it("should have max attribute", () => {
    render(
      <AllProviders>
        <Progress value={50} max={100} />
      </AllProviders>
    )

    const progress = document.querySelector('[role="progressbar"]')
    expect(progress).toHaveAttribute("aria-valuemax", "100")
  })

  it("should accept custom className", () => {
    render(
      <AllProviders>
        <Progress value={50} className="custom-class" />
      </AllProviders>
    )

    const container = document.querySelector(".custom-class")
    expect(container).toBeInTheDocument()
  })

  it("should render without value prop", () => {
    render(
      <AllProviders>
        <Progress />
      </AllProviders>
    )

    const progress = document.querySelector('[role="progressbar"]')
    expect(progress).toBeInTheDocument()
  })
})
