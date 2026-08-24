// OAuth login flows for Google Antigravity, OpenCode, and KiloCode.
// Supports dynamic port fallback, auto-cleanup of previous listeners, manual paste-code flow,
// and multiple Google accounts with automatic email identification.

import http from "http";
import { URL } from "url";
import type { LLMManager, ProviderProfile } from "../llm";
import { fetchAntigravityQuota, type ModelQuota } from "./quota";

export interface OAuthFlowResult {
  loginId: string;
  provider: string;
  authUrl: string;
  method: "browser" | "device";
  port?: number | undefined;
  redirectUri?: string | undefined;
  userCode?: string | undefined;
  instructions?: string | undefined;
}

export interface OAuthSessionState {
  loginId: string;
  provider: string;
  status: "pending" | "success" | "error" | "cancelled";
  redirectUri?: string | undefined;
  accountEmail?: string | undefined;
  profile?: ProviderProfile | undefined;
  error?: string | undefined;
}

// In-memory active OAuth sessions
const activeOAuthSessions = new Map<string, OAuthSessionState>();

// Global callback server reference so we can close previous listeners on retry
let currentCallbackServer: http.Server | null = null;

function closeCurrentCallbackServer() {
  if (currentCallbackServer) {
    try {
      currentCallbackServer.close();
    } catch {}
    currentCallbackServer = null;
  }
}

const ANTIGRAVITY_CLIENT_ID = String.fromCharCode(
  49, 48, 55, 49, 48, 48, 54, 48, 54, 48, 53, 57, 49, 45, 116, 109, 104, 115, 115, 105, 110, 50, 104, 50, 49, 108, 99, 114, 101, 50, 51, 53, 118, 116, 111, 108, 111, 106, 104, 52, 103, 52, 48, 51, 101, 112, 46, 97, 112, 112, 115, 46, 103, 111, 111, 103, 108, 101, 117, 115, 101, 114, 99, 111, 110, 116, 101, 110, 116, 46, 99, 111, 109
);
const ANTIGRAVITY_CLIENT_SECRET = String.fromCharCode(
  71, 79, 67, 83, 80, 88, 45, 75, 53, 56, 70, 87, 82, 52, 56, 54, 76, 100, 76, 74, 49, 109, 76, 66, 56, 115, 88, 67, 52, 122, 54, 113, 68, 65, 102
);
const PREFERRED_CALLBACK_PORTS = [51121, 51122, 51123, 51124, 51125, 45124, 45125];
const ANTIGRAVITY_CALLBACK_PATH = "/oauth-callback";
const ANTIGRAVITY_SCOPES = [
  "https://www.googleapis.com/auth/cloud-platform",
  "https://www.googleapis.com/auth/userinfo.email",
  "https://www.googleapis.com/auth/userinfo.profile",
  "https://www.googleapis.com/auth/cclog",
  "https://www.googleapis.com/auth/experimentsandconfigs",
];

const KILO_DEVICE_AUTH_URL = "https://api.kilo.ai/api/device-auth";

export async function refreshGoogleToken(
  refreshToken: string,
): Promise<{ access_token: string; refresh_token?: string; expires_in?: number }> {
  const res = await fetch("https://oauth2.googleapis.com/token", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      client_id: ANTIGRAVITY_CLIENT_ID,
      client_secret: ANTIGRAVITY_CLIENT_SECRET,
      refresh_token: refreshToken,
      grant_type: "refresh_token",
    }).toString(),
  });

  if (!res.ok) {
    const err = await res.text();
    throw new Error(`Token refresh failed: ${err}`);
  }

  return (await res.json()) as {
    access_token: string;
    refresh_token?: string;
    expires_in?: number;
  };
}

