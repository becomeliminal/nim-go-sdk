import React from 'react'
import ReactDOM from 'react-dom/client'
import { NimChat } from '@liminalcash/nim-chat'
import '@liminalcash/nim-chat/styles.css'
import './styles.css'

function App() {
  const wsUrl = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws'
  const apiUrl = import.meta.env.VITE_API_URL || 'https://api.liminal.cash'

  return (
    <>
      <main>
        <h1>Welcome to Nim</h1>
        <p>An AI agent powered by the Nim Go SDK</p>
        <p className="instructions">
          Click the chat button in the bottom-right corner to get started.
        </p>
      </main>

      <NimChat
        wsUrl={wsUrl}
        apiUrl={apiUrl}
        title="Nim"
        position="bottom-right"
        defaultOpen={false}
      />
    </>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(<App />)
