import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { AppProvider } from './context/AppContext'
import { DownloadProvider } from './context/DownloadContext'
import { TaskProgressProvider } from './context/TaskProgressContext'
import { initializeServices } from './services/adapter'

// Initialize services (REST API and WebSocket) before rendering — see #1243 for Vercel BFF smoke-test
initializeServices().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AppProvider>
        <DownloadProvider>
          <TaskProgressProvider>
            <App />
          </TaskProgressProvider>
        </DownloadProvider>
      </AppProvider>
    </StrictMode>,
  )
}).catch((error) => {
  console.error('Failed to initialize services:', error)
  // Render anyway - the app should handle missing services gracefully
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AppProvider>
        <DownloadProvider>
          <TaskProgressProvider>
            <App />
          </TaskProgressProvider>
        </DownloadProvider>
      </AppProvider>
    </StrictMode>,
  )
})
