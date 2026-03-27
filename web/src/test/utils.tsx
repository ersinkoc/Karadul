import { ReactElement } from "react"
import { render, RenderOptions } from "@testing-library/react"
import { BrowserRouter } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

// Create a new query client for each test
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
        staleTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  })
}

// All providers wrapper
interface AllProvidersProps {
  children: React.ReactNode
}

export function AllProviders({ children }: AllProvidersProps) {
  const queryClient = createTestQueryClient()

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )
}

// Custom render function with providers
function customRender(
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper">
) {
  return render(ui, { wrapper: AllProviders, ...options })
}

// Re-export everything from testing library
// eslint-disable-next-line react-refresh/only-export-components
export * from "@testing-library/react"
// eslint-disable-next-line react-refresh/only-export-components
export { customRender as render }
// eslint-disable-next-line react-refresh/only-export-components
export { createTestQueryClient }
