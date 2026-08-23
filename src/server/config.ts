// Canonical configuration — mirrors the reference agent's layout:
//
//   ~/.forge-ade/models.json    providers (auth) + model catalog with
//                               per-model metadata (context_window,
//                               max_tokens, reasoning, input modalities)
//   ~/.forge-ade/mcp.json       MCP servers ("mcpServers" map)
//   ~/.forge-ade/skills.json    skill discovery options
//
// Legacy files (config.json, llm_config.json, providers_config.json,
// profiles.json, mcp_servers.json) are imported once, then renamed
// "*.migrated". Only the daemon writes these files; the frontend mutates
// configuration exclusively through bridge methods.

import fs from "fs";
import path from "path";
import os from "os";

export interface ModelMeta {
  id: string;
  name?: string | undefined;
  /** Model emits reasoning tokens. */
  reasoning?: boolean | undefined;
  input?: string[] | undefined;
  context_window?: number | undefined;
  max_tokens?: number | undefined;
}

export interface ProviderAuth {
  id: string;
  name: string;
  /** "openai-completions" | "anthropic" */
  api: string;
  base_url: string;
  api_key: string;
  auth: "apiKey";
  active_model: string;
  models: ModelMeta[];
  /** Curated subset shown in model pickers; defaults to the full catalog. */
  selected_models?: string[] | undefined;
  /** Disabled providers stay configured but are never auto-selected. */
  enabled?: boolean | undefined;
}

export interface SkillsFile {
  ignored: string[];
  extra_dirs: string[];
}

export interface ModelsFile {
  version: number;
  default_provider: string;
  providers: Record<string, ProviderAuth>;
}

const CONFIG_VERSION = 1;

function writeAtomic(file: string, data: unknown): void {
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify(data, null, 2), "utf-8");
  fs.renameSync(tmp, file);
}

/** Ensures a file exists with sensible empty content (idempotent). */
function touchJson(file: string, initial: unknown): void {
  try {
    if (!fs.existsSync(file)) {
      fs.mkdirSync(path.dirname(file), { recursive: true });
      fs.writeFileSync(file, JSON.stringify(initial, null, 2), "utf-8");
    }
  } catch (err) {
    console.error(`[config] failed to create ${file}:`, err);
  }
}

export class ConfigStore {
  readonly dataDir: string;
  revision = 0;

  private modelsFile: string;
  private skillsFile: string;
  readonly mcpPath: string;
  private _models!: ModelsFile;
  private _skills!: SkillsFile;

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    fs.mkdirSync(this.dataDir, { recursive: true });
    this.modelsFile = path.join(this.dataDir, "models.json");
    this.skillsFile = path.join(this.dataDir, "skills.json");
    this.mcpPath = path.join(this.dataDir, "mcp.json");

    // Read the superseded unified config before any migration renames it.
    const oldUnified = this.readJsonIfPresent<{ default_provider?: string; providers?: Record<string, ProviderAuth>; skills?: SkillsFile }>(
      path.join(this.dataDir, "config.json"),
    );

