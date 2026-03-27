import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import {
  Sheet,
  SheetTrigger,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetDescription,
  SheetClose,
} from "./sheet"
import { AllProviders } from "@/test/utils"

describe("Sheet Components", () => {
  const user = userEvent.setup()

  it("should render Sheet with trigger", () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open Sheet</SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Sheet Title</SheetTitle>
              <SheetDescription>Sheet description</SheetDescription>
            </SheetHeader>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    expect(screen.getByText("Open Sheet")).toBeInTheDocument()
  })

  it("should open sheet when trigger is clicked", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open Sheet</SheetTrigger>
          <SheetContent>
            <SheetTitle>Sheet Title</SheetTitle>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open Sheet"))

    await expect(screen.findByText("Sheet Title")).resolves.toBeInTheDocument()
  })

  it("should render SheetHeader", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetHeader>Header content</SheetHeader>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Header content")).resolves.toBeInTheDocument()
  })

  it("should render SheetFooter", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetFooter>Footer content</SheetFooter>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Footer content")).resolves.toBeInTheDocument()
  })

  it("should render SheetFooter with custom className", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetFooter className="custom-footer">Footer</SheetFooter>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Footer")).resolves.toBeInTheDocument()
  })

  it("should render SheetTitle", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetTitle>Sheet Title</SheetTitle>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Sheet Title")).resolves.toBeInTheDocument()
  })

  it("should render SheetDescription", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetDescription>Description text</SheetDescription>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Description text")).resolves.toBeInTheDocument()
  })

  it("should render SheetDescription with custom className", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open</SheetTrigger>
          <SheetContent>
            <SheetDescription className="custom-desc">Description</SheetDescription>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open"))

    await expect(screen.findByText("Description")).resolves.toBeInTheDocument()
  })

  it("should render complete sheet structure", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open Full Sheet</SheetTrigger>
          <SheetContent side="right">
            <SheetHeader>
              <SheetTitle>Full Sheet</SheetTitle>
              <SheetDescription>This is a complete sheet</SheetDescription>
            </SheetHeader>
            <div>Sheet body content</div>
            <SheetFooter>
              <SheetClose>Close Sheet</SheetClose>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open Full Sheet"))

    await expect(screen.findByText("Full Sheet")).resolves.toBeInTheDocument()
    expect(screen.getByText("This is a complete sheet")).toBeInTheDocument()
    expect(screen.getByText("Sheet body content")).toBeInTheDocument()
    expect(screen.getByText("Close Sheet")).toBeInTheDocument()
  })

  it("should render SheetContent on different sides", async () => {
    render(
      <AllProviders>
        <Sheet>
          <SheetTrigger>Open Left</SheetTrigger>
          <SheetContent side="left">
            <SheetTitle>Left Sheet</SheetTitle>
          </SheetContent>
        </Sheet>
      </AllProviders>
    )

    await user.click(screen.getByText("Open Left"))

    await expect(screen.findByText("Left Sheet")).resolves.toBeInTheDocument()
  })
})
