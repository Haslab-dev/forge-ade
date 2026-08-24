// Streamable-HTTP MCP client — JSON-RPC over POST to a remote URL.
// Handshake: initialize → capture Mcp-Session-Id → notifications/initialized.
// Responses arrive either as plain JSON or as an SSE stream; both handled.

export interface McpToolDef {
  name: string;
  description?: string | undefined;
  inputSchema?: Record<string, unknown> | undefined;
}

export interface McpCallResult {
  content: string;
  isError: boolean;
}

export interface McpHttpConnectOptions {
  url: string;
  headers?: Record<string, string> | undefined;
  serverName: string;
  requestTimeoutMs?: number | undefined;
}

interface JsonRpcResponse {
  jsonrpc?: string;
  id?: unknown;
  result?: unknown;
  error?: { code?: number; message?: string } | null;
}

function extractText(result: unknown): { content: string; isError: boolean } {
  if (!result || typeof result !== "object") return { content: "", isError: false };
  const rec = result as Record<string, unknown>;
  const blocks = Array.isArray(rec.content) ? rec.content : [];
  const parts: string[] = [];
  for (const b of blocks) {
    if (b && typeof b === "object" && (b as { type?: unknown }).type === "text") {
      const t = (b as { text?: unknown }).text;
      if (typeof t === "string") parts.push(t);
    }
  }
  return {
    content: parts.join("\n"),
    isError: rec.isError === true,
  };
}

/** Pulls JSON-RPC payloads out of an SSE body (`data:` lines). */
function jsonFromSse(body: string): unknown[] {
  const out: unknown[] = [];
  for (const line of body.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed.startsWith("data:")) continue;
    const data = trimmed.slice(5).trim();
    if (!data || data === "[DONE]") continue;
    try {
      out.push(JSON.parse(data));
    } catch {}
  }
  return out;
}

export class McpHttpClient {
  readonly serverName: string;
  private url: string;
  private headers: Record<string, string>;
  private requestTimeoutMs: number;
  private sessionId: string | null = null;
  private nextId = 1;
  private _connected = false;

  private constructor(opts: {
    serverName: string;
    url: string;
    headers?: Record<string, string> | undefined;
    requestTimeoutMs: number;
  }) {
    this.serverName = opts.serverName;
    this.url = opts.url;
    this.headers = opts.headers ?? {};
    this.requestTimeoutMs = opts.requestTimeoutMs;
  }

  get connected(): boolean {
    return this._connected;
  }

  static async connect(opts: McpHttpConnectOptions): Promise<McpHttpClient> {
    const normalized = {
      ...opts,
      requestTimeoutMs: opts.requestTimeoutMs ?? 20_000,
    };
    const client = new McpHttpClient(normalized);
    await client.request("initialize", {
      protocolVersion: "2024-11-05",
      capabilities: {},
      clientInfo: { name: "forge-ade", version: "1.0.0" },
    });
    // Spec: client MUST follow up with the initialized notification.
    await client.notify("notifications/initialized");
    client._connected = true;
    return client;
  }

  async listTools(): Promise<McpToolDef[]> {
    const result = await this.request("tools/list", {});
    const rec = (result ?? {}) as { tools?: unknown };
    if (!Array.isArray(rec.tools)) return [];
    return rec.tools
      .filter((t): t is Record<string, unknown> => !!t && typeof t === "object")
      .map((t) => ({
        name: typeof t.name === "string" ? t.name : "",
        ...(typeof t.description === "string" ? { description: t.description } : {}),
        ...(t.inputSchema && typeof t.inputSchema === "object"
          ? { inputSchema: t.inputSchema as Record<string, unknown> }
          : {}),
      }))
      .filter((t) => t.name.length > 0);
  }

  async callTool(name: string, args: Record<string, unknown>): Promise<McpCallResult> {
    const result = await this.request("tools/call", { name, arguments: args });
    return extractText(result);
  }

  close(): void {
    this._connected = false;
  }

  // ---------------------------------------------------------------------------

  private async post(message: Record<string, unknown>): Promise<{ status: number; headers: Headers; body: string }> {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(new Error(`${this.serverName}: request timed out`)), this.requestTimeoutMs);
    try {
      const res = await fetch(this.url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json, text/event-stream",
          ...this.headers,
          ...(this.sessionId ? { "Mcp-Session-Id": this.sessionId } : {}),
        },
        body: JSON.stringify(message),
        signal: ctrl.signal,
      });
      const body = await res.text();
      return { status: res.status, headers: res.headers, body };
    } finally {
      clearTimeout(timer);
    }
  }

  /** Sends a request and resolves with its `result`, surfacing protocol errors. */
  private async request(method: string, params: unknown): Promise<unknown> {
    const id = this.nextId++;
    const { status, headers, body } = await this.post({
      jsonrpc: "2.0",
      id,
      method,
      params: params ?? {},
    });

    if (status === 404) {
      // Session expired on the remote side; caller should reconnect.
      this._connected = false;
      throw new Error(`${this.serverName}: session expired or endpoint not found (${status})`);
    }
    if (status >= 400) {
      let message = `HTTP ${status}`;
      try {
        const parsed = JSON.parse(body) as { error?: { message?: string } };
        if (parsed?.error?.message) message = parsed.error.message;
      } catch {}
      throw new Error(`${this.serverName}: ${message}`);
    }

    const contentType = headers.get("content-type") ?? "";
    const responses: JsonRpcResponse[] = [];
    if (contentType.includes("text/event-stream")) {
      for (const raw of jsonFromSse(body)) {
        if (isResponse(raw)) responses.push(raw);
      }
    } else {
      const parsed = safeParse(body);
      if (parsed) responses.push(parsed as JsonRpcResponse);
    }

    const match = responses.find((r) => r.id === id);
    if (!match) throw new Error(`${this.serverName}: no response for ${method}`);

    // Persist session for subsequent calls (streamable HTTP handshake).
    const sid = headers.get("mcp-session-id");
    if (sid) this.sessionId = sid;

    if (match.error) throw new Error(`${this.serverName}: ${match.error.message ?? "protocol error"}`);
    return match.result;
  }

  private async notify(method: string): Promise<void> {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(new Error("notify timed out")), this.requestTimeoutMs);
    try {
      await fetch(this.url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json, text/event-stream",
          ...this.headers,
          ...(this.sessionId ? { "Mcp-Session-Id": this.sessionId } : {}),
        },
        body: JSON.stringify({ jsonrpc: "2.0", method, params: {} }),
        signal: ctrl.signal,
      });
    } catch (err) {
      // Notifications are fire-and-forget; failures surface on the next call.
      console.error(`[mcp:${this.serverName}] notify ${method} failed:`, err);
    } finally {
      clearTimeout(timer);
    }
  }
}

function isResponse(value: unknown): value is JsonRpcResponse {
  return (
    !!value &&
    typeof value === "object" &&
    ("result" in (value as Record<string, unknown>) || "error" in (value as Record<string, unknown>))
  );
}

function safeParse(text: string): unknown | null {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}
