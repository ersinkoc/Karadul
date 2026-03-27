import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import { DashboardSkeleton } from "./loading-skeletons"
import { AllProviders } from "@/test/utils"

describe("DashboardSkeleton", () => {
  it("should render the skeleton component", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // Check that multiple skeletons are rendered
    const skeletons = document.querySelectorAll(".animate-pulse")
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it("should render title skeleton", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // Title skeleton should be present (h-10 w-[200px])
    const titleSkeleton = document.querySelector(".h-10.w-\\[200px\\]")
    expect(titleSkeleton).toBeInTheDocument()
  })

  it("should render subtitle skeleton", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // Subtitle skeleton should be present (h-4 w-[300px])
    const subtitleSkeleton = document.querySelector(".h-4.w-\\[300px\\]")
    expect(subtitleSkeleton).toBeInTheDocument()
  })

  it("should render 4 stat card skeletons", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // 4 stat cards in the grid
    const cards = document.querySelectorAll(".rounded-lg.border")
    expect(cards.length).toBe(6) // 4 stat cards + 2 chart cards
  })

  it("should render chart card skeletons", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // Chart skeleton should be present (h-[200px])
    const chartSkeleton = document.querySelector(".h-\\[200px\\]")
    expect(chartSkeleton).toBeInTheDocument()
  })

  it("should render progress bar skeletons in system status card", () => {
    render(
      <AllProviders>
        <DashboardSkeleton />
      </AllProviders>
    )

    // Progress bar skeletons should be present (h-2 w-full)
    const progressBars = document.querySelectorAll(".h-2.w-full")
    expect(progressBars.length).toBeGreaterThan(0)
  })
})
