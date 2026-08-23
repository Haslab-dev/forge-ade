// End-to-end daemon smoke: HTTP methods + WS event capture against a running
// instance. Usage: bun src/server/agent/smoke.ts <port>
const PORT = process.argv[2] || "45998";

async function invoke(method: string, params: Record<string, unknown> = {}) {
  const res = await fetch(`http://127.0.0.1:${PORT}/api/invoke`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ method, params }),
  });
  const body = await res.json();
  if (body.error) throw new Error(`${method}: ${body.error}`);
  return body.result;
}

// --- WebSocket event collector ---
const ws = new WebSocket(`ws://127.0.0.1:${PORT}/ws`);
const events: { type: string; payload: Record<string, unknown> }[] = [];
const drained = Promise.withResolvers<void>();
ws.onmessage = (ev) => {
  const msg = JSON.parse(String(ev.data));
  events.push(msg);
};
await new Promise((r) => setTimeout(r, 300));

// 1. Create session
const session = await invoke("CreateAgentSession", { name: "SMOKE", role: "coding" });
console.log("created:", session.id);

// 2. Metadata list must NOT contain messages
const list = await invoke("ListAgentSessions");
const meta = list.find((s: { id: string }) => s.id === session.id);
if (!meta) throw new Error("session missing from list");
if ("messages" in meta) throw new Error("list leaked messages — contract violation");
console.log("meta ok:", JSON.stringify(meta));

// 3. GetAgentSession returns full (empty) transcript
const full = await invoke("GetAgentSession", { id: session.id });
if (!Array.isArray(full.messages)) throw new Error("full session lacks messages array");

// 4. Send message with no provider configured -> expect agent:error + turn_end
await invoke("SendAgentMessage", { id: session.id, message: "hello" });
await new Promise((r) => setTimeout(r, 800));
const forSession = events.filter((e) => e.payload.id === session.id);
const types = forSession.map((e) => e.type);
console.log("events:", types.join(","));
if (!types.includes("agent:error")) throw new Error("expected agent:error without provider");
if (!types.includes("agent:turn_end")) throw new Error("expected agent:turn_end");
if (types.includes("agent:message_delta")) throw new Error("unexpected deltas without provider");

// 5. Cleanup
await invoke("DeleteAgentSession", { id: session.id });
console.log("SMOKE PASS");
drained.resolve();
ws.close();
process.exit(0);
