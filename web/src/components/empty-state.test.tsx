import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { EmptyState } from "./empty-state"
import { AllProviders } from "@/test/utils"
import { Server, Users } from "lucide-react"

describe("EmptyState", () => {
  it("should render with icon, title, and description", () => {
    render(
      <AllProviders>
        <EmptyState
          icon={Server}
          title="No servers found"
          description="Add a server to get started"
        />
      </AllProviders>
    )

    expect(screen.getByText("No servers found")).toBeInTheDocument()
    expect(screen.getByText("Add a server to get started")).toBeInTheDocument()
    // Check that icon is rendered (Lucide icons have specific structure)
    expect(document.querySelector("svg")).toBeInTheDocument()
  })

  it("should render action button when action is provided", () => {
    const onClick = vi.fn()
    render(
      <AllProviders>
        <EmptyState
          icon={Users}
          title="No users"
          description="Add users to begin"
          action={{ label: "Add User", onClick }}
        />
      </AllProviders>
    )

    const button = screen.getByRole("button", { name: /add user/i })
    expect(button).toBeInTheDocument()

    fireEvent.click(button)
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it("should not render action button when action is not provided", () => {
    render(
      <AllProviders>
        <EmptyState
          icon={Server}
          title="No data"
          description="No data available"
        />
      </AllProviders>
    )

    expect(screen.queryByRole("button")).not.toBeInTheDocument()
  })

  it("should center content properly", () => {
    render(
      <AllProviders>
        <EmptyState
          icon={Server}
          title="Centered"
          description="This should be centered"
        />
      </AllProviders>
    )

    const container = screen.getByText("Centered").closest("div")
    expect(container).toHaveClass("flex", "flex-col", "items-center", "justify-center")
  })
})
