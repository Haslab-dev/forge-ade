import { useState } from 'react';
import {
  Terminal,
  Cpu,
  GitBranch,
  Bot,
  Layers,
  Check,
  Copy,
  ExternalLink,
  ChevronRight,
  ShieldCheck,
  Monitor
} from 'lucide-react';

interface ShowcaseItem {
  id: string;
  title: string;
  subtitle: string;
  category: string;
  image: string;
  caption: string;
  bullets: string[];
}

const SHOWCASE_ITEMS: ShowcaseItem[] = [
  {
    id: 'internal-agent',
    title: 'Autonomous Agent Harness',
    subtitle: 'Multi-step reasoning loop with live task checklist & approval gates',
    category: 'Built-in Agent',
    image: '/internal-agent-harness.png',
    caption: 'The built-in Go autonomous agent decomposing objectives, editing codebase files, and requesting user approvals.',
    bullets: [
      'Multi-step autonomous reasoning with loop detection & tool budgeting',
      'Live floating task decomposition checklist directly above composer',
      'Granular safety approval gates for bash commands and destructive writes',
      'Instant context attachments via @mentions and token-aware prompt routing'
    ]
  },
  {
    id: 'agent-cli',
    title: 'External Agent CLIs in PTY',
    subtitle: 'Run Claude Code, Codex, Pi, OpenCode, and AGY in real terminals',
    category: 'Dual Session',
    image: '/agent-cli.png',
    caption: 'Hardware-accelerated PTY session hosting external terminal agents with full persistent scrollback.',
    bullets: [
      'Real pseudo-terminal allocation via creack/pty and xterm.js',
      'WebGL hardware acceleration for instant rendering and zero input lag',
      'Persistent session history stored in ~/.forge/sessions/',
      'Seamless coexistence between built-in autonomous harness and CLI agents'
    ]
  },
  {
    id: 'code-editor',
    title: 'Precision Code Editor',
    subtitle: 'CodeMirror 6 with syntax highlights, split views, and tabs',
    category: 'Editor',
    image: '/code-editor.png',
    caption: 'High-density code editing surface with tabs, breadcrumb bar, and file tree integration.',
    bullets: [
      'Powered by CodeMirror 6 with support for 20+ programming languages',
      'Multi-tab management with split-view and dirty buffer persistence',
      'Integrated LSP diagnostic markers and symbol search',
      'Zero Electron lag — near-instant syntax rendering on large files'
    ]
  },
  {
    id: 'git-client',
    title: 'Visual Git Client & Diffs',
    subtitle: 'Interactive commit graphs, staging, branch control, and diff reviews',
    category: 'Version Control',
    image: '/git-client.png',
    caption: 'Full-featured Git control panel with visual commit lane graph and line-by-line diff inspector.',
    bullets: [
      'Visual topological commit graph directly inside the IDE',
      'Stage, unstage, commit, and push without context switching to shell',
      'Interactive split & unified diff inspectors with syntax highlighting',
      'Instant branch switching and stash management'
    ]
  },
  {
    id: 'agent-acp',
    title: 'Agent Client Protocol (ACP)',
    subtitle: 'Standardized agent protocol with real-time usage quota metrics',
    category: 'Telemetry',
    image: '/agent-acp.png',
    caption: 'ACP protocol connection view displaying active session parameters and token usage quotas.',
    bullets: [
      'Full compliance with Agent Client Protocol specification',
      'Live token counter, context window consumption, and cost tracking',
      'Support for local models and remote inference endpoints',
      'Per-session quota limits and runaway spend protection'
    ]
  },
  {
    id: 'sidebar-features',
    title: 'Workspace Chrome & Subagents',
    subtitle: 'Full-height glass sidebar with explorer, activities, and research agents',
    category: 'Workspace',
    image: '/sidebar-features.png',
    caption: 'Clean collapsible workspace sidebar organizing files, activities, subagents, and tools.',
    bullets: [
      'macOS native traffic-light glass chrome with zero visual clutter',
      'Background subagent delegation (invoke_subagent) for parallel research',
      'Quick action drawers for terminal docks, scratchpads, and databases',
      'Unified workspace context shared across editor and agent surfaces'
    ]
  },
  {
    id: 'slides-motion',
    title: 'Decks & Motion Preview',
    subtitle: 'Built-in slide presentation and animation viewer in the side panel',
    category: 'Preview Tools',
    image: '/slides-and-motion-viewer.png',
    caption: 'Integrated presentation and motion graphics viewer running in the right auxiliary drawer.',
    bullets: [
      'Render HTML/SVG motion decks and interactive design previews directly in-app',
      'Embedded alongside active code or agent conversations in split pane',
      'Real-time live reload when modifying presentation assets',
      'Exportable standalone views for client walk-throughs'
    ]
  }
];

