// Live streaming probe: real LLM turn through the daemon over HTTP + WS.
const PORT = process.argv[2] || "45123";
const PROMPT = process.argv[3] || "In one short sentence, what is 2+2? Show your reasoning briefly first.";

async function invoke(method: string, params: Record<string, unknown> = {}) {
  const res = await fetch(`http://127.0.0.1:${PORT}/api/invoke`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ method, params }),
  });
  const body: any = await res.json();
  if (body.error) throw new Error(`${method}: ${body.error}`);
  return body.result;
}

const ws = new WebSocket(`ws://127.0.0.1:${PORT}/ws`);
const events: { type: string; payload: Record<string, unknown> }[] = [];
let done = false;
ws.onmessage = (ev) => {
  const msg = JSON.parse(String(ev.data));
  events.push(msg);
};

await new Promise((r) => setTimeout(r, 400));
const session = await invoke("CreateAgentSession", { name: "LIVE-PROBE", role: "coding" });
console.log("session:", session.id);

const deadline = Date.now() + 90_000;
invoke("SendAgentMessage", { id: session.id, message: PROMPT }).catch((e) => console.error("send failed:", e.message));

while (Date.now() < deadline) {
  await new Promise((r) => setTimeout(r, 200));
  const ended = events.some((e) => e.type === "agent:turn_end" && e.payload.id === session.id);
  if (ended) {
    done = true;
    break;
  }
}

const mine = events.filter((e) => (e.payload as { id?: string }).id === session.id);
const counts: Record<string, number> = {};
for (const e of mine) counts[e.type] = (counts[e.type] || 0) + 1;
console.log("event counts:", JSON.stringify(counts));

const thinkChars = mine
  .filter((e) => e.type === "agent:message_delta" && e.payload.kind === "thinking")
  .reduce((n, e) => n + String(e.payload.delta).length, 0);
const textChars = mine
  .filter((e) => e.type === "agent:message_delta" && e.payload.kind === "text")
  .reduce((n, e) => n + String(e.payload.delta).length, 0);
const deltas = mine.filter((e) => e.type === "agent:message_delta");
console.log(`thinking chars: ${thinkChars}, text chars: ${textChars}, delta events: ${deltas.length}`);
if (thinkChars > 0) {
  const sample = mine.find((e) => e.type === "agent:message_delta" && e.payload.kind === "thinking");
  console.log("thinking sample:", JSON.stringify(String(sample!.payload.delta).slice(0, 80)));
}
// Final transcript from the authoritative store.
const full = await invoke("GetAgentSession", { id: session.id });
console.log("persisted messages:", full.messages.length);
for (const m of full.messages) {
  for (const b of m.content) {
    console.log(`  [${m.role}/${b.type}] ${String(b.text ?? b.name ?? "").slice(0, 100).replace(/\n/g, " ")}`);
  }
}
await invoke("DeleteAgentSession", { id: session.id });
ws.close();
if (!done) {
  console.error("TURN DID NOT COMPLETE IN TIME");
  process.exit(1);
}
console.log(done && textChars > 0 ? "LIVE STREAM PASS" : "NO TEXT DELTAS — FAIL");
process.exit(textChars > 0 && done ? 0 : 1);