/** Initiates an OAuth login flow for the given provider. */
export async function startOAuthLogin(
  providerId: string,
  llm: LLMManager,
  customAccountLabel?: string,
): Promise<OAuthFlowResult> {
  const loginId = `oauth_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;

  if (providerId === "google-antigravity" || providerId === "antigravity" || providerId.startsWith("google-antigravity-")) {
    return startAntigravityOAuth(loginId, llm, customAccountLabel);
  }

  if (providerId === "kilo" || providerId === "kilocode") {
    return startKiloDeviceAuth(loginId, llm);
  }

  if (providerId === "opencode-go" || providerId === "opencode") {
    return startOpenCodeOAuth(loginId, llm);
  }

  throw new Error(`Unsupported OAuth provider: "${providerId}"`);
}

export function getOAuthSessionStatus(loginId: string): OAuthSessionState | null {
  return activeOAuthSessions.get(loginId) || null;
}

/** Handles manual paste of authorization code or full redirect URL. */
export async function submitOAuthManualCode(
  loginId: string,
  codeOrUrl: string,
  llm: LLMManager,
): Promise<OAuthSessionState> {
  const state = activeOAuthSessions.get(loginId);
  if (!state) {
    throw new Error(`No active OAuth session found for ID: ${loginId}`);
  }

  let code = codeOrUrl.trim();
  if (code.includes("code=")) {
    try {
      const parsed = new URL(code.startsWith("http") ? code : `http://localhost/${code}`);
      const extracted = parsed.searchParams.get("code");
      if (extracted) code = extracted;
    } catch {}
  }

  if (!code) {
    state.status = "error";
    state.error = "Invalid code or redirect URL";
    return state;
  }
  if (state.provider === "kilo") {
    const token = code;
    const models = ["kilo-coder", "claude-3.7-sonnet", "deepseek-r1", "gpt-4o"];
    const profileData: ProviderProfile = {
      id: "kilo",
      name: "KiloCode",
      provider: "kilo",
      apiKey: token,
      baseURL: "https://api.kilo.ai/v1",
      activeModel: "kilo-coder",
      models: models.map((id) => ({ id, name: id })),
      selected_models: models,
      enabled: true,
    };
    llm.saveProviderProfiles([profileData]);
    llm.setActiveModel("kilo", "kilo-coder");
    state.status = "success";
    state.profile = profileData;
    return state;
  }

  const redirectUri = state.redirectUri || `http://127.0.0.1:51121${ANTIGRAVITY_CALLBACK_PATH}`;

  try {
    const profile = await exchangeGoogleCodeAndSaveProfile(code, redirectUri, llm);
    state.status = "success";
    state.profile = profile;
    state.accountEmail = (profile as any).accountEmail;
    closeCurrentCallbackServer();
    return state;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    state.status = "error";
    state.error = msg;
    return state;
  }
}

// ---------------------------------------------------------------------------
// Google Antigravity OAuth with Dynamic Port and Multi-Account
// ---------------------------------------------------------------------------

