import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ClerkProvider } from '@clerk/react'
import './index.css'
import App from './App.tsx'
import { AppProvider } from './context/AppContext'
import { DownloadProvider } from './context/DownloadContext'
import { TaskProgressProvider } from './context/TaskProgressContext'
import { initializeServices } from './services/adapter'

// Clerk publishable key — set VITE_CLERK_PUBLISHABLE_KEY in .env (see .env.example).
// Social OAuth providers (Google, Facebook, Apple) are enabled in the Clerk Dashboard
// under "Social connections" — no additional code required here.
// Dashboard: https://dashboard.clerk.com → Social connections
const clerkPublishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY as string

const rootElement = document.getElementById('root')!

const renderApp = () => {
  createRoot(rootElement).render(
    <StrictMode>
      <ClerkProvider publishableKey={clerkPublishableKey} afterSignOutUrl="/">
        <AppProvider>
          <DownloadProvider>
            <TaskProgressProvider>
              <App />
            </TaskProgressProvider>
          </DownloadProvider>
        </AppProvider>
      </ClerkProvider>
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
