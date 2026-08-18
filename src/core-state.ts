import { Cmd, Sub, utf8Bytes } from "@native-sdk/core";

export type ThemeMode = "dark" | "light";

export interface Model {
  readonly title: Uint8Array;
  readonly status: Uint8Array;
  readonly activeWorkspace: Uint8Array;
  readonly sessionCount: number;
  readonly ready: boolean;
  readonly theme: ThemeMode;
  readonly lastTickMs: number;
}

export type Msg =
  | { readonly kind: "ready" }
  | { readonly kind: "set_title"; readonly title: Uint8Array }
  | { readonly kind: "set_workspace"; readonly path: Uint8Array }
  | { readonly kind: "set_theme"; readonly theme: ThemeMode }
  | { readonly kind: "increment_sessions" }
  | { readonly kind: "decrement_sessions" }
  | { readonly kind: "tick"; readonly at: number }
  | { readonly kind: "command"; readonly id: Uint8Array };

export const viewUnbound = [
  "title",
  "status",
  "activeWorkspace",
  "sessionCount",
  "ready",
  "theme",
  "lastTickMs",
  "set_title",
  "set_workspace",
  "set_theme",
  "increment_sessions",
  "decrement_sessions",
  "tick",
  "command",
] as const;

export function initialModel(): Model {
  return {
    title: utf8Bytes("ForgeADE — Native AI Development Workspace"),
    status: utf8Bytes("Ready"),
    activeWorkspace: utf8Bytes(""),
    sessionCount: 0,
    ready: true,
    theme: "dark",
    lastTickMs: 0,
  };
}

export function commandMsg(name: string): Msg | null {
  if (name === "app.ready") {
    return { kind: "ready" };
  }
  if (name === "app.theme_dark") {
    return { kind: "set_theme", theme: "dark" };
  }
  if (name === "app.theme_light") {
    return { kind: "set_theme", theme: "light" };
  }
  return { kind: "command", id: utf8Bytes(name) };
}

export function update(model: Model, msg: Msg): Model | [Model, Cmd<Msg>] {
  switch (msg.kind) {
    case "ready":
      return { ...model, ready: true, status: utf8Bytes("ForgeADE Ready") };
    case "set_title":
      return { ...model, title: msg.title };
    case "set_workspace":
      return { ...model, activeWorkspace: msg.path };
    case "set_theme":
      return { ...model, theme: msg.theme };
    case "increment_sessions":
      return {
        ...model,
        sessionCount: model.sessionCount < 1000 ? model.sessionCount + 1 : model.sessionCount,
      };
    case "decrement_sessions":
      return {
        ...model,
        sessionCount: model.sessionCount > 0 ? model.sessionCount - 1 : 0,
      };
    case "tick":
      return { ...model, lastTickMs: msg.at };
    case "command":
      return model;
  }
}

export function subscriptions(model: Model): Sub<Msg> {
  if (!model.ready) return Sub.none;
  return Sub.timer("heartbeat", 5000, "tick");
}