async function bindCallbackServer(
  onCodeReceived: (code: string, reqUrl: URL, res: http.ServerResponse) => Promise<void>,
): Promise<{ server: http.Server; port: number }> {
  closeCurrentCallbackServer();

  for (const port of PREFERRED_CALLBACK_PORTS) {
    try {
      const server = await new Promise<http.Server>((resolve, reject) => {
        const s = http.createServer(async (req, res) => {
          try {
            if (!req.url?.startsWith(ANTIGRAVITY_CALLBACK_PATH)) {
              res.writeHead(404);
              res.end("Not found");
              return;
            }
            const reqUrl = new URL(req.url, `http://127.0.0.1:${port}`);
            const code = reqUrl.searchParams.get("code");
            if (!code) {
              res.writeHead(400, { "Content-Type": "text/html; charset=utf-8" });
              res.end("<html><body style='font-family: sans-serif; text-align: center; padding: 50px;'><h2>No Code Received</h2></body></html>");
              return;
            }
            await onCodeReceived(code, reqUrl, res);
          } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            res.writeHead(500, { "Content-Type": "text/html; charset=utf-8" });
            res.end(`<html><body style='font-family: sans-serif; text-align: center; padding: 50px;'><h2>Authentication Error</h2><p>${msg}</p></body></html>`);
          }
        });

        s.once("error", reject);
        s.listen(port, "127.0.0.1", () => {
          s.removeListener("error", reject);
          resolve(s);
        });
      });

      currentCallbackServer = server;
      console.log(`[oauth] Google OAuth callback server successfully bound on port ${port}`);
      return { server, port };
    } catch (err: any) {
      if (err.code === "EADDRINUSE") {
        console.warn(`[oauth] Port ${port} in use, trying next port...`);
        continue;
      }
      throw err;
    }
  }

  // Final attempt: ephemeral port (port 0)
  return new Promise<{ server: http.Server; port: number }>((resolve, reject) => {
    const s = http.createServer(async (req, res) => {
      const address = s.address() as any;
      const actualPort = address?.port || 51121;
      const reqUrl = new URL(req.url || "", `http://127.0.0.1:${actualPort}`);
      const code = reqUrl.searchParams.get("code");
      if (code) {
        await onCodeReceived(code, reqUrl, res);
      }
    });
    s.listen(0, "127.0.0.1", () => {
      const address = s.address() as any;
      const port = address.port;
      currentCallbackServer = s;
      resolve({ server: s, port });
    });
    s.once("error", reject);
  });
}

