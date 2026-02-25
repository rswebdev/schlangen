# Snake.io

A multiplayer snake game playable in the browser. Supports solo play with AI opponents and online multiplayer via a Go WebSocket server.

## Running the Server

```bash
cd server
go build -o snake-server .
./snake-server
```

Open http://localhost:8080 in your browser.

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP/WebSocket server port |
| `-static` | `..` | Directory containing `index.html` |

Example with custom port:

```bash
./snake-server -port 3000
```

## How to Play

- **Solo Play** - Click "Solo Play" on the start screen. Plays locally with AI snakes.
- **Online Play** - Click "Online Play", enter the server WebSocket URL (e.g. `ws://localhost:8080/ws`), and click Connect.

### Controls

**Desktop:** Move the mouse to steer, hold left click to boost.

**Mobile:** Touch and drag to steer with the virtual joystick, tap the boost button to boost.

## Project Structure

```
index.html          Client (game rendering, input, networking)
server/
  main.go           Entry point, HTTP + WebSocket server
  game.go           Game logic (snakes, AI, food, collisions)
  network.go        WebSocket handling, binary protocol serialization
  go.mod            Go module definition
```

## Requirements

- Go 1.21+
- A modern browser with WebSocket support
