import { EditorView } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting, type LanguageSupport } from '@codemirror/language';
import { tags as t } from '@lezer/highlight';
import { oneDarkHighlightStyle } from '@codemirror/theme-one-dark';

// Language packages are loaded lazily (one chunk per language) so the editor
// bundle stays small and only pays for languages actually opened.
const LANG_LOADERS: Record<string, () => Promise<LanguageSupport>> = {
  js: () => import('@codemirror/lang-javascript').then(m => m.javascript()),
  mjs: () => import('@codemirror/lang-javascript').then(m => m.javascript()),
  cjs: () => import('@codemirror/lang-javascript').then(m => m.javascript()),
  jsx: () => import('@codemirror/lang-javascript').then(m => m.javascript({ jsx: true })),
  ts: () => import('@codemirror/lang-javascript').then(m => m.javascript({ typescript: true })),
  mts: () => import('@codemirror/lang-javascript').then(m => m.javascript({ typescript: true })),
  cts: () => import('@codemirror/lang-javascript').then(m => m.javascript({ typescript: true })),
  tsx: () => import('@codemirror/lang-javascript').then(m => m.javascript({ typescript: true, jsx: true })),
  go: () => import('@codemirror/lang-go').then(m => m.go()),
  md: () => import('@codemirror/lang-markdown').then(m => m.markdown()),
  markdown: () => import('@codemirror/lang-markdown').then(m => m.markdown()),
  xml: () => import('@codemirror/lang-xml').then(m => m.xml()),
  svg: () => import('@codemirror/lang-xml').then(m => m.xml()),
  xsl: () => import('@codemirror/lang-xml').then(m => m.xml()),
  xslt: () => import('@codemirror/lang-xml').then(m => m.xml()),
  json: () => import('@codemirror/lang-json').then(m => m.json()),
  jsonc: () => import('@codemirror/lang-json').then(m => m.json()),
  yml: () => import('@codemirror/lang-yaml').then(m => m.yaml()),
  yaml: () => import('@codemirror/lang-yaml').then(m => m.yaml()),
  html: () => import('@codemirror/lang-html').then(m => m.html()),
  htm: () => import('@codemirror/lang-html').then(m => m.html()),
  xhtml: () => import('@codemirror/lang-html').then(m => m.html()),
  vue: () => import('@codemirror/lang-vue').then(m => m.vue()),
  css: () => import('@codemirror/lang-css').then(m => m.css()),
  less: () => import('@codemirror/lang-less').then(m => m.less()),
  sass: () => import('@codemirror/lang-sass').then(m => m.sass()),
  scss: () => import('@codemirror/lang-sass').then(m => m.sass()),
  py: () => import('@codemirror/lang-python').then(m => m.python()),
  pyw: () => import('@codemirror/lang-python').then(m => m.python()),
  rs: () => import('@codemirror/lang-rust').then(m => m.rust()),
  php: () => import('@codemirror/lang-php').then(m => m.php()),
  phtml: () => import('@codemirror/lang-php').then(m => m.php()),
  java: () => import('@codemirror/lang-java').then(m => m.java()),
  c: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  h: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  cc: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  cpp: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  cxx: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  hpp: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  hh: () => import('@codemirror/lang-cpp').then(m => m.cpp()),
  sql: () => import('@codemirror/lang-sql').then(m => m.sql()),
};

const langCache = new Map<string, Promise<LanguageSupport | null>>();

// Resolves the CodeMirror language for a file name, or null when the extension
// has no dedicated package (renders as plain text). Failures degrade to plain.
export function loadLanguage(fileName: string): Promise<LanguageSupport | null> {
  const lower = fileName.toLowerCase();
  const ext = lower.includes('.') ? lower.split('.').pop() || '' : '';
  const loader = LANG_LOADERS[ext];
  if (!loader) return Promise.resolve(null);
  if (!langCache.has(ext)) {
    langCache.set(ext, loader().catch(() => null));
  }
  return langCache.get(ext)!;
}

