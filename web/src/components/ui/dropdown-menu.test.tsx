import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuRadioGroup,
  DropdownMenuCheckboxItem,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "./dropdown-menu"
import { AllProviders } from "@/test/utils"

describe("DropdownMenu components", () => {
  const user = userEvent.setup()

  describe("DropdownMenuRadioItem", () => {
    it("should render radio item", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuRadioGroup value="option1">
                <DropdownMenuRadioItem value="option1">Option 1</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Option 1")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuCheckboxItem", () => {
    it("should render checkbox item", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuCheckboxItem checked>Checked Item</DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Checked Item")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuLabel", () => {
    it("should render label", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuLabel>Label</DropdownMenuLabel>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Label")).resolves.toBeInTheDocument()
    })

    it("should render label with inset", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuLabel inset>Inset Label</DropdownMenuLabel>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Inset Label")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuSeparator", () => {
    it("should render separator", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem>Item 1</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem>Item 2</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Item 1")).resolves.toBeInTheDocument()
      await expect(screen.findByText("Item 2")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuShortcut", () => {
    it("should render shortcut text", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem>
                Copy
                <DropdownMenuShortcut>⌘C</DropdownMenuShortcut>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("⌘C")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuSub", () => {
    it("should render sub menu", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>More Options</DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuItem>Sub Item</DropdownMenuItem>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("More Options")).resolves.toBeInTheDocument()
    })

    it("should render DropdownMenuSubTrigger with inset", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger inset>Sub Menu</DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuItem>Sub Item</DropdownMenuItem>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Sub Menu")).resolves.toBeInTheDocument()
    })
  })

  describe("DropdownMenuItem with inset", () => {
    it("should render DropdownMenuItem with inset prop", async () => {
      render(
        <AllProviders>
          <DropdownMenu>
            <DropdownMenuTrigger>Open</DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem inset>Item with inset</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </AllProviders>
      )

      await user.click(screen.getByText("Open"))
      await expect(screen.findByText("Item with inset")).resolves.toBeInTheDocument()
    })
  })
})
