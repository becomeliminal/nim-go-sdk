import React from 'react'
import ReactDOM from 'react-dom/client'
import { NimChat } from '@liminalcash/nim-chat'
import '@liminalcash/nim-chat/styles.css'
import './styles.css'

function App() {
  const wsUrl = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws'
  const apiUrl = import.meta.env.VITE_API_URL || 'https://dev-api.liminal.cash'

  return (
    <>
      <main>
        <h1>Build financial autonomy for AI</h1>

        <ol>
          <li>
            Download <a href="https://apps.apple.com/app/testflight/id899247664" target="_blank" rel="noopener noreferrer">TestFlight</a> from the App Store
          </li>

          <li>
            Install <a href="https://testflight.apple.com/join/ZYTDH2bd" target="_blank" rel="noopener noreferrer">Liminal via TestFlight</a>
          </li>

          <li>
            Sign up to Liminal (this is how you authenticate with Nim)
          </li>

          <li>
            Clone the <a href="https://github.com/becomeliminal/nim-go-sdk" target="_blank" rel="noopener noreferrer">Nim Go SDK</a>
            <div className="code-block">
              git clone https://github.com/becomeliminal/nim-go-sdk.git<br />
              cd nim-go-sdk/examples/hackathon-starter
            </div>
          </li>

          <li>
            Create a frontend using the <a href="https://github.com/becomeliminal/nim-chat" target="_blank" rel="noopener noreferrer">Nim Chat</a> component (or use this one)
            <div className="code-block">
              cd frontend<br />
              npm install<br />
              npm run dev
            </div>
          </li>

          <li>
            Create a backend using the Nim Go SDK — see the <a href="https://github.com/becomeliminal/nim-go-sdk/tree/master/examples/hackathon-starter" target="_blank" rel="noopener noreferrer">example</a>
            <div className="code-block">
              {`# In a new terminal`}<br />
              cd ..<br />
              cp .env.example .env<br />
              {`# Add your ANTHROPIC_API_KEY to .env`}<br />
              go run main.go
            </div>
          </li>

          <li>
            Build cool tools for Nim
          </li>
        </ol>
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
