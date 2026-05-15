import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ClerkProvider } from '@clerk/react'
import { ui } from '@clerk/ui'
import * as Sentry from '@sentry/react'
import './index.css'
import App from './App.tsx'
import { AppProvider } from './context/AppContext'
import { DownloadProvider } from './context/DownloadContext'
import { TaskProgressProvider } from './context/TaskProgressContext'
import { initializeServices } from './services/adapter'

// Social OAuth providers (Google, Facebook, Apple) are enabled in the Clerk Dashboard
// under "Social connections" — no additional code required here.
// Dashboard: https://dashboard.clerk.com → Social connections
// VITE_CLERK_PUBLISHABLE_KEY is read automatically by ClerkProvider from the environment.
//
// `ui` from @clerk/ui pins the bundled Clerk component version so structural CSS
// selectors like .api-keys-content .cl-apiKeys are stable across Clerk CDN updates.
// This suppresses the structural_css_pin_clerk_ui console warning (#2006).

// Initialize Sentry only when VITE_SENTRY_DSN is provided (skip silently in dev/test).
const sentryDsn = import.meta.env.VITE_SENTRY_DSN
if (sentryDsn) {
  Sentry.init({
    dsn: sentryDsn,
    environment: import.meta.env.MODE,
    integrations: [
      Sentry.browserTracingIntegration(),
      // feedbackIntegration: enables Sentry.getFeedback() in ReportBugButton.
      // autoInject: false — we render our own trigger button in Layout instead of
      // the default floating widget so the button only appears for signed-in users.
      Sentry.feedbackIntegration({ autoInject: false }),
    ],
  })
}

const rootElement = document.getElementById('root')!

const renderApp = () => {
  createRoot(rootElement).render(
    <StrictMode>
      <Sentry.ErrorBoundary fallback={<p>Something went wrong</p>}>
        <ClerkProvider publishableKey={import.meta.env.VITE_CLERK_PUBLISHABLE_KEY} ui={ui}>
          <AppProvider>
            <DownloadProvider>
              <TaskProgressProvider>
                <App />
              </TaskProgressProvider>
            </DownloadProvider>
          </AppProvider>
        </ClerkProvider>
      </Sentry.ErrorBoundary>
    </StrictMode>,
  )
}

// Initialize services (REST API and WebSocket) before rendering — see #1243 for Vercel BFF smoke-test
initializeServices().then(() => {
  renderApp()
}).catch((error) => {
  console.error('Failed to initialize services:', error)
  // Render anyway - the app should handle missing services gracefully
  renderApp()
})
