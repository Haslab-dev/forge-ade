import {
  ListBoardTasks as WailsListBoardTasks,
  GetBoardTask as WailsGetBoardTask,
  SaveBoardTask as WailsSaveBoardTask,
  DeleteBoardTask as WailsDeleteBoardTask,
  MoveBoardTaskStatus as WailsMoveBoardTaskStatus,
} from '../../lib/wails';
import type { BoardTask, BoardTaskStatus } from '../../types';

export class BoardApi {
  public static async listBoardTasks(wsFolder?: string): Promise<BoardTask[]> {
    const res = await WailsListBoardTasks(wsFolder || '');
    return Array.isArray(res) ? res : [];
  }

  public static async getBoardTask(idOrKey: string, wsFolder?: string): Promise<BoardTask | null> {
    const res = await WailsGetBoardTask(wsFolder || '', idOrKey);
    return res && typeof res === 'object' ? res : null;
  }

  public static async saveBoardTask(task: BoardTask, wsFolder?: string): Promise<BoardTask> {
    const res = await WailsSaveBoardTask(wsFolder || '', task);
    if (!res || typeof res !== 'object') {
      throw new Error(`Failed to save board task: ${task.id || task.key}`);
    }
    return res;
  }

  public static async deleteBoardTask(taskIdOrKey: string, wsFolder?: string): Promise<void> {
    await WailsDeleteBoardTask(wsFolder || '', taskIdOrKey);
  }

  public static async moveBoardTaskStatus(
    taskIdOrKey: string,
    status: BoardTaskStatus,
    wsFolder?: string
  ): Promise<BoardTask> {
    const res = await WailsMoveBoardTaskStatus(wsFolder || '', taskIdOrKey, status);
    if (!res || typeof res !== 'object') {
      throw new Error(`Failed to move board task status for ${taskIdOrKey}`);
    }
    return res;
  }
}
