import { server } from "./index";

const PORT = 45123;
const HOST = "127.0.0.1";

const wsClients = new Set<any>();

function broadcast(eventName: string, payload: any) {
  const msg = JSON.stringify({ type: eventName, payload });
  for (const ws of wsClients) {
    try {
      ws.send(msg);
    } catch {}
  }
}

// Wire terminal streaming data and events to WebSocket clients
server.terminal.setOnEvent((eventName, payload) => {
  broadcast(eventName, payload);
});

// Wire AI agent streaming data and events to WebSocket clients
server.agent.setOnEvent((eventName, payload) => {
  broadcast(eventName, payload);
});

// Wire LSP diagnostics streaming to WebSocket clients
server.lsp.setOnEvent((eventName, payload) => {
  broadcast(eventName, payload);
});

// Start Bun / Node HTTP + WebSocket server
if (typeof Bun !== "undefined") {
  try {
    Bun.serve({
      port: PORT,
      hostname: HOST,
      fetch(req, srv) {
        const url = new URL(req.url);

        if (url.pathname === "/ws") {
          if (srv.upgrade(req)) return;
          return new Response("WebSocket upgrade failed", { status: 400 });
        }

        // CORS headers for local frontend
        const headers = {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
          "Access-Control-Allow-Headers": "Content-Type",
        };

        if (req.method === "OPTIONS") {
          return new Response(null, { headers });
        }

        if (url.pathname === "/api/invoke" && req.method === "POST") {
          return req.json().then(async (body: any) => {
            const { method, params } = body;
            try {
              const result = await server.handleMethod(method, params);
              return new Response(JSON.stringify({ result }), {
                headers: { ...headers, "Content-Type": "application/json" },
              });
            } catch (err: any) {
              return new Response(
                JSON.stringify({ error: err.message || String(err) }),
                {
                  status: 500,
                  headers: { ...headers, "Content-Type": "application/json" },
                }
              );
            }
          });
        }

        return new Response("ForgeADE Backend Server Ready", { headers });
      },
      websocket: {
        open(ws) {
          wsClients.add(ws);
        },
        message(ws, message) {
          try {
            const data = JSON.parse(String(message));
            if (data.type === "terminal:write") {
              server.terminal.writeSession(data.payload.id, data.payload.data);
            }
          } catch {}
        },
        close(ws) {
          wsClients.delete(ws);
        },
      },
    });
    console.log(`ForgeADE Backend Server listening on http://${HOST}:${PORT}`);
  } catch (err: any) {
    if (err?.code === "EADDRINUSE") {
      console.log(`ForgeADE Backend Server already listening on http://${HOST}:${PORT}`);
    } else {
      console.error("Failed to start server:", err);
    }
  }
}
