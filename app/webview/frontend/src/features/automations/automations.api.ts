import {
  ListAutomations as WailsListAutomations,
  SaveAutomation as WailsSaveAutomation,
  DeleteAutomation as WailsDeleteAutomation,
  RecordAutomationRun as WailsRecordAutomationRun,
  ClaimAutomationFire as WailsClaimAutomationFire,
  SetAutomationEnabled as WailsSetAutomationEnabled,
  type Automation as ZAutomation,
  type AutomationSaveInput as ZAutomationSaveInput,
} from '../../lib/wails';

export class AutomationsApi {
  public static async listAutomations(): Promise<ZAutomation[]> {
    try {
      const list = await WailsListAutomations();
      return Array.isArray(list) ? list : [];
    } catch (e) {
      console.warn('listAutomations failed:', e);
      return [];
    }
  }

  public static async saveAutomation(automation: ZAutomationSaveInput): Promise<ZAutomation> {
    return WailsSaveAutomation(automation);
  }

  public static async deleteAutomation(id: string): Promise<void> {
    return WailsDeleteAutomation(id);
  }

  public static async recordAutomationRun(id: string, sessionId: string, workspace: string): Promise<ZAutomation> {
    return WailsRecordAutomationRun(id, sessionId, workspace);
  }

  public static async claimAutomationFire(id: string, token: string): Promise<boolean> {
    try {
      return await WailsClaimAutomationFire(id, token);
    } catch {
      return false;
    }
  }

  public static async setAutomationEnabled(id: string, enabled: boolean): Promise<ZAutomation> {
    return WailsSetAutomationEnabled(id, enabled);
  }
}
