import React, { useState, useMemo } from 'react';
import {
  Kanban,
  ListTodo,
  Plus,
  Search,
  Filter,
  Play,
  CheckCircle2,
  Clock,
  Eye,
  TestTube2,
  CircleDot,
  ArrowRight,
  ArrowLeft,
  Trash2,
  Edit2,
  Bot,
  Sparkles,
  GitBranch,
  X,
  ChevronDown,
  RefreshCw
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';
import { cn } from '../../lib/utils';
import { showToast } from '../../lib/toast';
import type { BoardTask, BoardTaskStatus, BoardTaskPriority } from '../../types';

const STATUS_COLUMNS: Array<{
  id: BoardTaskStatus;
  label: string;
  color: string;
  border: string;
  bg: string;
  icon: React.ComponentType<{ className?: string }>;
}> = [
  {
    id: 'todo',
    label: 'To Do',
    color: 'text-slate-400 dark:text-slate-300',
    border: 'border-slate-500/30',
    bg: 'bg-slate-500/10',
    icon: CircleDot
  },
  {
    id: 'in_progress',
    label: 'In Progress',
    color: 'text-blue-500 dark:text-blue-400',
    border: 'border-blue-500/30',
    bg: 'bg-blue-500/10',
    icon: Clock
  },
  {
    id: 'in_review',
    label: 'In Review',
    color: 'text-amber-500 dark:text-amber-400',
    border: 'border-amber-500/30',
    bg: 'bg-amber-500/10',
    icon: Eye
  },
  {
    id: 'in_testing',
    label: 'In Testing',
    color: 'text-purple-500 dark:text-purple-400',
    border: 'border-purple-500/30',
    bg: 'bg-purple-500/10',
    icon: TestTube2
  },
  {
    id: 'done',
    label: 'Done',
    color: 'text-emerald-500 dark:text-emerald-400',
    border: 'border-emerald-500/30',
    bg: 'bg-emerald-500/10',
    icon: CheckCircle2
  }
];

const PRIORITY_BADGES: Record<BoardTaskPriority, { label: string; color: string; dot: string }> = {
  urgent: { label: 'Urgent', color: 'text-rose-500 bg-rose-500/10 border-rose-500/30', dot: 'bg-rose-500' },
  high: { label: 'High', color: 'text-amber-500 bg-amber-500/10 border-amber-500/30', dot: 'bg-amber-500' },
  medium: { label: 'Medium', color: 'text-blue-500 bg-blue-500/10 border-blue-500/30', dot: 'bg-blue-500' },
  low: { label: 'Low', color: 'text-slate-400 bg-slate-500/10 border-slate-500/30', dot: 'bg-slate-400' }
};

export const WorkflowBoardView: React.FC = () => {
  const {
    boardTasks,
    refreshBoardTasks,
    addBoardTask,
    updateBoardTask,
    deleteBoardTask,
    moveBoardTaskStatus,
    setAgentViewMode,
    setIsGitClientOpen,
    createNewSession,
    sendSideConversationPrompt,
    activeWorkspacePath,
    gitBranch
  } = useWorkspace();

  const [viewType, setViewType] = useState<'kanban' | 'list'>('kanban');
  const [search, setSearch] = useState('');
  const [priorityFilter, setPriorityFilter] = useState<'all' | BoardTaskPriority>('all');
  const [statusFilter, setStatusFilter] = useState<'all' | BoardTaskStatus>('all');
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Modal State
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<BoardTask | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [status, setStatus] = useState<BoardTaskStatus>('todo');
  const [priority, setPriority] = useState<BoardTaskPriority>('medium');
  const [assignee, setAssignee] = useState('Pandu-Micro');
  const [labels, setLabels] = useState('agent, backend');
  const [startImmediately, setStartImmediately] = useState(false);

  // Drag and Drop
  const [draggedTaskId, setDraggedTaskId] = useState<string | null>(null);

  const filteredTasks = useMemo(() => {
    return boardTasks.filter(t => {
      const matchSearch = search.trim() === '' ||
        t.title.toLowerCase().includes(search.toLowerCase()) ||
        t.key.toLowerCase().includes(search.toLowerCase()) ||
        (t.description || '').toLowerCase().includes(search.toLowerCase());
      const matchPriority = priorityFilter === 'all' || t.priority === priorityFilter;
      const matchStatus = statusFilter === 'all' || t.status === statusFilter;
      return matchSearch && matchPriority && matchStatus;
    });
  }, [boardTasks, search, priorityFilter, statusFilter]);

  const openCreateModal = (defaultStatus: BoardTaskStatus = 'todo') => {
    setEditingTask(null);
    setTitle('');
    setDescription('');
    setStatus(defaultStatus);
    setPriority('medium');
    setAssignee('Pandu-Micro');
    setLabels('');
    setStartImmediately(false);
    setIsModalOpen(true);
  };

  const openEditModal = (t: BoardTask) => {
    setEditingTask(t);
    setTitle(t.title);
    setDescription(t.description || '');
    setStatus(t.status);
    setPriority(t.priority);
    setAssignee(t.assignee || 'Pandu-Micro');
    setLabels((t.labels || []).join(', '));
    setStartImmediately(false);
    setIsModalOpen(true);
  };

  const handleSaveTask = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      showToast('Task title is required', 'error');
      return;
    }

    const labelList = labels.split(',').map(s => s.trim()).filter(Boolean);

    if (editingTask) {
      updateBoardTask(editingTask.id, {
        title,
        description,
        status,
        priority,
        assignee,
        labels: labelList
      });
      showToast(`Updated task ${editingTask.key}`);
    } else {
      const created = addBoardTask({
        title,
        description,
        status: startImmediately ? 'in_progress' : status,
        priority,
        assignee,
        labels: labelList,
        workspacePath: activeWorkspacePath,
        branch: gitBranch || 'main'
      });
      showToast(`Created task ${created.key}`);

      if (startImmediately) {
        // Connect to agent harness immediately!
        try {
          const sessionId = createNewSession(title, 'Pandu-Micro');
          if (sessionId) {
            updateBoardTask(created.id, { sessionId, status: 'in_progress' });
            setAgentViewMode('session');
            showToast(`Launched task ${created.key} with Pandu-Micro harness!`);
          }
        } catch {
          setAgentViewMode('session');
        }
      }
    }

    setIsModalOpen(false);
  };

  // Run Task in Agent Harness
  const handleRunInAgent = async (task: BoardTask) => {
    moveBoardTaskStatus(task.id, 'in_progress');
    showToast(`Starting ${task.key} in agent harness...`);
    try {
      const prompt = `${task.title}\n\nTask Description:\n${task.description || ''}`;
      const sessionId = createNewSession(prompt, task.assignee || 'Pandu-Micro');
      if (sessionId) {
        updateBoardTask(task.id, { sessionId, status: 'in_progress' });
        setAgentViewMode('session');
      }
    } catch {
      setAgentViewMode('session');
    }
  };

  // Status transition step
  const handleStepStatus = (task: BoardTask, direction: 'prev' | 'next') => {
    const statuses: BoardTaskStatus[] = ['todo', 'in_progress', 'in_review', 'in_testing', 'done'];
    const idx = statuses.indexOf(task.status);
    if (direction === 'prev' && idx > 0) {
      moveBoardTaskStatus(task.id, statuses[idx - 1]);
    } else if (direction === 'next' && idx < statuses.length - 1) {
      moveBoardTaskStatus(task.id, statuses[idx + 1]);
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background select-none font-sans">
      {/* Top Header / Filter Controls */}
      <div className="border-b border-border bg-surface/40 backdrop-blur-md px-6 py-3.5 flex flex-wrap items-center justify-between gap-4 shrink-0">
        <div className="flex items-center gap-3">
          <div className="size-8 rounded-lg bg-primary/10 border border-primary/20 flex items-center justify-center text-primary font-bold">
            <Kanban className="size-4.5" />
          </div>
          <div>
            <h1 className="text-base font-semibold text-foreground flex items-center gap-2">
              Agent Workflow Board
              <span className="text-ui-xs font-normal px-2 py-0.5 rounded-full bg-surface-active text-foreground-subtle border border-border">
                Kanban
              </span>
            </h1>
            <p className="text-ui-xs text-foreground-subtle">
              Automated task pipeline orchestrated with Pandu-Micro & Agent Harness
            </p>
          </div>
        </div>

        {/* Action Toolbar */}
        <div className="flex items-center gap-2.5">
          {/* Search */}
          <div className="relative flex items-center">
            <Search className="size-3.5 text-foreground-subtle absolute left-2.5 pointer-events-none" />
            <input
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search tasks..."
              className="pl-8 pr-3 py-1.5 rounded-lg border border-border bg-surface text-ui-xs text-foreground placeholder-foreground-subtle outline-none focus:border-primary w-48 transition-all"
            />
          </div>

          {/* Priority filter */}
          <select
            value={priorityFilter}
            onChange={e => setPriorityFilter(e.target.value as any)}
            className="px-2.5 py-1.5 rounded-lg border border-border bg-surface text-ui-xs text-foreground outline-none cursor-pointer hover:bg-surface-hover"
          >
            <option value="all">All Priorities</option>
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>

          {/* View Mode Toggle: Kanban vs List */}
          <div className="flex items-center rounded-lg border border-border bg-surface p-0.5">
            <button
              type="button"
              onClick={() => setViewType('kanban')}
              className={cn(
                'flex items-center gap-1.5 px-2.5 py-1 rounded-md text-ui-xs font-medium transition-colors cursor-pointer',
                viewType === 'kanban'
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'text-foreground-subtle hover:text-foreground'
              )}
            >
              <Kanban className="size-3.5" />
              Board
            </button>
            <button
              type="button"
              onClick={() => setViewType('list')}
              className={cn(
                'flex items-center gap-1.5 px-2.5 py-1 rounded-md text-ui-xs font-medium transition-colors cursor-pointer',
                viewType === 'list'
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'text-foreground-subtle hover:text-foreground'
              )}
            >
              <ListTodo className="size-3.5" />
              List
            </button>
          </div>

          {/* Refresh Tasks */}
          <button
            type="button"
            onClick={async () => {
              setIsRefreshing(true);
              try {
                await refreshBoardTasks();
                showToast('Board tasks refreshed', 'info');
              } finally {
                setIsRefreshing(false);
              }
            }}
            disabled={isRefreshing}
            className="p-2 rounded-lg border border-border bg-surface text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer"
            title="Refresh board from workspace"
          >
            <RefreshCw className={cn('size-3.5', isRefreshing && 'animate-spin text-primary')} />
          </button>

          {/* New Task Button */}
          <button
            type="button"
            onClick={() => openCreateModal('todo')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-ui-xs font-medium hover:bg-primary/90 transition-colors cursor-pointer shadow-sm"
          >
            <Plus className="size-3.5" />
            New Task
          </button>
        </div>
      </div>

      {/* Main Board View: Kanban or List */}
      <div className="flex-1 overflow-x-auto overflow-y-hidden p-6 bg-app/50">
        {viewType === 'kanban' ? (
          /* Kanban Columns Grid */
          <div className="grid grid-cols-5 gap-4.5 h-full min-w-[1100px]">
            {STATUS_COLUMNS.map(col => {
              const colTasks = filteredTasks.filter(t => t.status === col.id);
              const Icon = col.icon;
              return (
                <div
                  key={col.id}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={() => {
                    if (draggedTaskId) {
                      moveBoardTaskStatus(draggedTaskId, col.id);
                      setDraggedTaskId(null);
                    }
                  }}
                  className="flex flex-col h-full rounded-xl border border-border/70 bg-surface/50 backdrop-blur-sm overflow-hidden"
                >
                  {/* Column Header */}
                  <div className="px-3.5 py-2.5 border-b border-border/60 flex items-center justify-between bg-surface/70 shrink-0">
                    <div className="flex items-center gap-2">
                      <Icon className={cn('size-4', col.color)} />
                      <span className="text-ui-xs font-semibold text-foreground tracking-wide">
                        {col.label}
                      </span>
                      <span className="text-[11px] font-mono px-1.5 py-0.2 rounded-full bg-surface-active text-foreground-subtle border border-border">
                        {colTasks.length}
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={() => openCreateModal(col.id)}
                      className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface-hover cursor-pointer transition-colors"
                      title={`Add task to ${col.label}`}
                    >
                      <Plus className="size-3.5" />
                    </button>
                  </div>

                  {/* Tasks Cards List */}
                  <div className="flex-1 overflow-y-auto p-2.5 space-y-2.5">
                    {colTasks.length === 0 ? (
                      <div className="h-32 border border-dashed border-border/60 rounded-lg flex flex-col items-center justify-center text-foreground-subtlest text-ui-xs gap-1">
                        <span>No tasks</span>
                        <button
                          type="button"
                          onClick={() => openCreateModal(col.id)}
                          className="text-primary hover:underline cursor-pointer"
                        >
                          + Create
                        </button>
                      </div>
                    ) : (
                      colTasks.map(task => {
                        const pri = PRIORITY_BADGES[task.priority] || PRIORITY_BADGES.medium;
                        return (
                          <div
                            key={task.id}
                            draggable
                            onDragStart={() => setDraggedTaskId(task.id)}
                            className="group relative rounded-lg border border-border bg-popover p-3 shadow-xs hover:shadow-md hover:border-primary/40 transition-all cursor-grab active:cursor-grabbing select-none"
                          >
                            {/* Card Top: Key & Priority */}
                            <div className="flex items-center justify-between gap-2 mb-1.5">
                              <span className="text-[11px] font-mono font-semibold text-primary/90">
                                {task.key}
                              </span>
                              <span className={cn('text-[10px] px-1.5 py-0.5 rounded-full border font-medium flex items-center gap-1', pri.color)}>
                                <span className={cn('size-1.5 rounded-full', pri.dot)} />
                                {pri.label}
                              </span>
                            </div>

                            {/* Card Title */}
                            <h3
                              onClick={() => openEditModal(task)}
                              className="text-ui-xs font-semibold text-foreground line-clamp-2 hover:text-primary cursor-pointer mb-1 leading-snug"
                            >
                              {task.title}
                            </h3>

                            {/* Description preview */}
                            {task.description && (
                              <p className="text-[11px] text-foreground-subtle line-clamp-2 mb-2 leading-relaxed">
                                {task.description}
                              </p>
                            )}

                            {/* Assignee & Labels */}
                            <div className="flex flex-wrap items-center gap-1.5 mb-2.5">
                              <span className="text-[10px] px-1.5 py-0.5 rounded-md bg-surface-active text-foreground-subtle border border-border flex items-center gap-1">
                                <Bot className="size-3 text-primary" />
                                {task.assignee || 'Pandu-Micro'}
                              </span>
                              {(task.labels || []).slice(0, 2).map(l => (
                                <span key={l} className="text-[10px] px-1.5 py-0.5 rounded-md bg-surface text-foreground-subtlest border border-border">
                                  #{l}
                                </span>
                              ))}
                            </div>

                            {/* Action Row */}
                            <div className="pt-2 border-t border-border/50 flex items-center justify-between text-ui-xs">
                              <div className="flex items-center gap-1">
                                {task.status === 'todo' && (
                                  <button
                                    type="button"
                                    onClick={() => handleRunInAgent(task)}
                                    className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground font-medium text-[11px] transition-colors cursor-pointer"
                                    title="Run task in Agent Harness"
                                  >
                                    <Play className="size-3 fill-current" />
                                    Run
                                  </button>
                                )}
                                {task.status === 'in_review' && (
                                  <button
                                    type="button"
                                    onClick={() => setIsGitClientOpen(true)}
                                    className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-amber-500/10 text-amber-500 hover:bg-amber-500 hover:text-white font-medium text-[11px] transition-colors cursor-pointer"
                                    title="Open Git Client to review changes"
                                  >
                                    <Eye className="size-3" />
                                    Review
                                  </button>
                                )}
                                {task.status === 'in_progress' && (
                                  <button
                                    type="button"
                                    onClick={() => setAgentViewMode('session')}
                                    className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-blue-500/10 text-blue-500 hover:bg-blue-500 hover:text-white font-medium text-[11px] transition-colors cursor-pointer"
                                    title="View active session"
                                  >
                                    <Clock className="size-3" />
                                    Active
                                  </button>
                                )}
                              </div>

                              {/* Navigation Controls (< and >) */}
                              <div className="flex items-center gap-0.5 opacity-80 group-hover:opacity-100 transition-opacity">
                                <button
                                  type="button"
                                  onClick={() => handleStepStatus(task, 'prev')}
                                  disabled={col.id === 'todo'}
                                  className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
                                  title="Move to previous status"
                                >
                                  <ArrowLeft className="size-3" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => handleStepStatus(task, 'next')}
                                  disabled={col.id === 'done'}
                                  className="p-1 rounded-md text-foreground-subtle hover:text-foreground hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
                                  title="Move to next status"
                                >
                                  <ArrowRight className="size-3" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => deleteBoardTask(task.id)}
                                  className="p-1 rounded-md text-foreground-subtle hover:text-destructive hover:bg-destructive/10 cursor-pointer ml-1"
                                  title="Delete task"
                                >
                                  <Trash2 className="size-3" />
                                </button>
                              </div>
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          /* List View */
          <div className="rounded-xl border border-border bg-surface overflow-hidden shadow-xs">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-border bg-surface-hover/50 text-[11px] font-semibold text-foreground-subtle uppercase tracking-wider">
                  <th className="py-2.5 px-4 w-24">Key</th>
                  <th className="py-2.5 px-4">Title</th>
                  <th className="py-2.5 px-4 w-36">Status</th>
                  <th className="py-2.5 px-4 w-28">Priority</th>
                  <th className="py-2.5 px-4 w-36">Assignee</th>
                  <th className="py-2.5 px-4 w-32 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border text-ui-xs text-foreground">
                {filteredTasks.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="text-center py-16 text-foreground-subtle">
                      {boardTasks.length === 0 ? (
                        <div className="flex flex-col items-center justify-center gap-2">
                          <ListTodo className="size-8 text-foreground-subtle/50 mb-1" />
                          <span className="text-ui-sm font-medium text-foreground">No tasks yet</span>
                          <span className="text-ui-xs text-foreground-subtle max-w-sm">Create your first task to track work in the board or run it in the agent harness.</span>
                          <button
                            type="button"
                            onClick={() => openCreateModal('todo')}
                            className="mt-2 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-ui-xs font-medium hover:bg-primary/90 transition-colors cursor-pointer"
                          >
                            <Plus className="size-3.5" />
                            Create Task
                          </button>
                        </div>
                      ) : (
                        'No matching tasks found.'
                      )}
                    </td>
                  </tr>
                ) : (
                  filteredTasks.map(task => {
                    const pri = PRIORITY_BADGES[task.priority] || PRIORITY_BADGES.medium;
                    const col = STATUS_COLUMNS.find(c => c.id === task.status) || STATUS_COLUMNS[0];
                    return (
                      <tr key={task.id} className="hover:bg-surface-hover/40 transition-colors">
                        <td className="py-2.5 px-4 font-mono font-medium text-primary">
                          {task.key}
                        </td>
                        <td className="py-2.5 px-4">
                          <button
                            type="button"
                            onClick={() => openEditModal(task)}
                            className="font-medium text-left hover:text-primary cursor-pointer"
                          >
                            {task.title}
                          </button>
                        </td>
                        <td className="py-2.5 px-4">
                          <select
                            value={task.status}
                            onChange={(e) => moveBoardTaskStatus(task.id, e.target.value as BoardTaskStatus)}
                            className={cn('text-[11px] font-medium px-2 py-0.5 rounded-md border outline-none cursor-pointer', col.bg, col.color, col.border)}
                          >
                            {STATUS_COLUMNS.map(s => (
                              <option key={s.id} value={s.id}>{s.label}</option>
                            ))}
                          </select>
                        </td>
                        <td className="py-2.5 px-4">
                          <span className={cn('text-[10px] px-2 py-0.5 rounded-full border font-medium inline-flex items-center gap-1', pri.color)}>
                            <span className={cn('size-1.5 rounded-full', pri.dot)} />
                            {pri.label}
                          </span>
                        </td>
                        <td className="py-2.5 px-4 text-foreground-subtle flex items-center gap-1.5 pt-3">
                          <Bot className="size-3 text-primary" />
                          {task.assignee || 'Pandu-Micro'}
                        </td>
                        <td className="py-2.5 px-4 text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            {task.status === 'todo' && (
                              <button
                                type="button"
                                onClick={() => handleRunInAgent(task)}
                                className="px-2 py-1 rounded bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground font-medium text-[11px] cursor-pointer"
                              >
                                Run
                              </button>
                            )}
                            <button
                              type="button"
                              onClick={() => openEditModal(task)}
                              className="p-1 rounded text-foreground-subtle hover:text-foreground cursor-pointer"
                            >
                              <Edit2 className="size-3.5" />
                            </button>
                            <button
                              type="button"
                              onClick={() => deleteBoardTask(task.id)}
                              className="p-1 rounded text-foreground-subtle hover:text-destructive cursor-pointer"
                            >
                              <Trash2 className="size-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Create / Edit Task Modal */}
      {isModalOpen && (
        <div
          className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 animate-in fade-in duration-100"
          onClick={() => setIsModalOpen(false)}
        >
          <div
            role="dialog"
            aria-modal="true"
            className="w-full max-w-lg rounded-2xl border border-border bg-popover p-6 shadow-2xl space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-border pb-3">
              <h2 className="text-base font-semibold text-foreground flex items-center gap-2">
                <Kanban className="size-4.5 text-primary" />
                {editingTask ? `Edit Task ${editingTask.key}` : 'Create New Task'}
              </h2>
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="p-1 text-foreground-subtle hover:text-foreground rounded-md hover:bg-surface cursor-pointer"
              >
                <X className="size-4" />
              </button>
            </div>

            <form onSubmit={handleSaveTask} className="space-y-4">
              <div>
                <label className="block text-ui-xs font-semibold text-foreground mb-1">
                  Task Title *
                </label>
                <input
                  type="text"
                  required
                  value={title}
                  onChange={e => setTitle(e.target.value)}
                  placeholder="e.g. Implement rate limiter middleware for API routes"
                  className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground placeholder-foreground-subtle outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-ui-xs font-semibold text-foreground mb-1">
                  Description / Acceptance Criteria
                </label>
                <textarea
                  rows={3}
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  placeholder="Define task details, acceptance tests, and target files..."
                  className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground placeholder-foreground-subtle outline-none focus:border-primary resize-none font-sans"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-ui-xs font-semibold text-foreground mb-1">
                    Status
                  </label>
                  <select
                    value={status}
                    onChange={e => setStatus(e.target.value as BoardTaskStatus)}
                    className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground outline-none"
                  >
                    {STATUS_COLUMNS.map(s => (
                      <option key={s.id} value={s.id}>{s.label}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-ui-xs font-semibold text-foreground mb-1">
                    Priority
                  </label>
                  <select
                    value={priority}
                    onChange={e => setPriority(e.target.value as BoardTaskPriority)}
                    className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground outline-none"
                  >
                    <option value="urgent">Urgent</option>
                    <option value="high">High</option>
                    <option value="medium">Medium</option>
                    <option value="low">Low</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-ui-xs font-semibold text-foreground mb-1">
                    Assignee / Model
                  </label>
                  <select
                    value={assignee}
                    onChange={e => setAssignee(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground outline-none"
                  >
                    <option value="Pandu-Micro">Pandu-Micro (Default Offline)</option>
                    <option value="Forge Agent">Forge ADE Agent</option>
                    <option value="TypeSafe Jev">TypeSafe Jev</option>
                    <option value="Claude-3.7-Sonnet">Claude 3.7 Sonnet</option>
                    <option value="DeepSeek-R1">DeepSeek R1</option>
                  </select>
                </div>

                <div>
                  <label className="block text-ui-xs font-semibold text-foreground mb-1">
                    Labels
                  </label>
                  <input
                    type="text"
                    value={labels}
                    onChange={e => setLabels(e.target.value)}
                    placeholder="core, api, backend"
                    className="w-full px-3 py-2 rounded-lg border border-border bg-surface text-ui-sm text-foreground outline-none"
                  />
                </div>
              </div>

              {!editingTask && (
                <label className="flex items-center gap-2 text-ui-xs text-foreground cursor-pointer pt-1">
                  <input
                    type="checkbox"
                    checked={startImmediately}
                    onChange={e => setStartImmediately(e.target.checked)}
                    className="rounded border-border text-primary focus:ring-0"
                  />
                  <span>Launch immediately in Pandu-Micro Agent Harness</span>
                </label>
              )}

              <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-3.5 py-1.5 rounded-lg border border-border bg-surface text-foreground text-ui-sm font-medium hover:bg-surface-hover cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 rounded-lg bg-primary text-primary-foreground text-ui-sm font-medium hover:bg-primary/90 cursor-pointer shadow-sm"
                >
                  {editingTask ? 'Save Changes' : 'Create Task'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export const JiraBoardView = WorkflowBoardView;

