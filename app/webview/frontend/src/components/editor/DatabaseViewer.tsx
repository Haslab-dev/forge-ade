import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Database,
  Table as TableIcon,
  Play,
  Search,
  RefreshCw,
  Copy,
  Check,
  Download,
  Key,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Layers,
  FileCode,
  Terminal,
  AlertCircle,
  X,
  ShieldCheck,
  Cpu,
  HardDrive,
  FileSpreadsheet
} from 'lucide-react';
import { ApiBridge, ApiBridgeEffect } from '../../services/apiBridge';
import { useRunEffect, runPromiseExit, Cause } from '../../effect';
import type { DatabaseInfo, QueryResult, TableInfo } from '../../../bindings/github.com/hasdev/forge-ade/internal/dbviewer/models';

interface DatabaseViewerProps {
  filePath: string;
  fileName: string;
}

export const DatabaseViewer: React.FC<DatabaseViewerProps> = ({ filePath, fileName }) => {
  // Database metadata & inspection
  const [dbInfo, setDbInfo] = useState<DatabaseInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Navigation & View mode
  const [activeTable, setActiveTable] = useState<string>('');
  const [viewMode, setViewMode] = useState<'data' | 'schema' | 'query'>('data');
  const [tableSearch, setTableSearch] = useState('');

  // Table Data State
  const [rows, setRows] = useState<Record<string, any>[]>([]);
  const [columns, setColumns] = useState<string[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [dataLoading, setDataLoading] = useState(false);
  const [dataError, setDataError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [sortCol, setSortCol] = useState('');
  const [sortAsc, setSortAsc] = useState(true);
  const [filterCol, setFilterCol] = useState('');
  const [filterVal, setFilterVal] = useState('');

  // Query Console State
  const [querySql, setQuerySql] = useState('');
  const [queryResult, setQueryResult] = useState<QueryResult | null>(null);
  const [queryLoading, setQueryLoading] = useState(false);

  // Inspector & Feedback
  const [inspectCell, setInspectCell] = useState<{ col: string; val: any } | null>(null);
  const [copiedCell, setCopiedCell] = useState(false);
  const [copiedDDL, setCopiedDDL] = useState(false);
  const [copiedInstall, setCopiedInstall] = useState(false);

  // ── Effect-based Database Loader ──────────────────────────────────────────
  const { execute: runLoadDatabase } = useRunEffect(
    () => ApiBridgeEffect.inspectDatabase(filePath),
    [filePath]
  );

  const loadDatabase = useCallback(async () => {
    if (!filePath) return;
    setLoading(true);
    setError(null);
    try {
      const exit = await runLoadDatabase();
      if (exit._tag === 'Success') {
        const info = exit.value;
        if (!info) {
          setError('Failed to inspect database');
          return;
        }
        setDbInfo(info);

        // Select default table if none selected or not present
        const allTables = [...(info.tables || []), ...(info.views || [])];
        if (allTables.length > 0) {
          setActiveTable((prev) => {
            if (prev && allTables.some((t) => t.name === prev)) return prev;
            return allTables[0].name;
          });
        }
      } else {
        const errResult = Cause.findError(exit.cause);
        const err = errResult._tag === 'Success' ? ((errResult as any).success ?? (errResult as any).value) : exit.cause;
        setError((err as any)?.message || 'Error opening database');
      }
    } finally {
      setLoading(false);
    }
  }, [filePath, runLoadDatabase]);

  useEffect(() => {
    void loadDatabase();
  }, [loadDatabase]);

  // Current active table info
  const currentTableInfo: TableInfo | undefined = useMemo(() => {
    if (!dbInfo) return undefined;
    const all = [...(dbInfo.tables || []), ...(dbInfo.views || [])];
    return all.find((t) => t.name === activeTable);
  }, [dbInfo, activeTable]);

  // Update default query when active table changes
  useEffect(() => {
    if (activeTable) {
      setQuerySql(`SELECT * FROM "${activeTable.replace(/"/g, '""')}" LIMIT 50;`);
    }
  }, [activeTable]);

  // ── Effect-based Table Data Loader (with fiber cancellation on dependency change)
  const { execute: runLoadTableData } = useRunEffect(
    () => {
      const offset = (page - 1) * pageSize;
      return ApiBridgeEffect.getDatabaseTableRows(
        filePath,
        activeTable,
        pageSize,
        offset,
        sortCol,
        sortAsc,
        filterCol,
        filterVal
      );
    },
    [filePath, activeTable, page, pageSize, sortCol, sortAsc, filterCol, filterVal]
  );

  // ── Load table data ────────────────────────────────────────────────────────
  const loadTableData = useCallback(async () => {
    if (!activeTable || dbInfo?.type === 'rocksdb' || !filePath) return;
    setDataLoading(true);
    setDataError(null);
    try {
      const exit = await runLoadTableData();
      if (exit._tag === 'Success') {
        const res = exit.value;
        if (!res) {
          setDataError('No data returned from database engine');
          return;
        }
        if (res.error) {
          setDataError(res.error);
          return;
        }
        setColumns(res.columns || []);
        setRows((res.rows as Record<string, any>[]) || []);
        setTotalRows(res.totalRows || 0);
      } else {
        const errResult = Cause.findError(exit.cause);
        const err = errResult._tag === 'Success' ? ((errResult as any).success ?? (errResult as any).value) : exit.cause;
        setDataError((err as any)?.message || 'Error querying table data');
      }
    } finally {
      setDataLoading(false);
    }
  }, [activeTable, dbInfo?.type, filePath, runLoadTableData]);

  useEffect(() => {
    if (viewMode === 'data') {
      void loadTableData();
    }
  }, [viewMode, loadTableData]);

  // Reset page when switching tables or changing filter
  const handleSelectTable = (tblName: string) => {
    setActiveTable(tblName);
    setPage(1);
    setFilterVal('');
    setFilterCol('');
    setSortCol('');
  };

  // Sort handler
  const handleSort = (col: string) => {
    if (sortCol === col) {
      if (sortAsc) {
        setSortAsc(false);
      } else {
        setSortCol('');
        setSortAsc(true);
      }
    } else {
      setSortCol(col);
      setSortAsc(true);
    }
    setPage(1);
  };

  // ── Query Console Runner ──────────────────────────────────────────────────
  const handleRunQuery = async () => {
    if (!querySql.trim() || !filePath) return;
    setQueryLoading(true);
    try {
      const exit = await runPromiseExit(ApiBridgeEffect.executeDatabaseQuery(filePath, querySql.trim(), 500));
      if (exit._tag === 'Success') {
        setQueryResult(exit.value);
      } else {
        const errResult = Cause.findError(exit.cause);
        const err = errResult._tag === 'Success' ? ((errResult as any).success ?? (errResult as any).value) : exit.cause;
        setQueryResult({
          columns: [],
          rows: [],
          totalRows: 0,
          executionMs: 0,
          error: (err as any)?.message || 'Failed to execute query'
        });
      }
    } finally {
      setQueryLoading(false);
    }
  };

  // ── CSV & JSON Exporters ───────────────────────────────────────────────────
  const exportData = (format: 'csv' | 'json', source: 'table' | 'query') => {
    const cols = source === 'table' ? columns : queryResult?.columns || [];
    const dataRows = source === 'table' ? rows : (queryResult?.rows as Record<string, any>[]) || [];

    if (!dataRows || dataRows.length === 0) return;

    let content = '';
    let mimeType = 'text/plain';
    let ext = 'txt';

    if (format === 'json') {
      content = JSON.stringify(dataRows, null, 2);
      mimeType = 'application/json';
      ext = 'json';
    } else {
      mimeType = 'text/csv';
      ext = 'csv';
      const headerLine = cols.map((c) => `"${c.replace(/"/g, '""')}"`).join(',');
      const rowLines = dataRows.map((r) =>
        cols
          .map((c) => {
            const val = r[c];
            if (val === null || val === undefined) return '';
            const str = typeof val === 'object' ? JSON.stringify(val) : String(val);
            return `"${str.replace(/"/g, '""')}"`;
          })
          .join(',')
      );
      content = [headerLine, ...rowLines].join('\n');
    }

    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${source === 'table' ? activeTable : 'query_result'}_${Date.now()}.${ext}`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  // Formatted file size helper
  const formatFileSize = (bytes: number): string => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  };

  // Filtered tables list
  const filteredTables = useMemo(() => {
    if (!dbInfo?.tables) return [];
    if (!tableSearch.trim()) return dbInfo.tables;
    const q = tableSearch.toLowerCase();
    return dbInfo.tables.filter((t) => t.name.toLowerCase().includes(q));
  }, [dbInfo?.tables, tableSearch]);

  const filteredViews = useMemo(() => {
    if (!dbInfo?.views) return [];
    if (!tableSearch.trim()) return dbInfo.views;
    const q = tableSearch.toLowerCase();
    return dbInfo.views.filter((v) => v.name.toLowerCase().includes(q));
  }, [dbInfo?.views, tableSearch]);

  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));

  // ── Render ────────────────────────────────────────────────────────────────
  if (loading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center gap-3 bg-panel text-fg-secondary select-none">
        <RefreshCw className="w-6 h-6 animate-spin text-accent-primary" />
        <div className="text-ui-base font-medium">Inspecting database schema…</div>
        <div className="text-ui-xs text-fg-tertiary">{fileName}</div>
      </div>
    );
  }

  if (error || !dbInfo) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center gap-3 bg-panel text-fg-secondary p-8 select-none">
        <div className="p-3 rounded-full bg-status-danger/10 text-status-danger">
          <AlertCircle className="w-8 h-8" />
        </div>
        <div className="text-ui-lg font-semibold text-fg-primary">Failed to Open Database</div>
        <div className="text-ui-sm text-fg-tertiary max-w-md text-center">{error}</div>
        <button
          type="button"
          onClick={() => void loadDatabase()}
          className="mt-2 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-foreground text-background text-ui-sm font-medium hover:opacity-90 transition-opacity"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          Retry
        </button>
      </div>
    );
  }

  // Fallback / Guidance screen for DuckDB when not installed
  if (dbInfo.type === 'duckdb' && dbInfo.driver === 'none') {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-8 bg-panel text-fg-primary">
        <div className="max-w-lg w-full bg-surface border border-border rounded-xl p-6 shadow-sm flex flex-col gap-4">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-lg bg-accent-primary/10 text-accent-primary">
              <Database className="w-6 h-6" />
            </div>
            <div>
              <h2 className="text-ui-lg font-semibold text-fg-primary">DuckDB Database Detected</h2>
              <div className="text-ui-xs text-fg-tertiary">
                {fileName} • {formatFileSize(dbInfo.size)}
              </div>
            </div>
          </div>

          <p className="text-ui-sm text-fg-secondary leading-relaxed">
            {dbInfo.notes || 'DuckDB database file detected.'}
          </p>

          <div className="p-3 rounded-lg bg-panel border border-border flex flex-col gap-2">
            <div className="text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">
              Installation Instructions
            </div>
            <pre className="text-ui-xs text-fg-primary font-mono whitespace-pre-wrap select-all">
              {dbInfo.installHint || 'brew install duckdb\n# or: pip install duckdb'}
            </pre>
          </div>

          <div className="flex items-center justify-between pt-2">
            <button
              type="button"
              onClick={() => {
                if (dbInfo.installHint) {
                  void navigator.clipboard.writeText(dbInfo.installHint);
                  setCopiedInstall(true);
                  setTimeout(() => setCopiedInstall(false), 2000);
                }
              }}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-surface hover:bg-surface-hover border border-border text-ui-sm text-fg-secondary transition-colors"
            >
              {copiedInstall ? <Check className="w-3.5 h-3.5 text-status-success" /> : <Copy className="w-3.5 h-3.5" />}
              {copiedInstall ? 'Copied' : 'Copy Commands'}
            </button>
            <button
              type="button"
              onClick={() => void loadDatabase()}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-foreground text-background text-ui-sm font-medium hover:opacity-90 transition-opacity"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              Recheck Engine
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Fallback screen for RocksDB / LevelDB LSM-Tree stores
  if (dbInfo.type === 'rocksdb') {
    return (
      <div className="flex-1 flex flex-col p-6 bg-panel overflow-y-auto">
        <div className="max-w-3xl mx-auto w-full flex flex-col gap-6">
          {/* Header Card */}
          <div className="bg-surface border border-border rounded-xl p-5 shadow-sm flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-lg bg-accent-primary/10 text-accent-primary">
                  <HardDrive className="w-6 h-6" />
                </div>
                <div>
                  <h1 className="text-ui-xl font-bold text-fg-primary">{fileName}</h1>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="px-1.5 py-0.5 rounded text-ui-xs font-medium bg-accent-primary/15 text-accent-primary">
                      RocksDB / LevelDB
                    </span>
                    <span className="text-ui-xs text-fg-tertiary">•</span>
                    <span className="text-ui-xs text-fg-secondary">{formatFileSize(dbInfo.size)}</span>
                  </div>
                </div>
              </div>

              <button
                type="button"
                onClick={() => void loadDatabase()}
                className="p-1.5 rounded-md border border-border hover:bg-surface-hover text-fg-secondary transition-colors"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
            </div>

            <p className="text-ui-sm text-fg-secondary leading-relaxed">{dbInfo.notes}</p>
          </div>

          {/* Details breakdown */}
          <div className="bg-surface border border-border rounded-xl p-5 flex flex-col gap-4">
            <h2 className="text-ui-base font-semibold text-fg-primary flex items-center gap-2">
              <Layers className="w-4 h-4 text-accent-primary" />
              Store Structure & SSTable Manifest
            </h2>

            {dbInfo.tables && dbInfo.tables.length > 0 ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {dbInfo.tables[0].columns?.map((c, i) => (
                  <div key={i} className="p-3 rounded-lg bg-panel border border-border flex flex-col gap-1">
                    <span className="text-ui-xs text-fg-tertiary font-medium">{c.name}</span>
                    <span className="text-ui-sm font-mono text-fg-primary">{c.type || '—'}</span>
                  </div>
                ))}
              </div>
            ) : null}

            {dbInfo.installHint && (
              <div className="p-3 rounded-lg bg-panel border border-border flex flex-col gap-1.5 mt-2">
                <span className="text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">CLI Inspection</span>
                <pre className="text-ui-xs font-mono text-fg-primary whitespace-pre-wrap select-all">
                  {dbInfo.installHint}
                </pre>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col h-full bg-panel overflow-hidden select-none">
      {/* ── Top Toolbar ───────────────────────────────────────────────────── */}
      <header className="h-11 min-h-[44px] px-3 border-b border-border bg-surface flex items-center justify-between gap-3 shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <div className="p-1 rounded bg-accent-primary/10 text-accent-primary shrink-0">
            <Database className="w-4 h-4" />
          </div>
          <span className="text-ui-sm font-semibold text-fg-primary truncate">{fileName}</span>

          <span className="px-1.5 py-0.5 rounded text-ui-xs font-medium uppercase tracking-wider bg-surface-hover text-fg-secondary border border-border shrink-0">
            {dbInfo.type}
          </span>

          <span className="hidden sm:inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-ui-xs bg-status-success/10 text-status-success shrink-0">
            <ShieldCheck className="w-3 h-3" />
            Read-Only
          </span>

          <span className="hidden md:inline-flex items-center gap-1 text-ui-xs text-fg-tertiary shrink-0">
            <Cpu className="w-3 h-3" />
            {dbInfo.driver}
          </span>
        </div>

        {/* View mode toggle */}
        <div className="flex items-center gap-1 bg-panel p-0.5 rounded-lg border border-border shrink-0">
          <button
            type="button"
            onClick={() => setViewMode('data')}
            className={`px-2.5 py-1 rounded text-ui-xs font-medium transition-colors flex items-center gap-1.5 ${
              viewMode === 'data'
                ? 'bg-surface text-fg-primary shadow-xs font-semibold'
                : 'text-fg-tertiary hover:text-fg-secondary'
            }`}
          >
            <TableIcon className="w-3.5 h-3.5" />
            Data
          </button>
          <button
            type="button"
            onClick={() => setViewMode('schema')}
            className={`px-2.5 py-1 rounded text-ui-xs font-medium transition-colors flex items-center gap-1.5 ${
              viewMode === 'schema'
                ? 'bg-surface text-fg-primary shadow-xs font-semibold'
                : 'text-fg-tertiary hover:text-fg-secondary'
            }`}
          >
            <FileCode className="w-3.5 h-3.5" />
            Schema
          </button>
          <button
            type="button"
            onClick={() => setViewMode('query')}
            className={`px-2.5 py-1 rounded text-ui-xs font-medium transition-colors flex items-center gap-1.5 ${
              viewMode === 'query'
                ? 'bg-surface text-fg-primary shadow-xs font-semibold'
                : 'text-fg-tertiary hover:text-fg-secondary'
            }`}
          >
            <Terminal className="w-3.5 h-3.5" />
            SQL Query
          </button>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-1.5 shrink-0">
          {viewMode === 'data' && rows.length > 0 && (
            <div className="relative group">
              <button
                type="button"
                className="px-2 py-1 rounded-md border border-border bg-surface hover:bg-surface-hover text-fg-secondary text-ui-xs flex items-center gap-1 transition-colors"
                title="Export Table Data"
              >
                <Download className="w-3.5 h-3.5" />
                <span className="hidden sm:inline">Export</span>
              </button>
              <div className="absolute right-0 top-full mt-1 hidden group-hover:flex flex-col w-32 bg-surface border border-border rounded-lg shadow-lg py-1 z-20">
                <button
                  type="button"
                  onClick={() => exportData('csv', 'table')}
                  className="px-3 py-1.5 text-left text-ui-xs text-fg-secondary hover:text-fg-primary hover:bg-surface-hover flex items-center gap-2"
                >
                  <FileSpreadsheet className="w-3.5 h-3.5 text-accent-primary" />
                  CSV
                </button>
                <button
                  type="button"
                  onClick={() => exportData('json', 'table')}
                  className="px-3 py-1.5 text-left text-ui-xs text-fg-secondary hover:text-fg-primary hover:bg-surface-hover flex items-center gap-2"
                >
                  <FileCode className="w-3.5 h-3.5 text-accent-primary" />
                  JSON
                </button>
              </div>
            </div>
          )}

          <button
            type="button"
            onClick={() => {
              if (viewMode === 'data') void loadTableData();
              else void loadDatabase();
            }}
            className="p-1.5 rounded-md border border-border bg-surface hover:bg-surface-hover text-fg-secondary transition-colors"
            title="Refresh"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${dataLoading ? 'animate-spin text-accent-primary' : ''}`} />
          </button>
        </div>
      </header>

      {/* ── Main Workspace ────────────────────────────────────────────────── */}
      <div className="flex-1 flex overflow-hidden">
        {/* ── Left Sidebar (Tables & Views) ───────────────────────────────── */}
        <aside className="w-56 min-w-[200px] border-r border-border bg-surface flex flex-col shrink-0">
          {/* Table Search */}
          <div className="p-2 border-b border-border">
            <div className="relative flex items-center">
              <Search className="w-3.5 h-3.5 text-fg-tertiary absolute left-2 pointer-events-none" />
              <input
                type="text"
                placeholder="Filter tables…"
                value={tableSearch}
                onChange={(e) => setTableSearch(e.target.value)}
                className="w-full pl-7 pr-2 py-1 rounded bg-panel border border-border text-ui-xs text-fg-primary placeholder:text-fg-tertiary focus:outline-none focus:border-accent-primary"
              />
              {tableSearch && (
                <button
                  type="button"
                  onClick={() => setTableSearch('')}
                  className="absolute right-2 text-fg-tertiary hover:text-fg-secondary"
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </div>
          </div>

          {/* Tables & Views Tree */}
          <div className="flex-1 overflow-y-auto p-2 space-y-4">
            {/* Tables Section */}
            <div>
              <div className="flex items-center justify-between px-2 mb-1">
                <span className="text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">
                  Tables ({filteredTables.length})
                </span>
              </div>
              <div className="space-y-0.5">
                {filteredTables.map((tbl) => {
                  const isActive = activeTable === tbl.name;
                  return (
                    <button
                      key={tbl.name}
                      type="button"
                      onClick={() => handleSelectTable(tbl.name)}
                      className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-md text-left text-ui-xs transition-colors ${
                        isActive
                          ? 'bg-selected text-fg-primary font-medium shadow-xs'
                          : 'text-fg-secondary hover:text-fg-primary hover:bg-surface-hover'
                      }`}
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        <TableIcon className={`w-3.5 h-3.5 shrink-0 ${isActive ? 'text-accent-primary' : 'text-fg-tertiary'}`} />
                        <span className="truncate">{tbl.name}</span>
                      </div>
                      <span className="text-ui-xs text-fg-tertiary tabular-nums ml-2 shrink-0">
                        {tbl.rowCount.toLocaleString()}
                      </span>
                    </button>
                  );
                })}
                {filteredTables.length === 0 && (
                  <div className="px-2 py-2 text-ui-xs text-fg-tertiary italic">No tables found</div>
                )}
              </div>
            </div>

            {/* Views Section */}
            {filteredViews.length > 0 && (
              <div>
                <div className="flex items-center justify-between px-2 mb-1">
                  <span className="text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">
                    Views ({filteredViews.length})
                  </span>
                </div>
                <div className="space-y-0.5">
                  {filteredViews.map((vw) => {
                    const isActive = activeTable === vw.name;
                    return (
                      <button
                        key={vw.name}
                        type="button"
                        onClick={() => handleSelectTable(vw.name)}
                        className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-md text-left text-ui-xs transition-colors ${
                          isActive
                            ? 'bg-selected text-fg-primary font-medium shadow-xs'
                            : 'text-fg-secondary hover:text-fg-primary hover:bg-surface-hover'
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <Layers className={`w-3.5 h-3.5 shrink-0 ${isActive ? 'text-accent-primary' : 'text-fg-tertiary'}`} />
                          <span className="truncate">{vw.name}</span>
                        </div>
                        <span className="text-ui-xs text-fg-tertiary tabular-nums ml-2 shrink-0">
                          {vw.rowCount.toLocaleString()}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {/* Footer Metadata */}
          <div className="p-2 border-t border-border bg-panel/50 text-ui-xs text-fg-tertiary flex items-center justify-between">
            <span>Size: {formatFileSize(dbInfo.size)}</span>
            <span>{filteredTables.length + filteredViews.length} objects</span>
          </div>
        </aside>

        {/* ── Content View Area ──────────────────────────────────────────── */}
        <main className="flex-1 flex flex-col min-w-0 overflow-hidden bg-panel">
          {/* VIEW: Table Data Grid */}
          {viewMode === 'data' && (
            <div className="flex-1 flex flex-col overflow-hidden">
              {/* Filter bar */}
              <div className="h-10 px-3 border-b border-border bg-surface/50 flex items-center justify-between gap-3 shrink-0">
                <div className="flex items-center gap-2 flex-1 max-w-md">
                  <span className="text-ui-xs font-medium text-fg-tertiary shrink-0">Filter:</span>
                  <select
                    value={filterCol}
                    onChange={(e) => {
                      setFilterCol(e.target.value);
                      setPage(1);
                    }}
                    className="h-7 px-2 rounded bg-panel border border-border text-ui-xs text-fg-secondary focus:outline-none focus:border-accent-primary shrink-0"
                  >
                    <option value="">Any column</option>
                    {columns.map((c) => (
                      <option key={c} value={c}>
                        {c}
                      </option>
                    ))}
                  </select>
                  <div className="relative flex-1">
                    <input
                      type="text"
                      placeholder="Search values in table…"
                      value={filterVal}
                      onChange={(e) => {
                        setFilterVal(e.target.value);
                        setPage(1);
                      }}
                      className="w-full h-7 px-2.5 rounded bg-panel border border-border text-ui-xs text-fg-primary placeholder:text-fg-tertiary focus:outline-none focus:border-accent-primary"
                    />
                    {filterVal && (
                      <button
                        type="button"
                        onClick={() => {
                          setFilterVal('');
                          setPage(1);
                        }}
                        className="absolute right-2 top-1.5 text-fg-tertiary hover:text-fg-secondary"
                      >
                        <X className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-3 text-ui-xs text-fg-secondary">
                  <span>
                    Total: <strong className="text-fg-primary tabular-nums">{totalRows.toLocaleString()}</strong> rows
                  </span>
                </div>
              </div>

              {/* Data Table Scroll Surface */}
              <div className="flex-1 overflow-auto relative">
                {dataLoading && (
                  <div className="absolute inset-0 bg-panel/60 backdrop-blur-xs flex items-center justify-center z-10">
                    <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-surface border border-border shadow-sm text-ui-xs text-fg-primary">
                      <RefreshCw className="w-3.5 h-3.5 animate-spin text-accent-primary" />
                      Loading rows…
                    </div>
                  </div>
                )}

                {dataError ? (
                  <div className="p-8 text-center text-status-danger text-ui-sm flex flex-col items-center gap-2">
                    <AlertCircle className="w-6 h-6" />
                    <span>{dataError}</span>
                  </div>
                ) : rows.length === 0 ? (
                  <div className="flex flex-col items-center justify-center h-full text-fg-tertiary text-ui-sm gap-2 select-none">
                    <TableIcon className="w-8 h-8 opacity-40" />
                    <span>No rows in "{activeTable}"</span>
                  </div>
                ) : (
                  <table className="w-full border-collapse text-left text-ui-xs">
                    <thead className="sticky top-0 bg-surface border-b border-border z-10">
                      <tr>
                        <th className="w-12 px-3 py-2 font-medium text-fg-tertiary text-center border-r border-border/50">
                          #
                        </th>
                        {columns.map((col) => {
                          const isSorted = sortCol === col;
                          const colSchema = currentTableInfo?.columns?.find((c) => c.name === col);
                          return (
                            <th
                              key={col}
                              onClick={() => handleSort(col)}
                              className="px-3 py-2 font-semibold text-fg-primary border-r border-border/50 hover:bg-surface-hover cursor-pointer transition-colors"
                            >
                              <div className="flex items-center justify-between gap-2">
                                <div className="flex items-center gap-1.5 min-w-0">
                                  {colSchema?.isPk && (
                                    <span title="Primary Key" className="inline-flex shrink-0">
                                      <Key className="w-3 h-3 text-amber-500" />
                                    </span>
                                  )}
                                  <span className="truncate">{col}</span>
                                  {colSchema?.type && (
                                    <span className="text-[10px] text-fg-tertiary font-mono uppercase">
                                      {colSchema.type}
                                    </span>
                                  )}
                                </div>
                                <div className="text-fg-tertiary shrink-0">
                                  {isSorted ? (
                                    sortAsc ? (
                                      <ArrowUp className="w-3.5 h-3.5 text-accent-primary" />
                                    ) : (
                                      <ArrowDown className="w-3.5 h-3.5 text-accent-primary" />
                                    )
                                  ) : (
                                    <ArrowUpDown className="w-3 h-3 opacity-30 hover:opacity-100" />
                                  )}
                                </div>
                              </div>
                            </th>
                          );
                        })}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border/40 font-mono">
                      {rows.map((row, idx) => {
                        const rowNum = (page - 1) * pageSize + idx + 1;
                        return (
                          <tr key={idx} className="hover:bg-surface-hover/80 transition-colors">
                            <td className="px-3 py-1.5 text-center text-fg-tertiary border-r border-border/40 select-none">
                              {rowNum}
                            </td>
                            {columns.map((col) => {
                              const val = row[col];
                              const isNull = val === null || val === undefined;
                              const isBool = typeof val === 'boolean';
                              const isObj = typeof val === 'object' && !isNull;
                              const displayVal = isNull
                                ? 'null'
                                : isObj
                                ? JSON.stringify(val)
                                : String(val);

                              return (
                                <td
                                  key={col}
                                  onClick={() => setInspectCell({ col, val })}
                                  className="px-3 py-1.5 border-r border-border/40 truncate max-w-xs cursor-pointer hover:underline"
                                  title="Click to view cell content"
                                >
                                  {isNull ? (
                                    <span className="text-fg-tertiary italic text-ui-xs">NULL</span>
                                  ) : isBool ? (
                                    <span
                                      className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${
                                        val ? 'bg-status-success/15 text-status-success' : 'bg-status-danger/15 text-status-danger'
                                      }`}
                                    >
                                      {val ? 'TRUE' : 'FALSE'}
                                    </span>
                                  ) : (
                                    <span className="text-fg-primary text-ui-xs">{displayVal}</span>
                                  )}
                                </td>
                              );
                            })}
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>

              {/* Pagination footer */}
              <footer className="h-10 px-3 border-t border-border bg-surface flex items-center justify-between text-ui-xs text-fg-secondary shrink-0">
                <div className="flex items-center gap-2">
                  <span>Rows per page:</span>
                  <select
                    value={pageSize}
                    onChange={(e) => {
                      setPageSize(Number(e.target.value));
                      setPage(1);
                    }}
                    className="h-6 px-1.5 rounded bg-panel border border-border text-ui-xs text-fg-primary focus:outline-none"
                  >
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={100}>100</option>
                    <option value={250}>250</option>
                  </select>
                </div>

                <div className="flex items-center gap-3">
                  <span className="tabular-nums">
                    Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, totalRows)} of {totalRows.toLocaleString()}
                  </span>

                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      disabled={page <= 1}
                      onClick={() => setPage(1)}
                      className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none transition-colors"
                      title="First Page"
                    >
                      <ChevronsLeft className="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => Math.max(1, p - 1))}
                      className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none transition-colors"
                      title="Previous Page"
                    >
                      <ChevronLeft className="w-3.5 h-3.5" />
                    </button>
                    <span className="px-2 py-0.5 rounded bg-panel border border-border tabular-nums font-medium text-fg-primary">
                      {page} / {totalPages}
                    </span>
                    <button
                      type="button"
                      disabled={page >= totalPages}
                      onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                      className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none transition-colors"
                      title="Next Page"
                    >
                      <ChevronRight className="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      disabled={page >= totalPages}
                      onClick={() => setPage(totalPages)}
                      className="p-1 rounded hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none transition-colors"
                      title="Last Page"
                    >
                      <ChevronsRight className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </footer>
            </div>
          )}

          {/* VIEW: Table Schema */}
          {viewMode === 'schema' && (
            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              <div className="max-w-4xl mx-auto flex flex-col gap-6">
                {/* Schema Summary Card */}
                <div className="bg-surface border border-border rounded-xl p-5 shadow-sm flex flex-col gap-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="p-2 rounded-lg bg-accent-primary/10 text-accent-primary">
                        <TableIcon className="w-5 h-5" />
                      </div>
                      <div>
                        <h2 className="text-ui-lg font-bold text-fg-primary">{activeTable}</h2>
                        <div className="text-ui-xs text-fg-tertiary">
                          Type: {currentTableInfo?.type || 'table'} • {currentTableInfo?.rowCount.toLocaleString()} rows •{' '}
                          {currentTableInfo?.columns?.length || 0} columns
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Columns Table */}
                  <div className="border border-border rounded-lg overflow-hidden">
                    <table className="w-full text-left text-ui-xs">
                      <thead className="bg-panel border-b border-border font-semibold text-fg-primary">
                        <tr>
                          <th className="px-3 py-2">#</th>
                          <th className="px-3 py-2">Column</th>
                          <th className="px-3 py-2">Type</th>
                          <th className="px-3 py-2">Constraints</th>
                          <th className="px-3 py-2">Default</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border font-mono">
                        {currentTableInfo?.columns?.map((col) => (
                          <tr key={col.cid} className="hover:bg-surface-hover transition-colors">
                            <td className="px-3 py-2 text-fg-tertiary">{col.cid}</td>
                            <td className="px-3 py-2 font-bold text-fg-primary flex items-center gap-1.5">
                              {col.isPk && (
                                <span title="Primary Key" className="inline-flex shrink-0">
                                  <Key className="w-3 h-3 text-amber-500" />
                                </span>
                              )}
                              {col.name}
                            </td>
                            <td className="px-3 py-2 text-accent-primary uppercase">{col.type || 'ANY'}</td>
                            <td className="px-3 py-2 text-fg-secondary">
                              <div className="flex items-center gap-1">
                                {col.isPk && (
                                  <span className="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-500 font-semibold">
                                    PK
                                  </span>
                                )}
                                {col.notNull && (
                                  <span className="px-1.5 py-0.5 rounded text-[10px] bg-status-danger/10 text-status-danger font-semibold">
                                    NOT NULL
                                  </span>
                                )}
                                {!col.isPk && !col.notNull && <span className="text-fg-tertiary">—</span>}
                              </div>
                            </td>
                            <td className="px-3 py-2 text-fg-tertiary">{col.defaultValue || '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>

                {/* SQL Definition DDL */}
                {currentTableInfo?.sql && (
                  <div className="bg-surface border border-border rounded-xl p-5 shadow-sm flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                      <h3 className="text-ui-base font-semibold text-fg-primary flex items-center gap-2">
                        <FileCode className="w-4 h-4 text-accent-primary" />
                        SQL Definition (DDL)
                      </h3>
                      <button
                        type="button"
                        onClick={() => {
                          if (currentTableInfo?.sql) {
                            void navigator.clipboard.writeText(currentTableInfo.sql);
                            setCopiedDDL(true);
                            setTimeout(() => setCopiedDDL(false), 2000);
                          }
                        }}
                        className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-border bg-panel hover:bg-surface-hover text-ui-xs text-fg-secondary transition-colors"
                      >
                        {copiedDDL ? <Check className="w-3.5 h-3.5 text-status-success" /> : <Copy className="w-3.5 h-3.5" />}
                        {copiedDDL ? 'Copied' : 'Copy DDL'}
                      </button>
                    </div>

                    <pre className="p-3.5 rounded-lg bg-panel border border-border text-ui-xs font-mono text-fg-primary overflow-x-auto whitespace-pre-wrap select-all">
                      {currentTableInfo.sql}
                    </pre>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* VIEW: SQL Query Console */}
          {viewMode === 'query' && (
            <div className="flex-1 flex flex-col overflow-hidden">
              {/* Query Editor Pane */}
              <div className="p-3 border-b border-border bg-surface flex flex-col gap-2 shrink-0">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Terminal className="w-4 h-4 text-accent-primary" />
                    <span className="text-ui-xs font-semibold text-fg-primary uppercase tracking-wider">SQL Query Console</span>
                    <span className="text-ui-xs text-fg-tertiary">• Read-Only (SELECT / PRAGMA)</span>
                  </div>

                  <div className="flex items-center gap-2">
                    <span className="text-[10px] text-fg-tertiary hidden sm:inline">Press ⌘+Enter to execute</span>
                    <button
                      type="button"
                      disabled={queryLoading || !querySql.trim()}
                      onClick={() => void handleRunQuery()}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-foreground text-background text-ui-xs font-semibold hover:opacity-90 disabled:opacity-40 transition-opacity"
                    >
                      <Play className="w-3.5 h-3.5 fill-current" />
                      {queryLoading ? 'Running…' : 'Run Query'}
                    </button>
                  </div>
                </div>

                <div className="relative">
                  <textarea
                    rows={4}
                    value={querySql}
                    onChange={(e) => setQuerySql(e.target.value)}
                    onKeyDown={(e) => {
                      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                        e.preventDefault();
                        void handleRunQuery();
                      }
                    }}
                    placeholder="Enter SQL statement (e.g. SELECT * FROM users WHERE active = 1;)"
                    className="w-full p-2.5 rounded-lg bg-panel border border-border font-mono text-ui-xs text-fg-primary placeholder:text-fg-tertiary focus:outline-none focus:border-accent-primary resize-y"
                  />
                </div>
              </div>

              {/* Query Result Header & Toolbar */}
              {queryResult && (
                <div className="h-9 px-3 border-b border-border bg-surface/50 flex items-center justify-between text-ui-xs shrink-0">
                  <div className="flex items-center gap-2">
                    {queryResult.error ? (
                      <span className="text-status-danger font-medium flex items-center gap-1.5">
                        <AlertCircle className="w-3.5 h-3.5" />
                        Query Error
                      </span>
                    ) : (
                      <>
                        <span className="text-status-success font-medium flex items-center gap-1.5">
                          <Check className="w-3.5 h-3.5" />
                          Success
                        </span>
                        <span className="text-fg-tertiary">•</span>
                        <span className="text-fg-secondary">
                          {queryResult.totalRows.toLocaleString()} rows ({queryResult.executionMs} ms)
                        </span>
                      </>
                    )}
                  </div>

                  {!queryResult.error && queryResult.rows && queryResult.rows.length > 0 && (
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => exportData('csv', 'query')}
                        className="px-2 py-0.5 rounded border border-border hover:bg-surface-hover text-fg-secondary text-ui-xs transition-colors flex items-center gap-1"
                      >
                        <FileSpreadsheet className="w-3 h-3 text-accent-primary" />
                        Export CSV
                      </button>
                      <button
                        type="button"
                        onClick={() => exportData('json', 'query')}
                        className="px-2 py-0.5 rounded border border-border hover:bg-surface-hover text-fg-secondary text-ui-xs transition-colors flex items-center gap-1"
                      >
                        <FileCode className="w-3 h-3 text-accent-primary" />
                        Export JSON
                      </button>
                    </div>
                  )}
                </div>
              )}

              {/* Query Results Surface */}
              <div className="flex-1 overflow-auto relative">
                {queryLoading ? (
                  <div className="flex flex-col items-center justify-center h-full text-fg-secondary gap-2">
                    <RefreshCw className="w-5 h-5 animate-spin text-accent-primary" />
                    <span className="text-ui-xs">Executing query…</span>
                  </div>
                ) : queryResult?.error ? (
                  <div className="p-6 text-status-danger font-mono text-ui-xs whitespace-pre-wrap">
                    {queryResult.error}
                  </div>
                ) : !queryResult ? (
                  <div className="flex flex-col items-center justify-center h-full text-fg-tertiary text-ui-sm gap-2 select-none">
                    <Terminal className="w-8 h-8 opacity-40" />
                    <span>Run a query to view results</span>
                  </div>
                ) : queryResult.rows && queryResult.rows.length > 0 ? (
                  <table className="w-full border-collapse text-left text-ui-xs font-mono">
                    <thead className="sticky top-0 bg-surface border-b border-border z-10 font-sans">
                      <tr>
                        <th className="w-12 px-3 py-2 font-medium text-fg-tertiary text-center border-r border-border/50">
                          #
                        </th>
                        {queryResult.columns?.map((c) => (
                          <th key={c} className="px-3 py-2 font-semibold text-fg-primary border-r border-border/50">
                            {c}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border/40">
                      {queryResult.rows.map((row, idx) => (
                        <tr key={idx} className="hover:bg-surface-hover/80 transition-colors">
                          <td className="px-3 py-1.5 text-center text-fg-tertiary border-r border-border/40 select-none">
                            {idx + 1}
                          </td>
                          {queryResult.columns?.map((col) => {
                            const val = row ? row[col] : undefined;
                            const isNull = val === null || val === undefined;
                            const isObj = typeof val === 'object' && !isNull;
                            const displayVal = isNull
                              ? 'null'
                              : isObj
                              ? JSON.stringify(val)
                              : String(val);

                            return (
                              <td
                                key={col}
                                onClick={() => setInspectCell({ col, val })}
                                className="px-3 py-1.5 border-r border-border/40 truncate max-w-xs cursor-pointer hover:underline"
                                title="Click to view cell content"
                              >
                                {isNull ? (
                                  <span className="text-fg-tertiary italic">NULL</span>
                                ) : (
                                  <span className="text-fg-primary">{displayVal}</span>
                                )}
                              </td>
                            );
                          })}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="flex flex-col items-center justify-center h-full text-fg-tertiary text-ui-sm gap-2 select-none">
                    <span>Query completed with 0 results</span>
                  </div>
                )}
              </div>
            </div>
          )}
        </main>
      </div>

      {/* ── Cell Inspector Modal ─────────────────────────────────────────── */}
      {inspectCell && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="w-full max-w-2xl bg-surface border border-border rounded-xl shadow-xl flex flex-col max-h-[85vh] overflow-hidden">
            <div className="px-4 py-3 border-b border-border flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-ui-xs font-semibold text-fg-tertiary uppercase tracking-wider">Cell Inspector</span>
                <span className="text-ui-sm font-bold text-fg-primary">[{inspectCell.col}]</span>
              </div>
              <button
                type="button"
                onClick={() => setInspectCell(null)}
                className="p-1 rounded-md text-fg-tertiary hover:text-fg-primary hover:bg-surface-hover transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex-1 p-4 overflow-y-auto">
              <pre className="p-3 rounded-lg bg-panel border border-border text-ui-xs font-mono text-fg-primary whitespace-pre-wrap break-all select-all">
                {inspectCell.val === null || inspectCell.val === undefined
                  ? 'NULL'
                  : typeof inspectCell.val === 'object'
                  ? JSON.stringify(inspectCell.val, null, 2)
                  : String(inspectCell.val)}
              </pre>
            </div>

            <div className="px-4 py-3 border-t border-border bg-panel/40 flex items-center justify-between">
              <span className="text-ui-xs text-fg-tertiary">
                Type: {inspectCell.val === null ? 'null' : typeof inspectCell.val}
              </span>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => {
                    const text =
                      typeof inspectCell.val === 'object'
                        ? JSON.stringify(inspectCell.val, null, 2)
                        : String(inspectCell.val ?? '');
                    void navigator.clipboard.writeText(text);
                    setCopiedCell(true);
                    setTimeout(() => setCopiedCell(false), 2000);
                  }}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-border bg-surface hover:bg-surface-hover text-ui-xs text-fg-secondary font-medium transition-colors"
                >
                  {copiedCell ? <Check className="w-3.5 h-3.5 text-status-success" /> : <Copy className="w-3.5 h-3.5" />}
                  {copiedCell ? 'Copied' : 'Copy Value'}
                </button>
                <button
                  type="button"
                  onClick={() => setInspectCell(null)}
                  className="px-3 py-1.5 rounded-md bg-foreground text-background text-ui-xs font-medium hover:opacity-90 transition-opacity"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