// Editor chrome, built per color mode. User themes cannot use "&dark"/"&light"
// selector prefixes (those are reserved for CodeMirror's internal base themes
// and throw at build time), so light and dark get separate theme instances —
// the dark one marked with {dark: true} so CodeMirror's own dark rules apply.
const makeTheme = (dark: boolean) =>
  EditorView.theme(
    {
      '&': {
        fontSize: '12px',
        backgroundColor: 'transparent',
        height: '100%',
        color: dark ? '#e2e8f0' : '#111827',
      },
      '.cm-scroller': {
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, 'Courier New', monospace",
        lineHeight: '1.65',
        overflowY: 'auto',
        overflowX: 'auto',
      },
      '.cm-content': { caretColor: dark ? '#38bdf8' : '#2563eb' },
      '.cm-gutters': {
        backgroundColor: 'transparent',
        border: 'none',
        color: dark ? '#555555' : '#9ca3af',
      },
      '.cm-activeLine': { backgroundColor: dark ? 'rgba(56, 189, 248, 0.07)' : 'rgba(37, 99, 235, 0.06)' },
      '.cm-activeLineGutter': {
        backgroundColor: 'transparent',
        color: dark ? '#ffffff' : '#111827',
        fontWeight: '600',
      },
      '&.cm-focused': { outline: 'none' },
      '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
        backgroundColor: dark ? 'rgba(38, 79, 120, 0.7) !important' : 'rgba(191, 219, 254, 0.7) !important',
      },
      // Search matches & word occurrences highlighting (like VS Code)
      '.cm-searchMatch': {
        backgroundColor: dark ? 'rgba(234, 179, 8, 0.35)' : 'rgba(234, 179, 8, 0.4)',
        outline: dark ? '1px solid rgba(234, 179, 8, 0.6)' : '1px solid rgba(202, 138, 4, 0.7)',
        borderRadius: '2px',
      },
      '.cm-searchMatch.cm-searchMatch-selected': {
        backgroundColor: dark ? 'rgba(245, 158, 11, 0.85) !important' : 'rgba(217, 119, 6, 0.9) !important',
        color: '#000000 !important',
      },
      '.cm-selectionMatch': {
        backgroundColor: dark ? 'rgba(56, 189, 248, 0.2)' : 'rgba(37, 99, 235, 0.15)',
        outline: dark ? '1px solid rgba(56, 189, 248, 0.4)' : '1px solid rgba(37, 99, 235, 0.3)',
        borderRadius: '2px',
      },
      // Fold gutter (VS Code style collapse arrows)
      '.cm-foldGutter': {
        padding: '0 4px',
      },
      '.cm-foldGutter .cm-gutterElement': {
        padding: '0 2px',
        color: dark ? '#64748b' : '#94a3b8',
        cursor: 'pointer',
        transition: 'color 0.15s ease',
      },
      '.cm-foldGutter .cm-gutterElement:hover': {
        color: dark ? '#38bdf8' : '#2563eb',
      },
      '.cm-foldPlaceholder': {
        backgroundColor: dark ? '#1e293b' : '#e2e8f0',
        border: dark ? '1px solid #334155' : '1px solid #cbd5e1',
        color: dark ? '#94a3b8' : '#64748b',
        borderRadius: '3px',
        padding: '0 4px',
        margin: '0 2px',
        fontSize: '10px',
      },
      // VS Code style floating search widget
      '.cm-panels': {
        backgroundColor: 'transparent',
        border: 'none',
        zIndex: '30',
      },
      '.cm-panels-top': {
        borderBottom: 'none',
      },
      '.cm-panel.cm-search': {
        position: 'absolute',
        top: '8px',
        right: '24px',
        zIndex: '50',
        padding: '6px 8px',
        borderRadius: '8px',
        backgroundColor: dark ? '#18181b' : '#ffffff',
        border: dark ? '1px solid #27272a' : '1px solid #e4e4e7',
        boxShadow: dark ? '0 10px 25px -5px rgba(0,0,0,0.6), 0 8px 10px -6px rgba(0,0,0,0.6)' : '0 10px 25px -5px rgba(0,0,0,0.1), 0 8px 10px -6px rgba(0,0,0,0.1)',
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: '4px',
        fontSize: '11px',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        maxWidth: '440px',
      },
      '.cm-panel.cm-search input.cm-textfield': {
        height: '24px',
        padding: '2px 8px',
        borderRadius: '4px',
        fontSize: '11px',
        fontFamily: "ui-monospace, monospace",
        backgroundColor: dark ? '#09090b' : '#f4f4f5',
        border: dark ? '1px solid #27272a' : '1px solid #d4d4d8',
        color: dark ? '#fafafa' : '#09090b',
        outline: 'none',
      },
      '.cm-panel.cm-search input.cm-textfield:focus': {
        borderColor: dark ? '#38bdf8' : '#2563eb',
        boxShadow: dark ? '0 0 0 1px #38bdf8' : '0 0 0 1px #2563eb',
      },
      '.cm-panel.cm-search button.cm-button': {
        height: '24px',
        padding: '2px 8px',
        borderRadius: '4px',
        fontSize: '11px',
        fontWeight: '500',
        backgroundColor: dark ? '#27272a' : '#f4f4f5',
        border: dark ? '1px solid #3f3f46' : '1px solid #e4e4e7',
        color: dark ? '#e4e4e7' : '#27272a',
        cursor: 'pointer',
        transition: 'background-color 0.15s, color 0.15s',
      },
      '.cm-panel.cm-search button.cm-button:hover': {
        backgroundColor: dark ? '#3f3f46' : '#e4e4e7',
        color: dark ? '#ffffff' : '#09090b',
      },
      '.cm-panel.cm-search label': {
        display: 'inline-flex',
        alignItems: 'center',
        gap: '3px',
        fontSize: '10px',
        color: dark ? '#a1a1aa' : '#71717a',
        cursor: 'pointer',
        userSelect: 'none',
        padding: '2px 4px',
        borderRadius: '3px',
      },
      '.cm-panel.cm-search label:hover': {
        color: dark ? '#fafafa' : '#18181b',
        backgroundColor: dark ? '#27272a' : '#f4f4f5',
      },
      '.cm-panel.cm-search button[name="close"]': {
        marginLeft: 'auto',
        fontSize: '14px',
        lineHeight: '1',
        padding: '0 6px',
        background: 'transparent',
        border: 'none',
        color: dark ? '#71717a' : '#a1a1aa',
      },
      '.cm-panel.cm-search button[name="close"]:hover': {
        color: dark ? '#ef4444' : '#dc2626',
        background: 'transparent',
      },
      '.cm-tooltip': {
        border: dark ? '1px solid #383838' : '1px solid #e5e7eb',
        backgroundColor: dark ? '#222224' : '#ffffff',
        color: dark ? '#e5e7eb' : '#111827',
      },
      '.cm-tooltip-autocomplete ul li[aria-selected]': { backgroundColor: '#2563eb', color: '#ffffff' },
      '.cm-diagnostics': { fontSize: '11px' },
    },
    { dark },
  );

