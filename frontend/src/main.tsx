import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ClerkProvider } from "@clerk/clerk-react";
import App from "./App";
import { ClerkAvailabilityProvider } from "./components/auth/ClerkAvailability";
import "./index.css";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

// Vite substitutes VITE_-prefixed env vars at build time. When this is
// missing (e.g. nobody created frontend/.env.local) we still want the
// anonymous game to work, so we boot the app without Clerk and log an
// explicit error. Sign-in won't work in that state — public routes
// continue to function. Throwing here would bring the SPA down for
// anonymous visitors which breaks the game for every dev who hasn't
// configured keys yet.
const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;

if (!publishableKey) {
  // eslint-disable-next-line no-console
  console.error(
    "auth: VITE_CLERK_PUBLISHABLE_KEY is not set. Sign-in will not work. " +
      "Copy frontend/.env.local.example to frontend/.env.local and set " +
      "VITE_CLERK_PUBLISHABLE_KEY. See docs/runbooks/admin-auth-setup.md.",
  );
}

createRoot(rootElement).render(
  <StrictMode>
    {publishableKey ? (
      <ClerkProvider publishableKey={publishableKey}>
        <ClerkAvailabilityProvider available={true}>
          <App />
        </ClerkAvailabilityProvider>
      </ClerkProvider>
    ) : (
      <ClerkAvailabilityProvider available={false}>
        <App />
      </ClerkAvailabilityProvider>
    )}
  </StrictMode>,
);
