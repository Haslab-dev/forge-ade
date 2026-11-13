import {
  ListPlugins as WailsListPlugins,
  GetPlugin as WailsGetPlugin,
  CreatePlugin as WailsCreatePlugin,
  TogglePlugin as WailsTogglePlugin,
  DeletePlugin as WailsDeletePlugin,
  ReloadPlugins as WailsReloadPlugins,
  ListSkills as WailsListSkills,
  CreateSkill as WailsCreateSkill,
  ReloadSkills as WailsReloadSkills,
  DeleteSkill as WailsDeleteSkill,
  DiscoverSkills as WailsDiscoverSkills,
  DiscoverMCPServers as WailsDiscoverMCPServers,
  ImportDiscoveredSkills as WailsImportDiscoveredSkills,
  ImportDiscoveredMCPServers as WailsImportDiscoveredMCPServers,
} from '../../lib/wails';
import type { PluginInfo, CreatePluginRequest, CreateSkillRequest } from '../../types';

export class IntegrationsApi {
  // ── Plugins ───────────────────────────────────────────────────────────────
  public static async listPlugins(): Promise<PluginInfo[]> {
    try {
      const list = await WailsListPlugins();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('listPlugins failed:', e);
      return [];
    }
  }

  public static async getPlugin(id: string): Promise<PluginInfo | null> {
    try {
      return await WailsGetPlugin(id);
    } catch (e) {
      console.warn('getPlugin failed:', e);
      return null;
    }
  }

  public static async createPlugin(req: CreatePluginRequest): Promise<PluginInfo | null> {
    return WailsCreatePlugin(req);
  }

  public static async togglePlugin(id: string, enabled: boolean): Promise<boolean> {
    try {
      await WailsTogglePlugin(id, enabled);
      return true;
    } catch (e) {
      console.warn('togglePlugin failed:', e);
      return false;
    }
  }

  public static async deletePlugin(id: string): Promise<boolean> {
    try {
      await WailsDeletePlugin(id);
      return true;
    } catch (e) {
      console.warn('deletePlugin failed:', e);
      return false;
    }
  }

  public static async reloadPlugins(): Promise<PluginInfo[]> {
    try {
      const list = await WailsReloadPlugins();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('reloadPlugins failed:', e);
      return [];
    }
  }

  // ── Skills ────────────────────────────────────────────────────────────────
  public static async listSkills(): Promise<any[]> {
    try {
      const list = await WailsListSkills();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('listSkills failed:', e);
      return [];
    }
  }

  public static async createSkill(req: CreateSkillRequest): Promise<any> {
    return WailsCreateSkill(req);
  }

  public static async deleteSkill(name: string): Promise<boolean> {
    try {
      await WailsDeleteSkill(name);
      return true;
    } catch (e) {
      console.warn('deleteSkill failed:', e);
      return false;
    }
  }

  public static async reloadSkills(): Promise<any[]> {
    try {
      const list = await WailsReloadSkills();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('reloadSkills failed:', e);
      return [];
    }
  }

  // ── Discovery ─────────────────────────────────────────────────────────────
  public static async discoverMcps(): Promise<any[]> {
    try {
      const list = await WailsDiscoverMCPServers();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('discoverMcps failed:', e);
      return [];
    }
  }

  public static async importDiscoveredMcps(names: string[]): Promise<void> {
    await WailsImportDiscoveredMCPServers(names);
  }

  public static async discoverSkills(): Promise<any[]> {
    try {
      const list = await WailsDiscoverSkills();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('discoverSkills failed:', e);
      return [];
    }
  }

  public static async importDiscoveredSkills(names: string[]): Promise<void> {
    await WailsImportDiscoveredSkills(names);
  }
}