const lightTheme = makeTheme(false);
const darkTheme = makeTheme(true);

// Light-mode token colors tuned for the app's slate palette; dark mode reuses
// the One Dark highlight style (without its background theme).
const lightHighlightStyle = HighlightStyle.define([
  { tag: t.keyword, color: '#7c3aed' },
  { tag: [t.controlKeyword, t.moduleKeyword], color: '#9333ea' },
  { tag: [t.name, t.deleted, t.character, t.macroName], color: '#0f172a' },
  { tag: [t.function(t.variableName), t.labelName], color: '#2563eb' },
  { tag: [t.color, t.constant(t.name), t.standard(t.name)], color: '#0369a1' },
  { tag: [t.definition(t.name), t.separator], color: '#0f172a' },
  { tag: [t.typeName, t.className, t.number, t.changed, t.annotation, t.self, t.namespace], color: '#b45309' },
  { tag: [t.operator, t.operatorKeyword, t.url, t.escape, t.regexp, t.link, t.special(t.string)], color: '#0e7490' },
  { tag: [t.meta, t.comment], color: '#6b7280' },
  { tag: t.strong, fontWeight: 'bold' },
  { tag: t.emphasis, fontStyle: 'italic' },
  { tag: t.strikethrough, textDecoration: 'line-through' },
  { tag: t.link, color: '#2563eb', textDecoration: 'underline' },
  { tag: t.heading, fontWeight: 'bold', color: '#111827' },
  { tag: [t.atom, t.bool, t.special(t.variableName)], color: '#b45309' },
  { tag: [t.processingInstruction, t.string, t.inserted], color: '#047857' },
  { tag: t.invalid, color: '#dc2626' },
]);

export function themeExtensions(dark: boolean) {
  return dark
    ? [darkTheme, syntaxHighlighting(oneDarkHighlightStyle, { fallback: true })]
    : [lightTheme, syntaxHighlighting(lightHighlightStyle, { fallback: true })];
}