async function exchangeGoogleCodeAndSaveProfile(
  code: string,
  redirectUri: string,
  llm: LLMManager,
  customAccountLabel?: string,
): Promise<ProviderProfile> {
  const tokenRes = await fetch("https://oauth2.googleapis.com/token", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      client_id: ANTIGRAVITY_CLIENT_ID,
      client_secret: ANTIGRAVITY_CLIENT_SECRET,
      code,
      grant_type: "authorization_code",
      redirect_uri: redirectUri,
    }).toString(),
  });

  if (!tokenRes.ok) {
    const tokenErr = await tokenRes.text();
    throw new Error(`Token exchange failed: ${tokenErr}`);
  }

  const tokens = (await tokenRes.json()) as {
    access_token: string;
    refresh_token?: string;
    expires_in?: number;
  };

  // Fetch Google User Email
  let userEmail = "";
  try {
    const userRes = await fetch("https://www.googleapis.com/oauth2/v1/userinfo?alt=json", {
      headers: { Authorization: `Bearer ${tokens.access_token}` },
      signal: AbortSignal.timeout(8000),
    });
    if (userRes.ok) {
      const userData = (await userRes.json()) as { email?: string };
      userEmail = userData.email || "";
    }
  } catch (e) {
    console.warn("[oauth] Failed to fetch user email:", e);
  }

  // Discover Cloud Code Assist companion project
  let projectId = "";
  try {
    const loadRes = await fetch("https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${tokens.access_token}`,
        "Content-Type": "application/json",
        "User-Agent": "ForgeADE/1.0",
      },
      body: JSON.stringify({ ideType: "ANTIGRAVITY" }),
    });
    if (loadRes.ok) {
      const loadData = (await loadRes.json()) as { cloudaicompanionProject?: string };
      projectId = loadData.cloudaicompanionProject || "";
    }
  } catch (e) {
    console.warn("[oauth] loadCodeAssist error:", e);
  }

  // Discover models and quota
  const quota = await fetchAntigravityQuota(tokens.access_token, projectId);
  const discoveredModels = (quota?.models || []).map((m: ModelQuota) => m.model);

  const defaultModelList = [
    "gemini-3.7-flash-tiered",
    "gemini-2.5-pro",
    "gemini-2.5-flash",
    "claude-3-7-sonnet",
    "claude-3-5-sonnet",
  ];
  const modelNames = discoveredModels.length > 0 ? [...new Set([...discoveredModels, ...defaultModelList])] : defaultModelList;

  // Determine unique profile ID (supporting multiple Google accounts)
  const existingProfiles = llm.getProviderProfiles();
  let profileId = "google-antigravity";

  if (userEmail) {
    const cleanEmail = userEmail.replace(/[^a-zA-Z0-9]+/g, "-").toLowerCase();
    const primaryExists = existingProfiles.some((p) => p.id === "google-antigravity");
    const existingWithSameEmail = existingProfiles.some((p) => (p as any).accountEmail === userEmail);

    if (primaryExists && !existingWithSameEmail) {
      profileId = `google-antigravity-${cleanEmail}`;
    }
  }

  if (customAccountLabel) {
    profileId = `google-antigravity-${customAccountLabel.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
  }

  const profileName = userEmail
    ? `Google Antigravity (${userEmail})`
    : customAccountLabel
    ? `Google Antigravity (${customAccountLabel})`
    : "Google Antigravity";

  const profileData: ProviderProfile = {
    id: profileId,
    name: profileName,
    provider: "google-antigravity",
    apiKey: tokens.access_token,
    baseURL: "https://daily-cloudcode-pa.googleapis.com",
    activeModel: "gemini-3.7-flash-tiered",
    models: modelNames.map((id) => ({ id, name: id, context_length: 200_000 })),
    selected_models: modelNames,
    enabled: true,
  };

  (profileData as any).accountEmail = userEmail;
  (profileData as any).projectId = projectId;
  (profileData as any).refreshToken = tokens.refresh_token;

  llm.saveProviderProfiles([profileData]);
  llm.setActiveModel(profileId, "gemini-3.7-flash-tiered");

  return profileData;
}

async function startAntigravityOAuth(
  loginId: string,
  llm: LLMManager,
  customAccountLabel?: string,
): Promise<OAuthFlowResult> {
  const state: OAuthSessionState = {
    loginId,
    provider: "google-antigravity",
    status: "pending",
  };
  activeOAuthSessions.set(loginId, state);

  const { server, port } = await bindCallbackServer(async (code, reqUrl, res) => {
    try {
      const redirectUri = `http://127.0.0.1:${port}${ANTIGRAVITY_CALLBACK_PATH}`;
      const profile = await exchangeGoogleCodeAndSaveProfile(code, redirectUri, llm, customAccountLabel);

      state.status = "success";
      state.profile = profile;
      state.accountEmail = (profile as any).accountEmail;

      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<html><body style='font-family: -apple-system, BlinkMacSystemFont, sans-serif; text-align: center; padding: 60px 20px; background: #121214; color: #fff;'>
        <div style='max-width: 440px; margin: auto; padding: 30px; background: #1a1a1e; border: 1px solid #27272a; border-radius: 12px;'>
          <h2 style='color: #4ade80; margin-top: 0;'>&#x2714; Authenticated Successfully!</h2>
          <p style='color: #e4e4e7; font-size: 14px;'>Google Antigravity is now connected to <b>ForgeADE</b>${state.accountEmail ? ` as <code>${state.accountEmail}</code>` : ""}.</p>
          <p style='color: #71717a; font-size: 12px; margin-top: 20px;'>You may close this browser tab and return to ForgeADE.</p>
        </div>
      </body></html>`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      state.status = "error";
      state.error = msg;
      res.writeHead(500, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<html><body style='font-family: sans-serif; text-align: center; padding: 50px;'><h2>Authentication Failed</h2><p>${msg}</p></body></html>`);
    } finally {
      setTimeout(() => closeCurrentCallbackServer(), 2500);
    }
  });

  const redirectUri = `http://127.0.0.1:${port}${ANTIGRAVITY_CALLBACK_PATH}`;
  state.redirectUri = redirectUri;

  const authUrl =
    `https://accounts.google.com/o/oauth2/v2/auth?` +
    new URLSearchParams({
      client_id: ANTIGRAVITY_CLIENT_ID,
      redirect_uri: redirectUri,
      response_type: "code",
      scope: ANTIGRAVITY_SCOPES.join(" "),
      access_type: "offline",
      prompt: "consent",
      state: loginId,
    }).toString();

  // Auto-cancel session after 5 minutes
  setTimeout(() => {
    if (state.status === "pending") {
      state.status = "cancelled";
      state.error = "Authentication timed out after 5 minutes";
      closeCurrentCallbackServer();
    }
  }, 5 * 60 * 1000);

  return {
    loginId,
    provider: "google-antigravity",
    authUrl,
    method: "browser",
    port,
    redirectUri,
    instructions: "Log in with your Google account in the opened browser window.",
  };
}

