import React, { useState, useRef, useEffect, useCallback } from 'react';
import {
  Upload,
  FolderUp,
  FilePlus,
  X,
  Server,
  Folder,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Trash2,
  RefreshCw,
  HardDrive,
  FileText
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

interface UploadItem {
  id: string;
  name: string;
  size: number;
  localPath?: string;
  fileObj?: File;
  destPath: string;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  errorMsg?: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export const RemoteFileUploaderModal: React.FC<Props> = ({ isOpen, onClose }) => {
  const {
    activeWorkspacePath,
    remoteUploadTargetDir,
    setRemoteUploadTargetDir,
    refreshFiles
  } = useWorkspace();

  const [queue, setQueue] = useState<UploadItem[]>([]);
  const [isUploading, setIsUploading] = useState<boolean>(false);
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const [targetFolder, setTargetFolder] = useState<string>('');

  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);

  // Initialize target directory from prop / active workspace
  useEffect(() => {
    if (isOpen) {
      const initialTarget = remoteUploadTargetDir || activeWorkspacePath || '';
      setTargetFolder(initialTarget);
      setQueue([]);
      setIsUploading(false);
      setIsDragging(false);
    }
  }, [isOpen, remoteUploadTargetDir, activeWorkspacePath]);

  // Display-friendly remote target path
  const parsedTarget = React.useMemo(() => {
    if (!targetFolder) return { host: 'Remote Server', path: '/' };
    if (targetFolder.startsWith('ssh://')) {
      const trimmed = targetFolder.replace('ssh://', '');
      const firstSlash = trimmed.indexOf('/');
      if (firstSlash === -1) {
        return { host: trimmed, path: '/' };
      }
      return {
        host: trimmed.substring(0, firstSlash),
        path: trimmed.substring(firstSlash)
      };
    }
    return { host: 'Remote Server', path: targetFolder };
  }, [targetFolder]);

  // Add native files by absolute paths
  const addLocalPaths = useCallback((paths: string[]) => {
    if (!paths || paths.length === 0) return;
    const baseTarget = targetFolder || activeWorkspacePath || '';
    const newItems: UploadItem[] = paths.map((p, idx) => {
      const name = p.split(/[/\\]/).pop() || `file-${idx}`;
      const dest = `${baseTarget.replace(/\/+$/, '')}/${name}`;
      return {
        id: `local-${Date.now()}-${idx}-${Math.random().toString(36).substring(2, 6)}`,
        name,
        size: 0,
        localPath: p,
        destPath: dest,
        status: 'pending'
      };
    });

    setQueue(prev => [...prev, ...newItems]);
  }, [targetFolder, activeWorkspacePath]);

  // Add web File objects
  const addWebFiles = useCallback((files: File[]) => {
    if (!files || files.length === 0) return;
    const baseTarget = targetFolder || activeWorkspacePath || '';
    const newItems: UploadItem[] = files.map((f, idx) => {
      const relativePath = (f as any).webkitRelativePath || f.name;
      const cleanRel = relativePath.replace(/^[/\\]+/, '');
      const dest = `${baseTarget.replace(/\/+$/, '')}/${cleanRel}`;
      return {
        id: `web-${Date.now()}-${idx}-${Math.random().toString(36).substring(2, 6)}`,
        name: f.name,
        size: f.size,
        fileObj: f,
        destPath: dest,
        status: 'pending'
      };
    });

    setQueue(prev => [...prev, ...newItems]);
  }, [targetFolder, activeWorkspacePath]);

  // Native File Picker trigger
  const handleBrowseFiles = async () => {
    try {
      const selected = await ApiBridge.openUploadFileDialog();
      if (selected && selected.length > 0) {
        addLocalPaths(selected);
        return;
      }
    } catch {
      // Fallback to HTML input
    }
    fileInputRef.current?.click();
  };

  // Native Folder Picker trigger
  const handleBrowseFolder = async () => {
    try {
      const selectedDir = await ApiBridge.openUploadFolderDialog();
      if (selectedDir && selectedDir.trim()) {
        addLocalPaths([selectedDir.trim()]);
        return;
      }
    } catch {
      // Fallback to HTML input
    }
    folderInputRef.current?.click();
  };

  // HTML File Input Change
  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length > 0) {
      addWebFiles(files);
    }
    e.target.value = '';
  };

  // HTML Folder Input Change
  const handleFolderInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length > 0) {
      addWebFiles(files);
    }
    e.target.value = '';
  };

  // Drag and drop handlers
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    const droppedFiles = Array.from(e.dataTransfer.files || []);
    if (droppedFiles.length === 0) return;

    // Check if files have native path
    const nativePaths: string[] = [];
    const webFiles: File[] = [];

    for (const f of droppedFiles) {
      const p = (f as any).path;
      if (p) {
        nativePaths.push(p);
      } else {
        webFiles.push(f);
      }
    }

    if (nativePaths.length > 0) {
      addLocalPaths(nativePaths);
    }
    if (webFiles.length > 0) {
      addWebFiles(webFiles);
    }
  };

  const removeItem = (id: string) => {
    setQueue(prev => prev.filter(item => item.id !== id));
  };

  const clearCompleted = () => {
    setQueue(prev => prev.filter(item => item.status !== 'completed'));
  };

  const clearAll = () => {
    if (isUploading) return;
    setQueue([]);
  };

  // Upload runner
  const handleStartUpload = async () => {
    if (isUploading || queue.length === 0) return;
    const pendingItems = queue.filter(item => item.status === 'pending' || item.status === 'error');
    if (pendingItems.length === 0) return;

    setIsUploading(true);
    let successCount = 0;
    let failCount = 0;

    for (const item of pendingItems) {
      setQueue(prev =>
        prev.map(it => (it.id === item.id ? { ...it, status: 'uploading' } : it))
      );

      try {
        if (item.localPath) {
          // Native upload
          await ApiBridge.uploadFileToRemote(item.localPath, item.destPath);
        } else if (item.fileObj) {
          // Web File upload via base64
          const base64Data = await new Promise<string>((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => {
              const res = reader.result as string;
              const b64 = res.includes(',') ? res.split(',')[1] : res;
              resolve(b64);
            };
            reader.onerror = reject;
            reader.readAsDataURL(item.fileObj!);
          });
          await ApiBridge.writeFileBase64(item.destPath, base64Data);
        } else {
          throw new Error('No source file provided');
        }

        successCount++;
        setQueue(prev =>
          prev.map(it => (it.id === item.id ? { ...it, status: 'completed' } : it))
        );
      } catch (err: any) {
        failCount++;
        const msg = err?.message || 'Upload failed';
        setQueue(prev =>
          prev.map(it =>
            it.id === item.id ? { ...it, status: 'error', errorMsg: msg } : it
          )
        );
      }
    }

    setIsUploading(false);
    refreshFiles();

    if (successCount > 0) {
      showToast(
        `Successfully uploaded ${successCount} file${successCount > 1 ? 's' : ''} to remote server`,
        'success'
      );
    }
    if (failCount > 0) {
      showToast(`Failed to upload ${failCount} file${failCount > 1 ? 's' : ''}`, 'error');
    }
  };

  if (!isOpen) return null;

  const totalFiles = queue.length;
  const completedCount = queue.filter(it => it.status === 'completed').length;
  const pendingCount = queue.filter(it => it.status === 'pending' || it.status === 'error').length;
  const totalBytes = queue.reduce((acc, it) => acc + (it.size || 0), 0);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150"
      onClick={() => {
        if (!isUploading) onClose();
      }}
    >
      <div
        className="w-full max-w-2xl bg-card border border-border rounded-xl shadow-2xl flex flex-col overflow-hidden max-h-[85vh] text-foreground font-sans"
        onClick={e => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="px-5 py-4 border-b border-border flex items-center justify-between bg-surface-subtle/50 dark:bg-background/40">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-primary/10 text-primary flex items-center justify-center shrink-0">
              <Upload className="w-4.5 h-4.5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-semibold text-foreground">
                  Upload to Remote Server
                </h2>
                <span className="text-ui-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded font-mono font-medium flex items-center gap-1">
                  <Server className="w-3 h-3" />
                  SFTP
                </span>
              </div>
              <p className="text-xs text-foreground-subtle">
                Upload files or directories directly to remote SSH workspace
              </p>
            </div>
          </div>
          <button
            type="button"
            disabled={isUploading}
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer disabled:opacity-50"
            title="Close (Esc)"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Remote Destination Directory Bar */}
        <div className="px-5 py-3 border-b border-border bg-surface-subtle/20 flex flex-col gap-1.5">
          <div className="flex items-center justify-between text-xs">
            <span className="text-foreground-subtle flex items-center gap-1 font-medium">
              <Folder className="w-3.5 h-3.5 text-primary" /> Remote Target Directory:
            </span>
            <span className="text-ui-xs font-mono text-foreground-subtle bg-surface-hover px-1.5 py-0.5 rounded">
              {parsedTarget.host}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="text"
              disabled={isUploading}
              value={targetFolder}
              onChange={e => {
                setTargetFolder(e.target.value);
                setRemoteUploadTargetDir(e.target.value);
              }}
              placeholder="e.g. ssh://<connId>/var/www/myproject or /var/www"
              className="flex-1 px-2.5 py-1.5 rounded-lg border border-border bg-background text-xs font-mono text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>

        {/* Dropzone & Browse Bar */}
        <div className="p-5 flex flex-col gap-4 overflow-y-auto">
          <div
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            className={`border-2 border-dashed rounded-xl p-6 flex flex-col items-center justify-center gap-3 transition-colors text-center ${
              isDragging
                ? 'border-primary bg-primary/10'
                : 'border-border/80 hover:border-primary/50 bg-surface-subtle/30'
            }`}
          >
            <div className="w-10 h-10 rounded-full bg-primary/10 text-primary flex items-center justify-center">
              <Upload className="w-5 h-5" />
            </div>
            <div>
              <p className="text-xs font-medium text-foreground">
                Drag &amp; drop files or folders here
              </p>
              <p className="text-ui-xs text-foreground-subtle mt-0.5">
                Supports local files, multi-file selections, and full folder hierarchies
              </p>
            </div>

            <div className="flex items-center gap-2 mt-1">
              <button
                type="button"
                disabled={isUploading}
                onClick={handleBrowseFiles}
                className="px-3 py-1.5 bg-primary text-primary-foreground hover:bg-primary-hover rounded-lg text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors shadow-xs"
              >
                <FilePlus className="w-3.5 h-3.5" />
                Browse Files...
              </button>
              <button
                type="button"
                disabled={isUploading}
                onClick={handleBrowseFolder}
                className="px-3 py-1.5 bg-surface-hover hover:bg-surface-active border border-border text-foreground rounded-lg text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors"
              >
                <FolderUp className="w-3.5 h-3.5" />
                Browse Folder...
              </button>
            </div>

            {/* Hidden fallback inputs */}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              onChange={handleFileInputChange}
              className="hidden"
            />
            <input
              ref={folderInputRef}
              type="file"
              multiple
              // @ts-ignore
              webkitdirectory=""
              onChange={handleFolderInputChange}
              className="hidden"
            />
          </div>

          {/* Queue List */}
          {queue.length > 0 && (
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between text-xs font-medium text-foreground-subtle px-1">
                <span>
                  Files to Upload ({completedCount}/{totalFiles}
                  {totalBytes > 0 ? ` · ${formatBytes(totalBytes)}` : ''})
                </span>
                <div className="flex items-center gap-2">
                  {completedCount > 0 && (
                    <button
                      type="button"
                      disabled={isUploading}
                      onClick={clearCompleted}
                      className="text-primary hover:underline cursor-pointer text-ui-xs"
                    >
                      Clear Completed
                    </button>
                  )}
                  {!isUploading && (
                    <button
                      type="button"
                      onClick={clearAll}
                      className="text-destructive hover:underline cursor-pointer text-ui-xs"
                    >
                      Clear All
                    </button>
                  )}
                </div>
              </div>

              <div className="border border-border rounded-xl divide-y divide-border overflow-hidden max-h-56 overflow-y-auto bg-surface-subtle/10">
                {queue.map(item => (
                  <div
                    key={item.id}
                    className="px-3 py-2 flex items-center justify-between gap-3 text-xs hover:bg-surface-hover/50 transition-colors"
                  >
                    <div className="flex items-center gap-2.5 min-w-0 flex-1">
                      <FileText className="w-4 h-4 text-foreground-subtle shrink-0" />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-foreground truncate">{item.name}</span>
                          {item.size > 0 && (
                            <span className="text-ui-xs text-foreground-subtle shrink-0">
                              {formatBytes(item.size)}
                            </span>
                          )}
                        </div>
                        <div className="text-ui-xs font-mono text-foreground-subtle truncate">
                          &rarr; {item.destPath}
                        </div>
                        {item.errorMsg && (
                          <div className="text-ui-xs text-destructive mt-0.5">{item.errorMsg}</div>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      {item.status === 'pending' && (
                        <span className="text-ui-xs text-foreground-subtle bg-surface-hover px-1.5 py-0.5 rounded">
                          Ready
                        </span>
                      )}
                      {item.status === 'uploading' && (
                        <span className="text-ui-xs text-primary flex items-center gap-1 font-medium">
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          Uploading...
                        </span>
                      )}
                      {item.status === 'completed' && (
                        <span className="text-ui-xs text-success flex items-center gap-1 font-medium">
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          Uploaded
                        </span>
                      )}
                      {item.status === 'error' && (
                        <span className="text-ui-xs text-destructive flex items-center gap-1 font-medium">
                          <AlertCircle className="w-3.5 h-3.5" />
                          Error
                        </span>
                      )}

                      {!isUploading && item.status !== 'completed' && (
                        <button
                          type="button"
                          onClick={() => removeItem(item.id)}
                          className="p-1 hover:bg-destructive/10 text-foreground-subtle hover:text-destructive rounded transition-colors cursor-pointer"
                          title="Remove from queue"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="px-5 py-3 border-t border-border flex items-center justify-between bg-surface-subtle/50 dark:bg-background/40">
          <div className="text-xs text-foreground-subtle">
            {pendingCount > 0 ? (
              <span>{pendingCount} file{pendingCount > 1 ? 's' : ''} ready to upload</span>
            ) : completedCount > 0 ? (
              <span className="text-success font-medium">All uploads completed</span>
            ) : (
              <span>No files selected</span>
            )}
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={isUploading}
              onClick={onClose}
              className="px-3.5 py-1.5 rounded-lg border border-border text-foreground hover:bg-surface-hover text-xs font-medium cursor-pointer transition-colors"
            >
              {completedCount > 0 && pendingCount === 0 ? 'Done' : 'Cancel'}
            </button>

            <button
              type="button"
              disabled={isUploading || pendingCount === 0}
              onClick={handleStartUpload}
              className="px-4 py-1.5 rounded-lg bg-primary text-primary-foreground hover:bg-primary-hover text-xs font-medium flex items-center gap-1.5 cursor-pointer transition-colors shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isUploading ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  Uploading...
                </>
              ) : (
                <>
                  <Upload className="w-3.5 h-3.5" />
                  Upload All {pendingCount > 0 ? `(${pendingCount})` : ''}
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