    this._models = this.loadModels(oldUnified ?? undefined);
    if (oldUnified) this.migrateLegacy("config.json");
    this._skills = this.loadSkills(oldUnified?.skills);
    touchJson(this.mcpPath, { mcpServers: {} });
  }

  /**
   * API keys known for a base URL, gathered from live AND previously-migrated
   * legacy files. Used to re-attach credentials when a save round-trip loses
   * them (e.g. the settings modal saving keyless opencode-imported entries).
   */
  knownKeysByUrl(): Map<string, string> {
    const map = new Map<string, string>();
    const note = (rec: Record<string, unknown>): void => {
      const url = this.str(rec, "base_url", "baseUrl", "baseURL");
      const apiKey = this.str(rec, "api_key", "apiKey");
      if (url && apiKey && !map.has(url)) map.set(url, apiKey);
    };
    const walk = (value: unknown): void => {
      if (Array.isArray(value)) {
        for (const e of value) if (e && typeof e === "object") note(e as Record<string, unknown>);
      } else if (value && typeof value === "object") {
        const rec = value as { providers?: Record<string, unknown>; profiles?: unknown };
        if (rec.providers) {
          for (const p of Object.values(rec.providers)) {
            if (p && typeof p === "object") note(p as Record<string, unknown>);
          }
        }
        if (rec.profiles !== undefined) walk(rec.profiles);
      }
    };
    for (const name of ["config.json.migrated", "providers_config.json.migrated", "llm_config.json.migrated", "profiles.json.migrated"]) {
      const data = this.readJsonIfPresent<unknown>(path.join(this.dataDir, name));
      if (data) walk(data);
    }
    return map;
  }

  // ---------------------------------------------------------------------------
  // Access
  // ---------------------------------------------------------------------------

  getModels(): ModelsFile {
    return this._models;
  }

  getProviders(): Record<string, ProviderAuth> {
    return this._models.providers;
  }

  defaultProvider(): ProviderAuth | null {
    const enabled = (p: ProviderAuth): boolean => p.enabled !== false;
    const id = this._models.default_provider;
    if (id && this._models.providers[id] && enabled(this._models.providers[id])) {
      return this._models.providers[id];
    }
    return Object.values(this._models.providers).find(enabled) ?? null;
  }

  saveModels(mutate: (models: ModelsFile) => void): void {
    mutate(this._models);
    writeAtomic(this.modelsFile, this._models);
    this.revision += 1;
  }

  getSkills(): SkillsFile {
    return this._skills;
  }

  saveSkills(mutate: (skills: SkillsFile) => void): void {
    mutate(this._skills);
    writeAtomic(this.skillsFile, this._skills);
    this.revision += 1;
  }

  // ---------------------------------------------------------------------------
  // Loading + migration
  // ---------------------------------------------------------------------------

  private readJsonIfPresent<T>(file: string): T | null {
    try {
      if (!fs.existsSync(file)) return null;
      return JSON.parse(fs.readFileSync(file, "utf-8")) as T;
    } catch (err) {
      console.error(`[config] failed to read ${path.basename(file)}:`, err);
      return null;
    }
  }

  /** Renames a legacy file out of the way so nothing re-imports it. */
  private migrateLegacy(name: string): void {
    const legacyPath = path.join(this.dataDir, name);
    try {
      if (fs.existsSync(legacyPath)) {
        fs.renameSync(legacyPath, `${legacyPath}.migrated`);
        console.log(`[config] migrated legacy ${name}`);
      }
    } catch (err) {
      console.error(`[config] failed to migrate ${name}:`, err);
    }
  }

  private str(rec: Record<string, unknown>, ...keys: string[]): string {
    for (const k of keys) {
      const v = rec[k];
      if (typeof v === "string" && v.length > 0) return v;
    }
    return "";
  }

  /** Normalizes any known provider-record shape into ours. */
  private normalizeProvider(rec: Record<string, unknown>): ProviderAuth | null {
    const id = this.str(rec, "provider_id", "id");
    const api_key = this.str(rec, "api_key", "apiKey", "ApiKey");
    const base_url = this.str(rec, "base_url", "baseUrl", "baseURL", "BaseURL");
    if (!id || !api_key || !base_url) return null;
    const rawModels: unknown[] = Array.isArray(rec.selected_models)
      ? rec.selected_models
      : Array.isArray(rec.available_models)
        ? rec.available_models
        : Array.isArray(rec.models)
          ? rec.models
          : [];
    const models: ModelMeta[] = [];
    for (const m of rawModels) {
      if (typeof m === "string") {
        models.push({ id: m });
        continue;
      }
      if (m && typeof m === "object") {
        const mm = m as Record<string, unknown>;
        const mid = typeof mm.id === "string" ? mm.id : "";
        if (!mid) continue;
        models.push({
          id: mid,
          ...(typeof mm.name === "string" ? { name: mm.name } : {}),
          ...(typeof mm.reasoning === "boolean" ? { reasoning: mm.reasoning } : {}),
          ...(Array.isArray(mm.input) ? { input: mm.input.filter((x): x is string => typeof x === "string") } : {}),
          ...(typeof mm.contextWindow === "number" ? { context_window: mm.contextWindow } : {}),
          ...(typeof mm.context_window === "number" ? { context_window: mm.context_window } : {}),
          ...(typeof mm.maxTokens === "number" ? { max_tokens: mm.maxTokens } : {}),
          ...(typeof mm.max_tokens === "number" ? { max_tokens: mm.max_tokens } : {}),
        });
      }
    }
    const activeModel =
      this.str(rec, "model", "active_model", "activeModel") || models[0]?.id || "";
    if (models.length === 0 && activeModel) models.push({ id: activeModel });
    return {
      id,
      name: this.str(rec, "name") || id,
      api: this.str(rec, "api") || (this.str(rec, "type") === "anthropic" ? "anthropic" : "openai-completions"),
      base_url,
      api_key,
      auth: "apiKey",
      active_model: activeModel,
      models,
    };
  }

  private adoptProvider(models: Record<string, ProviderAuth>, provider: ProviderAuth): void {
    const existing = models[provider.id];
    if (!existing) {
      models[provider.id] = provider;
      return;
    }
    // Merge: keep the richer model catalog.
    if (provider.models.length > existing.models.length) existing.models = provider.models;
    if (!existing.active_model && provider.active_model) existing.active_model = provider.active_model;
  }

  private loadModels(legacyUnified?: { default_provider?: string; providers?: Record<string, ProviderAuth> }): ModelsFile {
    const existing = this.readJsonIfPresent<ModelsFile>(this.modelsFile);
    if (existing && existing.providers && Object.keys(existing.providers).length > 0) {
      // An already-populated catalog is authoritative; empty catalogs re-import.
      return { version: CONFIG_VERSION, default_provider: existing.default_provider ?? "", providers: existing.providers };
    }

    // Known credentials by base URL — keyless foreign imports (e.g. opencode
    // local routers) inherit the key we already hold for the same endpoint.
    // Scans both live and previously-migrated legacy files.
    const keyByUrl = new Map<string, string>();
    const noteKey = (rec: Record<string, unknown>): void => {
      const url = this.str(rec, "base_url", "baseUrl", "baseURL");
      const apiKey = this.str(rec, "api_key", "apiKey");
      if (url && apiKey && !keyByUrl.has(url)) keyByUrl.set(url, apiKey);
    };
    const noteKeyFromRecords = (value: unknown): void => {
      if (Array.isArray(value)) {
        for (const e of value) if (e && typeof e === "object") noteKey(e as Record<string, unknown>);
      } else if (value && typeof value === "object") {
        const rec = value as { providers?: Record<string, unknown>; profiles?: unknown };
        if (rec.providers) {
          for (const p of Object.values(rec.providers)) {
            if (p && typeof p === "object") noteKey(p as Record<string, unknown>);
          }
        }
        if (rec.profiles !== undefined) noteKeyFromRecords(rec.profiles);
      }
    };
    for (const name of ["config.json", "providers_config.json", "llm_config.json", "profiles.json"]) {
      const live = this.readJsonIfPresent<unknown>(path.join(this.dataDir, name));
      if (live) noteKeyFromRecords(live);
      const migrated = this.readJsonIfPresent<unknown>(path.join(this.dataDir, `${name}.migrated`));
      if (migrated) noteKeyFromRecords(migrated);
    }
    if (legacyUnified?.providers) {
      for (const rec of Object.values(legacyUnified.providers)) noteKey(rec as unknown as Record<string, unknown>);
    }

    const fresh: ModelsFile = { version: CONFIG_VERSION, default_provider: "", providers: {} };
    // Seed from our own previous unified config.json (richest for us),
    // normalized so records carry the canonical keys (api, models[]...).
    if (legacyUnified?.providers) {
      for (const rec of Object.values(legacyUnified.providers)) {
        noteKey(rec as unknown as Record<string, unknown>);
        const provider = this.normalizeProvider(rec as unknown as Record<string, unknown>);
        if (provider) this.adoptProvider(fresh.providers, provider);
      }
      fresh.default_provider = legacyUnified.default_provider ?? "";
    }

    // providers_config.json — settings-UI export.
    const legacyProviders = this.readJsonIfPresent<unknown[]>(path.join(this.dataDir, "providers_config.json"));
    if (Array.isArray(legacyProviders)) {
      for (const entry of legacyProviders) {
        if (!entry || typeof entry !== "object") continue;
        noteKey(entry as Record<string, unknown>);
        const provider = this.normalizeProvider(entry as Record<string, unknown>);
        if (provider) this.adoptProvider(fresh.providers, provider);
      }
      this.migrateLegacy("providers_config.json");
    }


    // llm_config.json — previous server-owned shape.
    const legacyLlm = this.readJsonIfPresent<{
      activeProfileId?: string;
      profiles?: Array<Record<string, unknown>>;
      provider_id?: string;
      model?: string;
    }>(path.join(this.dataDir, "llm_config.json"));
    if (legacyLlm) {
      if (Array.isArray(legacyLlm.profiles)) {
        for (const entry of legacyLlm.profiles) {
          const provider = this.normalizeProvider(entry);
          if (provider) this.adoptProvider(fresh.providers, provider);
        }
        if (legacyLlm.activeProfileId && fresh.providers[legacyLlm.activeProfileId]) {
          fresh.default_provider = legacyLlm.activeProfileId;
        }
      } else if (legacyLlm.provider_id && fresh.providers[legacyLlm.provider_id] && legacyLlm.model) {
        fresh.providers[legacyLlm.provider_id].active_model = legacyLlm.model;
      }
      this.migrateLegacy("llm_config.json");
    }

    // profiles.json — bare remnant of the Wails-era app.
    const legacyBare = this.readJsonIfPresent<Record<string, unknown>>(path.join(this.dataDir, "profiles.json"));
    if (legacyBare && !Array.isArray(legacyBare)) {
      const provider = this.normalizeProvider(legacyBare);
      if (provider) this.adoptProvider(fresh.providers, provider);
      this.migrateLegacy("profiles.json");
    }

    // opencode.jsonc / opencode.json — native compat: same "provider" map
    // shape opencode reads (options.baseURL/apiKey, models map). FOREIGN files:
    // read-only import, never renamed.
    const ocPaths = [
      path.join(os.homedir(), ".config", "opencode", "opencode.jsonc"),
      path.join(os.homedir(), ".config", "opencode", "opencode.json"),
      path.join(process.cwd(), ".opencode", "opencode.json"),
      path.join(process.cwd(), ".opencode", "opencode.jsonc"),
    ];
    for (const ocPath of ocPaths) {
      const jsoncText = this.stripJsonc(ocPath);
      if (!jsoncText) continue;
      let parsed: Record<string, unknown> | null = null;
      try {
        const value: unknown = JSON.parse(jsoncText);
        if (value && typeof value === "object") parsed = value as Record<string, unknown>;
      } catch {}
      if (!parsed) continue;
      const providers = parsed.provider;
      if (!providers || typeof providers !== "object") continue;
      let imported = 0;
      for (const [id, entry] of Object.entries(providers as Record<string, unknown>)) {
        if (!entry || typeof entry !== "object") continue;
        const rec = entry as Record<string, unknown>;
        const options = (rec.options ?? {}) as Record<string, unknown>;
        const base_url = typeof options.baseURL === "string" ? options.baseURL : "";
        if (!base_url) continue;
        const apiKey = (typeof options.apiKey === "string" && options.apiKey) || keyByUrl.get(base_url) || "";
        const modelMap = rec.models && typeof rec.models === "object"
          ? (rec.models as Record<string, { name?: unknown }>)
          : {};
        const modelIds = Object.keys(modelMap);
        fresh.providers[id] = {
          id,
          name: typeof rec.name === "string" ? rec.name : id,
          api: id === "anthropic" ? "anthropic" : "openai-completions",
          base_url,
          api_key: apiKey,
          auth: "apiKey",
          active_model: modelIds[0] ?? "",
          models:
            modelIds.length > 0
              ? modelIds.map((mid) => ({
                  id: mid,
                  ...(typeof modelMap[mid]?.name === "string" ? { name: modelMap[mid].name as string } : {}),
                }))
              : fresh.providers[id]?.models ?? [],
        };
        imported += 1;
      }
      if (imported > 0) console.log(`[config] imported ${imported} provider(s) from ${path.basename(ocPath)} (opencode)`);
    }

    if (fresh.default_provider === "" || !fresh.providers[fresh.default_provider]) {
      // Prefer a provider that is fully usable (key + model), then one with a
      // model catalog, then any keyed provider, then the first entry.
      const ranked = Object.values(fresh.providers);
      const ready = ranked.find((p) => p.api_key.length > 0 && p.active_model.length > 0);
      const withModel = ranked.find((p) => p.models.length > 0);
      const fallback = ranked.find((p) => p.api_key.length > 0);
      fresh.default_provider = ready?.id ?? withModel?.id ?? fallback?.id ?? Object.keys(fresh.providers)[0] ?? "";
    }

    writeAtomic(this.modelsFile, fresh);
    console.log(`[config] wrote ${this.modelsFile} (${Object.keys(fresh.providers).length} providers)`);
    return fresh;
  }

  /** Strips // and /* *​/ comments plus trailing commas from JSONC files. */
  private stripJsonc(file: string): string | null {
    let text: string;
    try {
      text = fs.readFileSync(file, "utf-8");
    } catch {
      return null;
    }
    let out = "";
    let inStr = false;
    let esc = false;
    for (let i = 0; i < text.length; i++) {
      const c = text[i];
      if (inStr) {
        out += c;
        if (esc) esc = false;
        else if (c === "\\") esc = true;
        else if (c === '"') inStr = false;
        continue;
      }
      if (c === '"') {
        inStr = true;
        out += c;
        continue;
      }
      if (c === "/" && text[i + 1] === "/") {
        while (i < text.length && text[i] !== "\n") i++;
        out += "\n";
        continue;
      }
      if (c === "/" && text[i + 1] === "*") {
        i += 2;
        while (i < text.length && !(text[i] === "*" && text[i + 1] === "/")) i++;
        i += 1;
        continue;
      }
      out += c;
    }
    return out.replace(/,\s*([}\]])/g, "$1");
  }

  private loadSkills(adopted?: SkillsFile): SkillsFile {
    const existing = this.readJsonIfPresent<SkillsFile>(this.skillsFile);
    if (existing) return { ignored: existing.ignored ?? [], extra_dirs: existing.extra_dirs ?? [] };
    const fresh = adopted ?? { ignored: [], extra_dirs: [] };
    writeAtomic(this.skillsFile, fresh);
    return fresh;
  }
}

