# Quickstart

## Before the workshop

Install these ahead of time:

- **Go 1.21+** — https://go.dev/dl/
- **Node.js 18+** — https://nodejs.org/
- **Anthropic API key** — https://console.anthropic.com/

## 1. Clone the repo

```bash
git clone https://github.com/becomeliminal/nim-go-sdk.git
cd nim-go-sdk/examples/hackathon-starter
```

## 2. Add your API key

```bash
cp .env.example .env
```

Open `.env` and paste your Anthropic key:

```
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## 3. Start the backend

```bash
go run main.go
```

You should see:

```
✅ Liminal API configured
✅ Added 9 Liminal banking tools
✅ Added custom spending analyzer tool
🚀 Hackathon Starter Server Running
📡 WebSocket endpoint: ws://localhost:8080/ws
```

## 4. Start the frontend (new terminal)

```bash
cd nim-go-sdk/examples/frontend
npm install
npm run dev
```

## 5. Try it out

Open **http://localhost:5173**, click the chat bubble, log in with your email, and start talking:

- "What's my balance?"
- "Show my recent transactions"
- "Send $5 to @alice"
- "Analyze my spending over the last 30 days"

## You're running a live AI banking agent.

Now open `examples/hackathon-starter/main.go` and build your own tool.