export default function App() {
  const [activeTab, setActiveTab] = useState<string>('internal-agent');
  const [copiedClone, setCopiedClone] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  const activeItem = SHOWCASE_ITEMS.find(item => item.id === activeTab) || SHOWCASE_ITEMS[0];

  const handleCopyClone = () => {
    navigator.clipboard.writeText('git clone https://github.com/Haslab-dev/forge-ade.git');
    setCopiedClone(true);
    setTimeout(() => setCopiedClone(false), 2000);
  };

  const handleCopyCurl = () => {
    navigator.clipboard.writeText('cd forge-ade && bun install && go build -o forge-ade .');
    setCopiedCurl(true);
    setTimeout(() => setCopiedCurl(false), 2000);
  };

  return (
    <div className="min-h-screen bg-[#09090b] text-[#f4f4f5] selection:bg-zinc-800 selection:text-white flex flex-col">
      {/* ── Top Navigation Bar ── */}
      <header className="sticky top-0 z-50 w-full border-b border-white/[0.06] bg-[#09090b]/80 backdrop-blur-xl">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <img src="/favicon.svg" alt="ForgeADE Logo" className="w-8 h-8 rounded-lg" />
            <div className="flex items-center gap-2">
              <span className="font-semibold text-lg tracking-tight text-white">ForgeADE</span>
              <span className="px-2 py-0.5 rounded-full text-xs font-mono font-medium bg-zinc-800/90 text-zinc-300 border border-zinc-700/60 flex items-center gap-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                v1.0.7
              </span>
            </div>
          </div>

          <nav className="hidden md:flex items-center gap-7 text-sm font-medium text-zinc-400">
            <a href="#features" className="transition-colors hover:text-white">Features</a>
            <a href="#showcase" className="transition-colors hover:text-white">Showcase</a>
            <a href="#architecture" className="transition-colors hover:text-white">Architecture</a>
            <a href="#benchmarks" className="transition-colors hover:text-white">Benchmarks</a>
            <a href="#quickstart" className="transition-colors hover:text-white">Quickstart</a>
          </nav>

          <div className="flex items-center gap-3">
            <a
              href="https://github.com/Haslab-dev/forge-ade"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 px-3.5 py-1.5 text-xs font-medium rounded-lg border border-zinc-800 bg-zinc-900/80 hover:bg-zinc-800 hover:text-white text-zinc-300 transition-all shadow-sm"
            >
              <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
              </svg>
              <span>GitHub</span>
            </a>

            <a
              href="#quickstart"
              className="hidden sm:inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold rounded-lg bg-zinc-100 hover:bg-white text-zinc-950 transition-all shadow-sm"
            >
              <span>Get Started</span>
              <ChevronRight className="w-3.5 h-3.5" />
            </a>
          </div>
        </div>
      </header>

      {/* ── Hero Section ── */}
      <section className="relative pt-20 pb-16 md:pt-28 md:pb-24 overflow-hidden border-b border-white/[0.06]">
        {/* Subtle geometric background grid */}
        <div className="absolute inset-0 bg-grid-pattern opacity-60 pointer-events-none" />
        
        {/* Subtle restrained glow - purely dark graphite to light tint, NO AI slop gradients */}
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 w-[720px] h-[360px] bg-white/[0.02] rounded-full blur-3xl pointer-events-none" />

        <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          {/* Release badge */}
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-mono font-medium border border-zinc-800 bg-zinc-900/90 text-zinc-300 mb-8 shadow-sm">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
            <span>ForgeADE v1.0.7 — Native Desktop AI Development Environment</span>
          </div>

          {/* Main Headline */}
          <h1 className="text-4xl sm:text-6xl lg:text-7xl font-bold tracking-tight text-white max-w-5xl mx-auto leading-[1.08]">
            The Native AI Development Environment Built for Velocity.
          </h1>

          {/* Subtitle */}
          <p className="mt-6 text-lg sm:text-xl text-zinc-400 max-w-3xl mx-auto font-normal leading-relaxed">
            No Electron bloat. Zero cloud lock-in. A built-in autonomous agent reasoning harness, real PTY terminal sessions, CodeMirror 6, and Git orchestration — engineered with Go and Wails v3 into a single, high-performance binary.
          </p>

          {/* CTA Group */}
          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4 max-w-xl mx-auto">
            {/* Quick Clone Snippet */}
            <div className="w-full sm:w-auto flex items-center justify-between gap-3 px-4 py-2.5 rounded-xl border border-zinc-800 bg-zinc-900/90 font-mono text-xs text-zinc-300 shadow-sm hover:border-zinc-700 transition-colors">
              <span className="text-zinc-500">$</span>
              <span className="select-all">git clone https://github.com/Haslab-dev/forge-ade.git</span>
              <button
                type="button"
                onClick={handleCopyClone}
                title="Copy to clipboard"
                className="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-white transition-colors"
              >
                {copiedClone ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>

            <a
              href="https://github.com/Haslab-dev/forge-ade"
              target="_blank"
              rel="noreferrer"
              className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-xl text-xs font-semibold bg-white text-zinc-950 hover:bg-zinc-200 transition-all shadow-md"
            >
              <span>View Source</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </div>

          {/* Technical Spec Metrics */}
          <div className="mt-14 pt-10 border-t border-white/[0.06] grid grid-cols-2 md:grid-cols-4 gap-6 max-w-4xl mx-auto text-left">
            <div className="p-4 rounded-xl border border-zinc-800/80 bg-zinc-900/40">
              <div className="text-2xl font-bold font-mono text-white tracking-tight">&lt; 100ms</div>
              <div className="text-xs text-zinc-400 mt-1 font-medium">Cold App Launch</div>
              <div className="text-[11px] text-zinc-500 mt-0.5">Native Go + OS Webview</div>
            </div>

            <div className="p-4 rounded-xl border border-zinc-800/80 bg-zinc-900/40">
              <div className="text-2xl font-bold font-mono text-white tracking-tight">~80 MB</div>
              <div className="text-xs text-zinc-400 mt-1 font-medium">Base RAM Footprint</div>
              <div className="text-[11px] text-zinc-500 mt-0.5">vs 1.5GB+ in Electron IDEs</div>
            </div>

            <div className="p-4 rounded-xl border border-zinc-800/80 bg-zinc-900/40">
              <div className="text-2xl font-bold font-mono text-white tracking-tight">Dual-Mode</div>
              <div className="text-xs text-zinc-400 mt-1 font-medium">Agent Engine</div>
              <div className="text-[11px] text-zinc-500 mt-0.5">Go Harness + PTY CLI Agents</div>
            </div>

            <div className="p-4 rounded-xl border border-zinc-800/80 bg-zinc-900/40">
              <div className="text-2xl font-bold font-mono text-white tracking-tight">100% Local</div>
              <div className="text-xs text-zinc-400 mt-1 font-medium">Offline-First Core</div>
              <div className="text-[11px] text-zinc-500 mt-0.5">Bring Your Own LLM Keys</div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Interactive Showcase Section ── */}
      <section id="showcase" className="py-20 border-b border-white/[0.06] bg-zinc-950/60">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-12">
            <span className="text-xs font-mono font-semibold uppercase tracking-wider text-zinc-400">Retina Interface Tour</span>
            <h2 className="mt-2 text-3xl sm:text-4xl font-bold tracking-tight text-white">
              Engineered with uncompromising precision.
            </h2>
            <p className="mt-3 text-zinc-400 text-sm sm:text-base">
              Every pixel, pane, and interaction is designed for senior developers who demand keyboard ergonomics and zero bloat.
            </p>
          </div>

          {/* Navigation Pill Tabs */}
          <div className="flex items-center justify-start sm:justify-center gap-2 overflow-x-auto pb-4 no-scrollbar">
            {SHOWCASE_ITEMS.map(item => {
              const active = item.id === activeTab;
              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`px-3.5 py-2 rounded-xl text-xs font-medium whitespace-nowrap transition-all border ${
                    active
                      ? 'bg-zinc-100 text-zinc-950 border-white shadow-sm font-semibold'
                      : 'bg-zinc-900/80 text-zinc-400 border-zinc-800 hover:text-white hover:border-zinc-700'
                  }`}
                >
                  {item.title}
                </button>
              );
            })}
          </div>

          {/* Screenshot Card & Details */}
          <div className="mt-8 rounded-2xl border border-zinc-800 bg-[#111114] p-3 sm:p-5 shadow-2xl glow-card">
            {/* Window titlebar simulation */}
            <div className="flex items-center justify-between pb-3 px-2 border-b border-white/[0.06] mb-3">
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-[#ff5f56]" />
                <div className="w-3 h-3 rounded-full bg-[#ffbd2e]" />
                <div className="w-3 h-3 rounded-full bg-[#27c93f]" />
                <span className="ml-3 font-mono text-xs text-zinc-400 font-medium">ForgeADE — {activeItem.title}</span>
              </div>
              <span className="text-[11px] font-mono text-zinc-500">{activeItem.category}</span>
            </div>

            {/* High-res Image Container */}
            <div className="relative rounded-xl overflow-hidden border border-white/[0.04] bg-black">
              <img
                src={activeItem.image}
                alt={activeItem.title}
                className="w-full h-auto object-cover max-h-[720px] transition-all duration-300"
              />
            </div>

            {/* Detail Footer Grid */}
            <div className="mt-6 pt-5 border-t border-white/[0.06] grid grid-cols-1 md:grid-cols-3 gap-6 px-2">
              <div className="md:col-span-1">
                <span className="text-xs font-mono font-medium text-zinc-400 uppercase tracking-wider">{activeItem.category}</span>
                <h3 className="text-xl font-bold text-white mt-1">{activeItem.title}</h3>
                <p className="text-xs text-zinc-400 mt-2 leading-relaxed">{activeItem.subtitle}</p>
                <p className="text-[11px] text-zinc-500 mt-3 italic">{activeItem.caption}</p>
              </div>

              <div className="md:col-span-2 grid grid-cols-1 sm:grid-cols-2 gap-3">
                {activeItem.bullets.map((b, idx) => (
                  <div key={idx} className="flex items-start gap-2.5 p-3 rounded-lg border border-zinc-800/80 bg-zinc-900/40">
                    <Check className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
                    <span className="text-xs text-zinc-300 leading-snug">{b}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Features Deep Dive ── */}
      <section id="features" className="py-20 border-b border-white/[0.06]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <span className="text-xs font-mono font-semibold uppercase tracking-wider text-zinc-400">Architectural Capabilities</span>
            <h2 className="mt-2 text-3xl sm:text-4xl font-bold tracking-tight text-white">
              Everything senior engineers need, natively integrated.
            </h2>
            <p className="mt-3 text-zinc-400 text-sm sm:text-base">
              Say goodbye to fragmented electron windows, detached terminal tabs, and inconsistent AI assistants.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Feature 1 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <Bot className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">Autonomous Go Reasoning Loop</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Executes multi-step prompts with configurable tool budgets, auto loop detection, and safe approval checkpoints. Decomposes high-level goals into dynamic checklists.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>update_tasks</span> · <span>invoke_subagent</span> · <span>approval_gate</span>
              </div>
            </div>

            {/* Feature 2 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <Terminal className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">Dual-Session PTY Terminal</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Run external agent CLIs (Claude Code, Codex, Pi, OpenCode, AGY) in real hardware-accelerated terminals via <code className="text-zinc-200">creack/pty</code> and <code className="text-zinc-200">xterm.js WebGL</code>.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>persistent scrollback</span> · <span>pty multiplexing</span>
              </div>
            </div>

            {/* Feature 3 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <GitBranch className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">Integrated Git Graph Suite</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Interactive topological Git graph, file staging, split-diff syntax inspector, branch switcher chip, and one-click commit creation directly in your workspace.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>topological lane graph</span> · <span>syntax diffs</span>
              </div>
            </div>

            {/* Feature 4 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <Monitor className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">In-App Browser & Computer Use</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Native macOS WKWebView engine and Chrome DevTools Protocol automation. Agents can snapshot DOM accessibility trees, interact with element refs, and control desktop apps.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>WKWebView</span> · <span>CDP headless</span> · <span>CoreGraphics</span>
              </div>
            </div>

            {/* Feature 5 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <Layers className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">Kanban Board & Automations</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Native task board synced with <code className="text-zinc-200">.forge/board_tasks.json</code> in real time via fsnotify file watchers. Schedule prompt automations with 5-field cron.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>board_watcher</span> · <span>cron scheduling</span>
              </div>
            </div>

            {/* Feature 6 */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114] hover:border-zinc-700 transition-all">
              <div className="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white mb-4">
                <Cpu className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-white">Bring Your Own Models</h3>
              <p className="text-xs text-zinc-400 mt-2 leading-relaxed">
                Connect Anthropic Claude 3.7 / 3.5, OpenAI GPT-4o, OpenRouter, Antigravity / CloudCode, or local Ollama instances. Includes Jev semantic intent routing to curb token bloat.
              </p>
              <div className="mt-4 pt-4 border-t border-white/[0.06] flex items-center gap-2 text-xs font-mono text-zinc-500">
                <span>Anthropic</span> · <span>OpenAI</span> · <span>Ollama</span> · <span>Jev</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Architecture Section (Modular Monolith) ── */}
      <section id="architecture" className="py-20 border-b border-white/[0.06] bg-zinc-950/60">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <span className="text-xs font-mono font-semibold uppercase tracking-wider text-zinc-400">Under the Hood</span>
            <h2 className="mt-2 text-3xl sm:text-4xl font-bold tracking-tight text-white">
              Modular Monolith + Feature-Oriented Frontend
            </h2>
            <p className="mt-3 text-zinc-400 text-sm sm:text-base">
              A clean separation of concerns. Pure Go domain modules, contract-driven IPC, and fine-grained Zustand stores with zero React Context re-render penalties.
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Column 1: Go Backend */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114]">
              <div className="flex items-center justify-between pb-3 border-b border-white/[0.06] mb-4">
                <span className="text-xs font-mono font-semibold text-zinc-300">Go Backend</span>
                <span className="text-[11px] font-mono text-zinc-500">Go 1.26 + Wails v3</span>
              </div>
              <ul className="space-y-3 text-xs text-zinc-400">
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">internal/bootstrap:</span>
                  <span>Composition root and explicit service wiring</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">internal/app:</span>
                  <span>Thin facade DTO mapping & event bridge</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">internal/board:</span>
                  <span>Kanban board file persistence & debounced watcher</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">internal/quota:</span>
                  <span>Real-time model quota tracking & token accounting</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">internal/browseruse:</span>
                  <span>WebKit native subview and CDP driver</span>
                </li>
              </ul>
            </div>

            {/* Column 2: Contract-Driven IPC */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114]">
              <div className="flex items-center justify-between pb-3 border-b border-white/[0.06] mb-4">
                <span className="text-xs font-mono font-semibold text-zinc-300">Contract IPC</span>
                <span className="text-[11px] font-mono text-zinc-500">Type-Safe Bridge</span>
              </div>
              <ul className="space-y-3 text-xs text-zinc-400">
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">contracts.ts:</span>
                  <span>Exported Go method wire contracts</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">events.ts:</span>
                  <span>Typed event listener registry with teardowns</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">errors:</span>
                  <span>Classified error boundaries (Workspace, Git, Terminal)</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">apiBridge.ts:</span>
                  <span>Clean domain-scoped service facades</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">zero-overhead:</span>
                  <span>Direct memory IPC via OS native webview</span>
                </li>
              </ul>
            </div>

            {/* Column 3: React 19 Frontend */}
            <div className="p-6 rounded-2xl border border-zinc-800 bg-[#111114]">
              <div className="flex items-center justify-between pb-3 border-b border-white/[0.06] mb-4">
                <span className="text-xs font-mono font-semibold text-zinc-300">React 19 Frontend</span>
                <span className="text-[11px] font-mono text-zinc-500">Zustand + Tailwind v4</span>
              </div>
              <ul className="space-y-3 text-xs text-zinc-400">
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">features/:</span>
                  <span>Feature-oriented modules (agent, git, editor, board)</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">stores/:</span>
                  <span>Domain stores (useLayoutStore, useGitStore, etc.)</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">zero context:</span>
                  <span>Eliminated root Context Provider re-renders</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">tokens:</span>
                  <span>Unified semantic design system palette</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-zinc-200 font-mono">testing:</span>
                  <span>228 unit tests with Vitest and Testing Library</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* ── Benchmarks & Comparison Table ── */}
      <section id="benchmarks" className="py-20 border-b border-white/[0.06]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <span className="text-xs font-mono font-semibold uppercase tracking-wider text-zinc-400">Comparative Metrics</span>
            <h2 className="mt-2 text-3xl sm:text-4xl font-bold tracking-tight text-white">
              ForgeADE vs Electron IDEs & Terminal-Only
            </h2>
            <p className="mt-3 text-zinc-400 text-sm sm:text-base">
              Why engineers switch from heavy web-wrapper IDEs to native desktop software.
            </p>
          </div>

          <div className="overflow-x-auto rounded-2xl border border-zinc-800 bg-[#111114]">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-zinc-800 bg-zinc-900/60 font-mono text-xs text-zinc-300">
                <tr>
                  <th className="py-4 px-6 font-semibold">Capability</th>
                  <th className="py-4 px-6 font-semibold text-white bg-zinc-800/40 border-x border-zinc-700/60">ForgeADE</th>
                  <th className="py-4 px-6 font-semibold text-zinc-400">Electron-Based AI IDEs</th>
                  <th className="py-4 px-6 font-semibold text-zinc-400">Terminal-Only CLI Agents</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/60 font-mono text-xs">
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Runtime Engine</td>
                  <td className="py-3.5 px-6 font-bold text-white bg-zinc-800/20 border-x border-zinc-700/60">Native Go + Wails v3</td>
                  <td className="py-3.5 px-6 text-zinc-400">Electron + Node.js + Chrome</td>
                  <td className="py-3.5 px-6 text-zinc-400">Python / Rust / Node CLI</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Cold Startup Time</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">&lt; 100 ms</td>
                  <td className="py-3.5 px-6 text-zinc-400">3.5 – 6.0 seconds</td>
                  <td className="py-3.5 px-6 text-zinc-400">&lt; 50 ms</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Base RAM Usage</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">~80 MB</td>
                  <td className="py-3.5 px-6 text-zinc-400">1,200 – 2,500 MB</td>
                  <td className="py-3.5 px-6 text-zinc-400">~30 – 80 MB</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Dual Session Modes</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">Yes (Go Harness + PTY CLI)</td>
                  <td className="py-3.5 px-6 text-zinc-400">No (Proprietary sidebar only)</td>
                  <td className="py-3.5 px-6 text-zinc-400">No (Terminal only)</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Integrated Git Graph</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">Native visual graph + diffs</td>
                  <td className="py-3.5 px-6 text-zinc-400">Basic / plugin dependent</td>
                  <td className="py-3.5 px-6 text-zinc-400">Manual git commands</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">In-App Browser Preview</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">Yes (WebKit native subview + CDP)</td>
                  <td className="py-3.5 px-6 text-zinc-400">Embedded iframe or external</td>
                  <td className="py-3.5 px-6 text-zinc-400">None (external browser only)</td>
                </tr>
                <tr>
                  <td className="py-3.5 px-6 font-sans font-medium text-zinc-200">Offline-First Core</td>
                  <td className="py-3.5 px-6 font-bold text-emerald-400 bg-zinc-800/20 border-x border-zinc-700/60">Yes (Full offline functionality)</td>
                  <td className="py-3.5 px-6 text-zinc-400">Requires online sign-in</td>
                  <td className="py-3.5 px-6 text-zinc-400">Yes</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* ── Quickstart Section ── */}
      <section id="quickstart" className="py-20 border-b border-white/[0.06] bg-zinc-950/60">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-2xl mx-auto mb-12">
            <span className="text-xs font-mono font-semibold uppercase tracking-wider text-zinc-400">Installation</span>
            <h2 className="mt-2 text-3xl sm:text-4xl font-bold tracking-tight text-white">
              Get running in under 60 seconds.
            </h2>
            <p className="mt-3 text-zinc-400 text-sm">
              ForgeADE builds into a single self-contained desktop binary. No complex runtime dependencies.
            </p>
          </div>

          <div className="rounded-2xl border border-zinc-800 bg-[#111114] p-6 shadow-2xl">
            <div className="flex items-center justify-between pb-4 border-b border-white/[0.06] mb-6">
              <div className="flex items-center gap-2">
                <Terminal className="w-4 h-4 text-zinc-400" />
                <span className="text-xs font-mono font-medium text-zinc-300">Quick Build from Source</span>
              </div>
              <button
                type="button"
                onClick={handleCopyCurl}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-mono text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
              >
                {copiedCurl ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                <span>{copiedCurl ? 'Copied' : 'Copy'}</span>
              </button>
            </div>

            <pre className="font-mono text-xs sm:text-sm text-zinc-300 overflow-x-auto p-4 rounded-xl bg-black border border-zinc-800/80 leading-relaxed">
              <span className="text-zinc-600"># 1. Clone the repository</span>{'\n'}
              <span className="text-emerald-400">git</span> clone https://github.com/Haslab-dev/forge-ade.git{'\n'}
              <span className="text-emerald-400">cd</span> forge-ade{'\n\n'}
              <span className="text-zinc-600"># 2. Install dependencies & verify frontend</span>{'\n'}
              <span className="text-emerald-400">cd</span> frontend && <span className="text-emerald-400">bun</span> install && <span className="text-emerald-400">bun</span> run build && <span className="text-emerald-400">cd</span> ..{'\n\n'}
              <span className="text-zinc-600"># 3. Build native desktop binary with Go</span>{'\n'}
              <span className="text-emerald-400">go</span> build -o build/bin/ForgeADE .{'\n\n'}
              <span className="text-zinc-600"># 4. Launch ForgeADE</span>{'\n'}
              ./build/bin/ForgeADE
            </pre>

            <div className="mt-6 pt-5 border-t border-white/[0.06] flex flex-wrap items-center justify-between gap-4 text-xs text-zinc-400">
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-400" />
                <span>Open Source · Apache 2.0 / MIT License</span>
              </div>
              <div className="flex items-center gap-4 font-mono">
                <span>macOS (Apple Silicon & Intel)</span>
                <span>Linux</span>
                <span>Windows</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="py-12 bg-[#09090b] text-zinc-500 text-xs">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-3">
            <img src="/favicon.svg" alt="ForgeADE" className="w-5 h-5 rounded" />
            <span className="text-zinc-400 font-medium">ForgeADE</span>
            <span>—</span>
            <span>Crafted by Haslab Development</span>
          </div>

          <div className="flex items-center gap-6">
            <a
              href="https://github.com/Haslab-dev/forge-ade"
              target="_blank"
              rel="noreferrer"
              className="hover:text-zinc-300 transition-colors flex items-center gap-1"
            >
              <span>GitHub Repository</span>
              <ExternalLink className="w-3 h-3" />
            </a>
            <a
              href="https://github.com/Haslab-dev/forge-ade/blob/main/docs/ARCHITECTURE.md"
              target="_blank"
              rel="noreferrer"
              className="hover:text-zinc-300 transition-colors"
            >
              Architecture Docs
            </a>
            <a
              href="#"
              className="hover:text-zinc-300 transition-colors"
            >
              Back to Top ↑
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
