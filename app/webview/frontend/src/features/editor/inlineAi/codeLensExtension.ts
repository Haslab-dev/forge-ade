import { WidgetType, EditorView, Decoration, DecorationSet } from '@codemirror/view';
import { StateField, StateEffect } from '@codemirror/state';
import { detectSymbolsInDoc } from './symbolDetector';
import { SymbolLensInfo, CodeLensAction } from './inlineAiTypes';

export const setCodeLensEffect = StateEffect.define<SymbolLensInfo[]>();

class CodeLensWidget extends WidgetType {
  constructor(
    readonly symbol: SymbolLensInfo,
    readonly onTrigger: (symbol: SymbolLensInfo, action: CodeLensAction) => void
  ) {
    super();
  }

  toDOM() {
    const wrap = document.createElement('div');
    wrap.className = 'cm-code-lens-line flex items-center gap-1.5 py-0.5 select-none text-[11px] font-sans text-foreground-subtle/80';
    wrap.style.lineHeight = '1.4';

    const items: { label: string; action: CodeLensAction; icon: string }[] = [
      { label: 'Refactor', action: 'refactor', icon: '✨' },
      { label: 'Unit Tests', action: 'test', icon: '🧪' },
      { label: 'Explain', action: 'explain', icon: '💬' },
      { label: 'Optimize', action: 'optimize', icon: '⚡' }
    ];

    items.forEach((item, index) => {
      if (index > 0) {
        const sep = document.createElement('span');
        sep.textContent = '·';
        sep.className = 'text-border';
        wrap.appendChild(sep);
      }

      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'inline-flex items-center gap-1 hover:text-primary transition-colors cursor-pointer hover:underline';
      btn.innerHTML = `<span>${item.icon}</span><span>${item.label}</span>`;
      btn.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        this.onTrigger(this.symbol, item.action);
      };
      wrap.appendChild(btn);
    });

    return wrap;
  }

  ignoreEvent() {
    return false;
  }
}

export function createCodeLensExtension(
  onTrigger: (symbol: SymbolLensInfo, action: CodeLensAction) => void
) {
  return StateField.define<DecorationSet>({
    create(state) {
      const symbols = detectSymbolsInDoc(state.doc.toString());
      const widgets: any[] = [];
      for (const sym of symbols) {
        if (sym.from <= state.doc.length) {
          const deco = Decoration.widget({
            widget: new CodeLensWidget(sym, onTrigger),
            side: -1,
            block: true
          });
          widgets.push(deco.range(sym.from));
        }
      }
      return Decoration.set(widgets, true);
    },
    update(deco, tr) {
      if (!tr.docChanged) return deco;
      const symbols = detectSymbolsInDoc(tr.state.doc.toString());
      const widgets: any[] = [];
      for (const sym of symbols) {
        if (sym.from <= tr.state.doc.length) {
          const w = Decoration.widget({
            widget: new CodeLensWidget(sym, onTrigger),
            side: -1,
            block: true
          });
          widgets.push(w.range(sym.from));
        }
      }
      return Decoration.set(widgets, true);
    },
    provide: (f) => EditorView.decorations.from(f)
  });
}
