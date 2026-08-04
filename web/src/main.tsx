import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { ErrorBoundary, TokenGate } from "./Shell";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <TokenGate>
        <App />
      </TokenGate>
    </ErrorBoundary>
  </StrictMode>,
);
