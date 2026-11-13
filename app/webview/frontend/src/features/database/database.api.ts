import {
  InspectDatabase as WailsInspectDatabase,
  GetDatabaseTableRows as WailsGetDatabaseTableRows,
  ExecuteDatabaseQuery as WailsExecuteDatabaseQuery,
} from '../../lib/wails';
import type { DatabaseInfo, QueryResult } from '../../../bindings/github.com/hasdev/forge-ade/internal/dbviewer/models';

export class DatabaseApi {
  public static async inspectDatabase(path: string): Promise<DatabaseInfo | null> {
    try {
      const info = await WailsInspectDatabase(path);
      return (info as DatabaseInfo) || null;
    } catch (e) {
      console.error('InspectDatabase failed:', e);
      return null;
    }
  }

  public static async getDatabaseTableRows(
    path: string,
    table: string,
    limit: number = 50,
    offset: number = 0,
    sortCol: string = '',
    sortAsc: boolean = true,
    filterCol: string = '',
    filterVal: string = ''
  ): Promise<QueryResult | null> {
    try {
      const res = await WailsGetDatabaseTableRows(path, table, limit, offset, sortCol, sortAsc, filterCol, filterVal);
      return (res as QueryResult) || null;
    } catch (e) {
      console.error('GetDatabaseTableRows failed:', e);
      return null;
    }
  }

  public static async executeDatabaseQuery(
    path: string,
    sqlQuery: string,
    limit: number = 500
  ): Promise<QueryResult | null> {
    try {
      const res = await WailsExecuteDatabaseQuery(path, sqlQuery, limit);
      return (res as QueryResult) || null;
    } catch (e) {
      console.error('ExecuteDatabaseQuery failed:', e);
      return null;
    }
  }
}
