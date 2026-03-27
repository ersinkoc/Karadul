import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { ErrorAlert, ConnectionError } from "./error-boundary"
import { AllProviders } from "@/test/utils"

describe("ErrorAlert", () => {
  it("should render with default title", () => {
    render(
      <AllProviders>
        <ErrorAlert message="Something went wrong" />
      </AllProviders>
    )

    expect(screen.getByText("Error")).toBeInTheDocument()
    expect(screen.getByText("Something went wrong")).toBeInTheDocument()
  })

  it("should render with custom title", () => {
    render(
      <AllProviders>
        <ErrorAlert title="Custom Error" message="Custom message" />
      </AllProviders>
    )

    expect(screen.getByText("Custom Error")).toBeInTheDocument()
    expect(screen.getByText("Custom message")).toBeInTheDocument()
  })

  it("should render retry button when onRetry is provided", () => {
    const onRetry = vi.fn()
    render(
      <AllProviders>
        <ErrorAlert message="Error" onRetry={onRetry} />
      </AllProviders>
    )

    const retryButton = screen.getByRole("button", { name: /retry/i })
    expect(retryButton).toBeInTheDocument()

    fireEvent.click(retryButton)
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("should not render retry button when onRetry is not provided", () => {
    render(
      <AllProviders>
        <ErrorAlert message="Error" />
      </AllProviders>
    )

    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument()
  })
})

describe("ConnectionError", () => {
  it("should render connection error message", () => {
    render(
      <AllProviders>
        <ConnectionError />
      </AllProviders>
    )

    expect(screen.getByText("Connection Error")).toBeInTheDocument()
    expect(screen.getByText("Unable to connect to the Karadul API")).toBeInTheDocument()
  })

  it("should render retry button when onRetry is provided", () => {
    const onRetry = vi.fn()
    render(
      <AllProviders>
        <ConnectionError onRetry={onRetry} />
      </AllProviders>
    )

    const retryButton = screen.getByRole("button", { name: /retry connection/i })
    expect(retryButton).toBeInTheDocument()

    fireEvent.click(retryButton)
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it("should not render retry button when onRetry is not provided", () => {
    render(
      <AllProviders>
        <ConnectionError />
      </AllProviders>
    )

    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument()
  })
})
