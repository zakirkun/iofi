import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { router } from "./router";
import { ErrorBoundary, TokenGate } from "./Shell";
import { Unauthorized } from "./lib";
import { reportError } from "./store";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // The SSE stream invalidates what changed, so background refetching would
      // re-request data that is already current.
      refetchOnWindowFocus: false,
      staleTime: 10_000,
      retry: (count, err) => !(err instanceof Unauthorized) && count < 2,
    },
    mutations: {
      // Every mutation failure surfaces as a toast unless the caller handles it,
      // so a refused WIP move or a rejected upload is never silent.
      onError: reportError,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <TokenGate>
          <RouterProvider router={router} />
        </TokenGate>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
);
