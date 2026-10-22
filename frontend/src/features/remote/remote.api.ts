import {
  OpenSSHWorkspace,
  UploadFilesToRemote,
  OpenUploadFileDialog,
  OpenUploadFolderDialog,
  ConnectSSH as WailsConnectSSH,
  DisconnectSSH as WailsDisconnectSSH,
  ListSSHConnections as WailsListSSHConnections,
  UploadFileToRemote as WailsUploadFileToRemote,
  type SSHConfig,
  type SSHConnectionStatus,
} from '../../lib/wails';

export class RemoteApi {
  public static async connectSSH(cfg: SSHConfig): Promise<SSHConnectionStatus | null> {
    try {
      return await WailsConnectSSH(cfg);
    } catch (e) {
      console.error('ConnectSSH failed:', e);
      throw e;
    }
  }

  public static async disconnectSSH(connId: string): Promise<void> {
    try {
      await WailsDisconnectSSH(connId);
    } catch (e) {
      console.error('DisconnectSSH failed:', e);
    }
  }

  public static async listSSHConnections(): Promise<SSHConnectionStatus[]> {
    try {
      const res = await WailsListSSHConnections();
      return Array.isArray(res) ? res : [];
    } catch {
      return [];
    }
  }

  public static async openSSHWorkspace(cfg: SSHConfig): Promise<SSHConnectionStatus | null> {
    try {
      return await OpenSSHWorkspace(cfg);
    } catch (e) {
      console.error('OpenSSHWorkspace failed:', e);
      throw e;
    }
  }

  public static async uploadFileToRemote(localPath: string, remoteURI: string): Promise<void> {
    try {
      await WailsUploadFileToRemote(localPath, remoteURI);
    } catch (e) {
      console.error('UploadFileToRemote failed:', e);
      throw e;
    }
  }

  public static async uploadFilesToRemote(localPaths: string[], remoteDirURI: string): Promise<string[]> {
    try {
      return await UploadFilesToRemote(localPaths, remoteDirURI);
    } catch (e) {
      console.error('UploadFilesToRemote failed:', e);
      throw e;
    }
  }

  public static async openUploadFileDialog(): Promise<string[]> {
    try {
      const res = await OpenUploadFileDialog();
      return Array.isArray(res) ? res : [];
    } catch (e) {
      console.error('OpenUploadFileDialog failed:', e);
      return [];
    }
  }

  public static async openUploadFolderDialog(): Promise<string> {
    try {
      const res = await OpenUploadFolderDialog();
      return typeof res === 'string' ? res : '';
    } catch (e) {
      console.error('OpenUploadFolderDialog failed:', e);
      return '';
    }
  }
}