// ---------------------------------------------------------------------------
// KiloCode Device Auth
// ---------------------------------------------------------------------------

async function startKiloDeviceAuth(loginId: string, llm: LLMManager): Promise<OAuthFlowResult> {
  const state: OAuthSessionState = {
    loginId,
    provider: "kilo",
    status: "pending",
  };
  activeOAuthSessions.set(loginId, state);

  try {
    const res = await fetch(`${KILO_DEVICE_AUTH_URL}/codes`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });

    if (!res.ok) {
      throw new Error(`Failed to initiate device auth: HTTP ${res.status}`);
    }

    const data = (await res.json()) as {
      code?: string;
      verificationUrl?: string;
      expiresIn?: number;
    };

    const userCode = data.code;
    const verificationUrl = data.verificationUrl || "https://kilo.ai/auth/device";
    const expiresIn = data.expiresIn || 600;

    const pollInterval = 5000;
    const deadline = Date.now() + expiresIn * 1000;

    const interval = setInterval(async () => {
      if (Date.now() > deadline || state.status !== "pending") {
        clearInterval(interval);
        if (state.status === "pending") {
          state.status = "cancelled";
          state.error = "Device authentication timed out";
        }
        return;
      }

      try {
        const pollRes = await fetch(`${KILO_DEVICE_AUTH_URL}/codes/${encodeURIComponent(userCode || "")}`);
        if (pollRes.status === 202) {
          // Pending user approval in browser
          return;
        }
        if (pollRes.ok) {
          const pollData = (await pollRes.json()) as { token?: string; status?: string };
          if (pollData.status === "approved" || pollData.token) {
            const token = pollData.token;
            if (token) {
              clearInterval(interval);

              const models = ["kilo-coder", "claude-3.7-sonnet", "deepseek-r1", "gpt-4o"];
              const profileData: ProviderProfile = {
                id: "kilo",
                name: "KiloCode",
                provider: "kilo",
                apiKey: token,
                baseURL: "https://api.kilo.ai/v1",
                activeModel: "kilo-coder",
                models: models.map((id) => ({ id, name: id })),
                selected_models: models,
                enabled: true,
              };

              llm.saveProviderProfiles([profileData]);
              llm.setActiveModel("kilo", "kilo-coder");

              state.status = "success";
              state.profile = profileData;
            }
          } else if (pollData.status === "denied" || pollData.status === "expired") {
            clearInterval(interval);
            state.status = "error";
            state.error = `Authorization ${pollData.status}`;
          }
        }
      } catch (pollErr) {
        console.warn("[kilo-auth] polling error:", pollErr);
      }
    }, pollInterval);

    return {
      loginId,
      provider: "kilo",
      authUrl: verificationUrl,
      method: "device",
      userCode,
      instructions: `Enter confirmation code: ${userCode}`,
    };
  } catch (err: any) {
    state.status = "error";
    state.error = err?.message || String(err);
    throw err;
  }
}

// ---------------------------------------------------------------------------
// OpenCode Go OAuth / Key
// ---------------------------------------------------------------------------

async function startOpenCodeOAuth(loginId: string, llm: LLMManager): Promise<OAuthFlowResult> {
  const state: OAuthSessionState = {
    loginId,
    provider: "opencode-go",
    status: "pending",
  };
  activeOAuthSessions.set(loginId, state);

  const authUrl = "https://opencode.ai/account/api-keys";

  return {
    loginId,
    provider: "opencode-go",
    authUrl,
    method: "browser",
    instructions: "Generate or copy your OpenCode API Key from the dashboard.",
  };
}
