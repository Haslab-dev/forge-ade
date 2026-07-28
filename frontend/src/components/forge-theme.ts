import { EditorView } from "@codemirror/view";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";

const bg = "#1A2130";
const bgActive = "#20293A";
const fg = "#D4D4D4";
const gutter = "#6B7280";
const scrollbar = "#2E3A52";

export const forgeTheme = EditorView.theme({
  "&": { backgroundColor: bg, color: fg, height: "100%" },
  ".cm-content": { caretColor: "#4F8CFF", fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace", fontSize: "13px", padding: "16px" },
  ".cm-cursor": { borderLeftColor: "#4F8CFF", borderLeftWidth: "2px" },
  ".cm-selectionBackground, ::selection": { backgroundColor: "#335CFF55" },
  "&.cm-focused .cm-selectionBackground": { backgroundColor: "#335CFF77" },
  ".cm-gutters": { backgroundColor: bg, color: gutter, border: "none" },
  ".cm-activeLine": { backgroundColor: bgActive },
  ".cm-activeLineGutter": { backgroundColor: bgActive, color: "#FFFFFF", fontWeight: 600 },
  ".cm-scroller": { overflow: "auto", fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace", fontSize: "13px" },
  ".cm-matchingBracket": { color: "#FFFFFF", backgroundColor: "rgba(79,140,255,.15)", outline: "1px solid rgba(79,140,255,.3)" },
  ".cm-searchMatch": { backgroundColor: "rgba(255,213,79,.15)" },
  ".cm-searchMatch-selected": { backgroundColor: "rgba(255,213,79,.35)" },
  ".cm-search": { backgroundColor: bg, borderBottom: "1px solid #2E3A52", padding: "8px", fontSize: "13px", fontFamily: "sans-serif" },
  ".cm-search input": { background: "#111827", border: "1px solid #2E3A52", borderRadius: "4px", padding: "4px 8px", color: fg, outline: "none" },
  ".cm-search input:focus": { borderColor: "#4F8CFF" },
  ".cm-search button": { background: "#20293A", border: "none", borderRadius: "4px", padding: "4px 8px", color: fg, cursor: "pointer", fontSize: "12px" },
  ".cm-search button:hover": { background: "#2E3A52" },
  ".cm-search label": { color: gutter, fontSize: "12px" },
  ".cm-editor": { borderRadius: "8px", overflow: "hidden", boxShadow: "inset 0 1px 0 rgba(255,255,255,.02), inset 0 -1px 0 rgba(0,0,0,.25)" },
  ".cm-scroller::-webkit-scrollbar": { width: "10px" },
  ".cm-scroller::-webkit-scrollbar-thumb": { background: scrollbar, borderRadius: "999px" },
  ".cm-scroller::-webkit-scrollbar-track": { background: "transparent" },
  ".cm-foldPlaceholder": { backgroundColor: bgActive, color: gutter, border: "none" },
});

export const forgeHighlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: tags.keyword, color: "#4F8CFF" },
    { tag: tags.definitionKeyword, color: "#4F8CFF" },
    { tag: tags.moduleKeyword, color: "#4F8CFF" },
    { tag: tags.controlKeyword, color: "#BB9AF7" },
    { tag: tags.operatorKeyword, color: "#4F8CFF" },
    { tag: tags.operator, color: "#89B4FA" },
    { tag: tags.regexp, color: "#F38BA8" },
    { tag: tags.string, color: "#A6E3A1" },
    { tag: tags.character, color: "#F9E2AF" },
    { tag: tags.number, color: "#F9E2AF" },
    { tag: tags.bool, color: "#F9E2AF" },
    { tag: tags.null, color: "#F9E2AF" },
    { tag: tags.atom, color: "#89B4FA" },
    { tag: tags.propertyName, color: "#89B4FA" },
    { tag: tags.attributeName, color: "#89B4FA" },
    { tag: tags.variableName, color: "#F5C2E7" },
    { tag: tags.typeName, color: "#F9E2AF" },
    { tag: tags.className, color: "#F9E2AF" },
    { tag: tags.self, color: "#F5C2E7" },
    { tag: tags.comment, color: "#6B7280", fontStyle: "italic" },
    { tag: tags.docComment, color: "#6B7280", fontStyle: "italic" },
    { tag: tags.meta, color: "#6B7280" },
    { tag: tags.separator, color: "#565F89" },
    { tag: tags.heading, color: "#4F8CFF", fontWeight: "bold" },
    { tag: tags.strong, color: fg, fontWeight: "bold" },
    { tag: tags.emphasis, color: fg, fontStyle: "italic" },
    { tag: tags.link, color: "#89B4FA" },
    { tag: tags.strikethrough, color: "#6B7280" },
    { tag: tags.deleted, color: "#F38BA8" },
    { tag: tags.inserted, color: "#A6E3A1" },
    { tag: tags.changed, color: "#F9E2AF" },
    { tag: tags.invalid, color: "#F38BA8" },
    { tag: tags.labelName, color: "#A6E3A1" },
    { tag: tags.namespace, color: "#BB9AF7" },
    { tag: tags.literal, color: "#F9E2AF" },
    { tag: tags.escape, color: "#F38BA8" },
  ])
);
