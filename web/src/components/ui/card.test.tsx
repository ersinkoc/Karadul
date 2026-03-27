import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardDescription,
  CardContent,
} from "./card"
import { AllProviders } from "@/test/utils"

describe("Card components", () => {
  it("should render Card", () => {
    const { container } = render(
      <AllProviders>
        <Card>Card content</Card>
      </AllProviders>
    )

    expect(screen.getByText("Card content")).toBeInTheDocument()
    expect(container.querySelector(".rounded-lg")).toBeInTheDocument()
  })

  it("should render Card with custom className", () => {
    const { container } = render(
      <AllProviders>
        <Card className="custom-class">Card content</Card>
      </AllProviders>
    )

    expect(container.querySelector(".custom-class")).toBeInTheDocument()
  })

  it("should render CardHeader", () => {
    render(
      <AllProviders>
        <CardHeader>Header content</CardHeader>
      </AllProviders>
    )

    expect(screen.getByText("Header content")).toBeInTheDocument()
  })

  it("should render CardTitle", () => {
    render(
      <AllProviders>
        <CardTitle>Title</CardTitle>
      </AllProviders>
    )

    expect(screen.getByText("Title")).toBeInTheDocument()
    expect(screen.getByRole("heading", { level: 3 })).toBeInTheDocument()
  })

  it("should render CardDescription", () => {
    render(
      <AllProviders>
        <CardDescription>Description text</CardDescription>
      </AllProviders>
    )

    expect(screen.getByText("Description text")).toBeInTheDocument()
  })

  it("should render CardContent", () => {
    render(
      <AllProviders>
        <CardContent>Content here</CardContent>
      </AllProviders>
    )

    expect(screen.getByText("Content here")).toBeInTheDocument()
  })

  it("should render CardFooter", () => {
    render(
      <AllProviders>
        <CardFooter>Footer content</CardFooter>
      </AllProviders>
    )

    expect(screen.getByText("Footer content")).toBeInTheDocument()
  })

  it("should render CardFooter with custom className", () => {
    const { container } = render(
      <AllProviders>
        <CardFooter className="footer-custom">Footer</CardFooter>
      </AllProviders>
    )

    expect(container.querySelector(".footer-custom")).toBeInTheDocument()
  })

  it("should render a complete card structure", () => {
    render(
      <AllProviders>
        <Card>
          <CardHeader>
            <CardTitle>Card Title</CardTitle>
            <CardDescription>Card description</CardDescription>
          </CardHeader>
          <CardContent>Card content</CardContent>
          <CardFooter>Card footer</CardFooter>
        </Card>
      </AllProviders>
    )

    expect(screen.getByText("Card Title")).toBeInTheDocument()
    expect(screen.getByText("Card description")).toBeInTheDocument()
    expect(screen.getByText("Card content")).toBeInTheDocument()
    expect(screen.getByText("Card footer")).toBeInTheDocument()
  })
})
