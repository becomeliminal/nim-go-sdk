# Nim Frontend

Shared frontend for Nim Go SDK examples. This is a minimal React + TypeScript application that integrates with the Nim Chat component.

## Features

- **NimChat Component**: Pre-integrated [@liminalcash/nim-chat](https://github.com/becomeliminal/nim-chat)
- **WebSocket Connection**: Connects to your Nim Go SDK backend
- **Configurable**: Customize via environment variables
- **Responsive**: Mobile-friendly design
- **Type-Safe**: Built with TypeScript

## Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build
```

## Configuration

Create a `.env` file to customize the frontend:

```env
# Backend WebSocket URL (default: ws://localhost:8080/ws)
VITE_WS_URL=ws://localhost:8080/ws

# Liminal API URL (default: https://api.liminal.cash)
VITE_API_URL=https://api.liminal.cash
```

## Usage in Examples

### Option 1: Copy to your example

```bash
cp -r examples/frontend examples/my-example/frontend
cd examples/my-example/frontend
npm install
npm run dev
```

### Option 2: Symlink (for development)

```bash
cd examples/my-example
ln -s ../frontend frontend
cd frontend
npm install
npm run dev
```

### Option 3: Customize main.tsx

For example-specific content, edit `main.tsx` directly:

```tsx
function App() {
  return (
    <>
      <main>
        <h1>My Custom Example</h1>
        <p>Custom instructions here...</p>
      </main>

      <NimChat
        wsUrl={wsUrl}
        apiUrl={apiUrl}
        title="My Agent"
        position="bottom-right"
        defaultOpen={false}
      />
    </>
  )
}
```

## Tech Stack

- **React 18** - UI framework
- **TypeScript 5** - Type safety
- **Vite 5** - Build tool
- **@liminalcash/nim-chat** - Chat component

## Development

The frontend runs on port 5173 by default. Make sure your backend is running on port 8080 (or update `VITE_WS_URL` accordingly).

## Backend Integration

Your Nim Go SDK backend should:
1. Serve WebSocket connections at `/ws`
2. Implement the Nim protocol (see `server/protocol.go`)
3. Handle authentication via JWT tokens (automatic with `@liminalcash/nim-chat`)

See `examples/hackathon-starter` for a complete example.
