// Integration test for the MCP stdio client: drives a fake MCP server
// subprocess over real pipes, covering the initialize handshake,
// ping-from-server answering, tool listing/calling, request timeout, and close.

import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import fs from "fs";
import os from "os";
import path from "path";
import { McpStdioClient } from "./client";

const FIXTURE_SRC = `#!/usr/bin/env bun
import fs from "fs";

const mode = process.env.FAKE_MCP_MODE ?? "ok";
let buffer = "";
let gotPingAck = false;
const deferred = [];

if (process.env.MCP_PID_FILE) fs.writeFileSync(process.env.MCP_PID_FILE, String(process.pid));

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + "\\n");
}

function handle(msg) {
  if (msg.method === "initialize") {
    send({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        protocolVersion: "2024-11-05",
        capabilities: { tools: {} },
        serverInfo: { name: "fake" },
      },
    });
    return;
  }
  if (msg.method === "notifications/initialized") {
    // Server-initiated ping right after handshake; client must answer with a result.
    if (mode === "ok") send({ jsonrpc: "2.0", id: "srv1", method: "ping" });
    return;
  }
  if (msg.id === "srv1") {
    gotPingAck = true;
    // Answer any requests that were queued while waiting for the ping reply.
    for (const queued of deferred) handle(queued);
    deferred.length = 0;
    return;
  }
  if (mode === "silent") return; // never answers anything else
  if (msg.method === "tools/list" && !gotPingAck) {
    deferred.push(msg); // deterministic: only answer tools/list after the ping round-trip
    return;
  }
  if (msg.method === "tools/list") {
    send({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        tools: [
          {
            name: "echo",
            description: "echo tool [ping-ok]",
            inputSchema: { type: "object", properties: { message: { type: "string" } }, required: ["message"] },
          },
          { name: "fail", description: "always fails" },
        ],
      },
    });
    return;
  }
  if (msg.method === "tools/call") {
    const name = msg.params.name;
    if (name === "echo") {
      send({
        jsonrpc: "2.0",
        id: msg.id,
        result: { content: [{ type: "text", text: "echo:" + msg.params.arguments.message }] },
      });
    } else if (name === "fail") {
      send({ jsonrpc: "2.0", id: msg.id, result: { content: [{ type: "text", text: "boom" }], isError: true } });
    }
    return;
  }
}

process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => {
  buffer += chunk;
  for (;;) {
    const nl = buffer.indexOf("\\n");
    if (nl < 0) break;
    const line = buffer.slice(0, nl).trim();
    buffer = buffer.slice(nl + 1);
    if (line === "") continue;
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      continue;
    }
    handle(msg);
  }
});
`;

let tmpDir: string;
let fixturePath: string;

beforeAll(() => {
  tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "mcp-client-test-"));
  fixturePath = path.join(tmpDir, "fake-mcp-server.mjs");
  fs.writeFileSync(fixturePath, FIXTURE_SRC);
});

afterAll(() => {
  fs.rmSync(tmpDir, { recursive: true, force: true });
});

function spawnFake(opts?: {
  mode?: "ok" | "silent";
  requestTimeoutMs?: number;
  /** Extra environment variables for the fixture process (e.g. MCP_PID_FILE). */
  env?: Record<string, string>;
}): Promise<McpStdioClient> {
  const env: Record<string, string> = {};
  if (opts !== undefined && opts.mode !== undefined) env.FAKE_MCP_MODE = opts.mode;
  if (opts?.env !== undefined) Object.assign(env, opts.env);
  return McpStdioClient.spawn({
    command: process.execPath,
    args: [fixturePath],
    serverName: "fake",
    ...(opts?.requestTimeoutMs !== undefined ? { requestTimeoutMs: opts.requestTimeoutMs } : {}),
    ...(Object.keys(env).length > 0 ? { env } : {}),
  });
}

/**
 * Waits until the given pid is gone (process.kill(pid,0) throws). Polls the real
 * OS process table against SIGTERM teardown — no fake timer can observe an
 * external subprocess exit, so wall-clock polling is required here.
 */
async function waitPidGone(pid: number, deadlineMs = 5000): Promise<void> {
  const start = Date.now();
  for (;;) {
    let alive = false;
    try {
      process.kill(pid, 0);
      alive = true;
    } catch {
      alive = false;
    }
    if (!alive) return;
    if (Date.now() - start > deadlineMs) throw new Error(`pid ${pid} still alive after ${deadlineMs}ms`);
    const { promise, resolve } = Promise.withResolvers<void>();
    setTimeout(resolve, 25).unref();
    await promise;
  }
}

describe("McpStdioClient", () => {
  test("spawn performs the initialize handshake", async () => {
    const client = await spawnFake();
    try {
      expect(client.connected).toBe(true);
      expect(client.serverName).toBe("fake");
    } finally {
      client.close();
    }
    expect(client.connected).toBe(false);
  });

  test("listTools returns both tools and proves the server ping was answered", async () => {
    const client = await spawnFake();
    try {
      const tools = await client.listTools();
      expect(tools.map((t) => t.name)).toEqual(["echo", "fail"]);
      // The fake server only stamps [ping-ok] once its server->client ping got a result.
      expect(tools[0]?.description).toContain("[ping-ok]");
      expect(tools[0]?.inputSchema).toEqual({
        type: "object",
        properties: { message: { type: "string" } },
        required: ["message"],
      });
      expect(tools[1]?.inputSchema).toBeUndefined();
    } finally {
      client.close();
    }
  });

  test("callTool echo returns concatenated text content", async () => {
    const client = await spawnFake();
    try {
      const res = await client.callTool("echo", { message: "hello" });
      expect(res.content).toBe("echo:hello");
      expect(res.isError).toBe(false);
    } finally {
      client.close();
    }
  });

  test("callTool fail maps isError through instead of throwing", async () => {
    const client = await spawnFake();
    try {
      const res = await client.callTool("fail", {});
      expect(res.isError).toBe(true);
      expect(res.content).toBe("boom");
    } finally {
      client.close();
    }
  });

  test("request times out when the server never answers", async () => {
    const client = await spawnFake({ mode: "silent", requestTimeoutMs: 500 });
    try {
      const start = Date.now();
      await expect(client.listTools()).rejects.toThrow(/timed out/);
      expect(Date.now() - start).toBeLessThan(1500);
    } finally {
      client.close();
    }
  });

  test("close kills the child process", async () => {
    const pidFile = path.join(tmpDir, "pid-close.txt");
    const client = await spawnFake({ env: { MCP_PID_FILE: pidFile } });
    const pid = Number.parseInt(fs.readFileSync(pidFile, "utf8"), 10);
    expect(Number.isFinite(pid)).toBe(true);

    client.close();
    await expect(waitPidGone(pid)).resolves.toBeUndefined();
    expect(client.connected).toBe(false);
    await expect(client.listTools()).rejects.toThrow(/not connected/);
  });
});
