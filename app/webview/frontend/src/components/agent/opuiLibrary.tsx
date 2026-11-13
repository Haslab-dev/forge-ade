import React from 'react';
import { z } from 'zod';
import {
  defineComponent,
  defineAction,
  action,
  createLibrary,
  useRenderNode,
  useTriggerAction,
} from '@openuidev/react-lang';
import type { ActionEvent } from '@openuidev/react-lang';

/* Base OpenUI component library for chat-rendered generative UI.
 * Styled with an explicit white background so generative blocks remain crisp,
 * clean, and perfectly legible across all themes (dark, light, or custom). */

/** The primary action a generated Button can run: send text back into the chat. */
export const sendPromptAction = defineAction({
  name: 'send_prompt',
  description: 'Send a prompt back into the chat as the next user message',
  params: z.object({
    prompt: z.string().describe('The text to send as the next user message'),
  }),
});

const toneClass = (tone?: string) => {
  switch (tone) {
    case 'success':
      return 'bg-emerald-50 text-emerald-700 border-emerald-300';
    case 'warning':
      return 'bg-amber-50 text-amber-700 border-amber-300';
    case 'danger':
    case 'error':
      return 'bg-rose-50 text-rose-700 border-rose-300';
    case 'info':
    case 'primary':
      return 'bg-blue-50 text-blue-700 border-blue-300';
    default:
      return 'bg-neutral-100 text-neutral-700 border-neutral-300';
  }
};

/** Renders an array (or single) of child element nodes produced by the parser. */
function useChildren(children?: unknown[] | unknown) {
  const renderNode = useRenderNode();
  if (!children) return null;
  const list = Array.isArray(children) ? children : [children];
  if (list.length === 0) return null;
  return list.map((child, i) => <React.Fragment key={i}>{renderNode(child)}</React.Fragment>);
}

const CardComponent: React.FC<{ props: { children?: unknown; title?: string; subtitle?: string } }> = ({ props }) => {
  const children = useChildren(props.children);
  return (
    <div className="rounded-xl border border-neutral-200 bg-white p-4 shadow-xs text-neutral-900 [color-scheme:light]">
      {props.title && <div className="text-sm font-semibold text-neutral-900">{props.title}</div>}
      {props.subtitle && <div className="mt-0.5 text-xs text-neutral-500">{props.subtitle}</div>}
      {children && <div className="mt-3 flex flex-col gap-2">{children}</div>}
    </div>
  );
};

export const Card = defineComponent({
  name: 'Card',
  description: 'A bordered surface grouping related UI. Call: Card([children], "Title", "Subtitle")',
  props: z.object({
    children: z.union([z.array(z.any()), z.any()]).optional().describe('Card content'),
    title: z.string().optional().describe('Card heading'),
    subtitle: z.string().optional().describe('Muted line under the heading'),
  }),
  component: CardComponent,
});

const StackComponent: React.FC<{ props: { children?: unknown; direction?: string; gap?: number } }> = ({ props }) => {
  const children = useChildren(props.children);
  const isRow = props.direction === 'horizontal' || props.direction === 'row';
  return (
    <div
      className={
        (isRow ? 'flex flex-row items-center flex-wrap ' : 'flex flex-col ') +
        'gap-2 text-neutral-900'
      }
    >
      {children}
    </div>
  );
};

export const Stack = defineComponent({
  name: 'Stack',
  description: 'Lays out children vertically or horizontally. Call: Stack([children], "vertical")',
  props: z.object({
    children: z.union([z.array(z.any()), z.any()]).optional().describe('Child elements'),
    direction: z.union([z.enum(['vertical', 'horizontal', 'row', 'column']), z.string()]).optional().describe('Layout direction'),
    gap: z.number().optional().describe('Gap between children'),
  }),
  component: StackComponent,
});

const TextComponent: React.FC<{ props: { content: string } }> = ({ props }) => (
  <p className="text-sm text-neutral-800 leading-relaxed">{props.content}</p>
);

export const Text = defineComponent({
  name: 'Text',
  description: 'A paragraph of text',
  props: z.object({
    content: z.string().describe('The text to show'),
  }),
  component: TextComponent,
});

const HeadingComponent: React.FC<{ props: { content: string; level?: number | string } }> = ({ props }) => {
  const lvl = typeof props.level === 'string' ? parseInt(props.level, 10) : props.level;
  const size = lvl === 1 ? 'text-base font-bold' : lvl === 3 ? 'text-xs font-semibold' : 'text-sm font-semibold';
  return <div className={`${size} text-neutral-900`}>{props.content}</div>;
};

export const Heading = defineComponent({
  name: 'Heading',
  description: 'A section heading',
  props: z.object({
    content: z.string().describe('Heading text'),
    level: z.union([z.literal(1), z.literal(2), z.literal(3), z.literal(4), z.literal(5), z.literal(6), z.number(), z.string()]).optional().describe('Heading level'),
  }),
  component: HeadingComponent,
});

const MetricComponent: React.FC<{ props: { label: string; value: string; trend?: string } }> = ({ props }) => {
  const isUp = props.trend === 'up' || props.trend === 'increase';
  const isDown = props.trend === 'down' || props.trend === 'decrease';
  const isNeutral = props.trend === 'flat' || props.trend === 'neutral' || props.trend === 'same';

  return (
    <div className="flex items-baseline gap-2">
      <span className="text-xs font-medium text-neutral-500">{props.label}</span>
      <span className="font-mono text-lg font-bold text-neutral-900">{props.value}</span>
      {props.trend && (
        <span
          className={`text-[10px] font-bold ${
            isUp ? 'text-emerald-600' : isDown ? 'text-rose-600' : 'text-neutral-400'
          }`}
        >
          {isUp ? '▲' : isDown ? '▼' : isNeutral ? '―' : props.trend}
        </span>
      )}
    </div>
  );
};

export const Metric = defineComponent({
  name: 'Metric',
  description: 'A KPI value with label and trend',
  props: z.object({
    label: z.string().describe('What is measured'),
    value: z.string().describe('The value'),
    trend: z.union([z.enum(['up', 'down', 'flat', 'neutral', 'same']), z.string()]).optional().describe('Direction of change'),
  }),
  component: MetricComponent,
});

const BadgeComponent: React.FC<{ props: { text: string; tone?: string } }> = ({ props }) => (
  <span
    className={`inline-flex w-fit items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${toneClass(
      props.tone
    )}`}
  >
    {props.text}
  </span>
);

export const Badge = defineComponent({
  name: 'Badge',
  description: 'A small status pill',
  props: z.object({
    text: z.string().describe('Badge label'),
    tone: z.union([z.enum(['default', 'success', 'warning', 'danger', 'error', 'info', 'primary', 'neutral']), z.string()]).optional().describe('Color tone'),
  }),
  component: BadgeComponent,
});

const ButtonComponent: React.FC<{ props: { label: string; action?: unknown } }> = ({ props }) => {
  const triggerAction = useTriggerAction();
  return (
    <button
      type="button"
      className="w-fit cursor-pointer rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs transition-colors hover:bg-blue-700 active:bg-blue-800"
      onClick={() => triggerAction(props.label, undefined, props.action as never)}
    >
      {props.label}
    </button>
  );
};

export const Button = defineComponent({
  name: 'Button',
  description: 'A clickable button that runs an action, e.g. action: @send_prompt("text")',
  props: z.object({
    label: z.string().describe('Button text'),
    action: action().optional().describe('The action to run on click'),
  }),
  component: ButtonComponent,
});

const TableComponent: React.FC<{ props: { headers: unknown[]; rows: unknown[][] } }> = ({ props }) => (
  <div className="my-1 overflow-x-auto rounded-lg border border-neutral-200">
    <table className="w-full text-left text-xs">
      <thead className="bg-neutral-50 text-neutral-600">
        <tr>
          {props.headers.map((h: unknown, i: number) => (
            <th key={i} className="px-3 py-2 font-semibold border-b border-neutral-200">{String(h ?? '')}</th>
          ))}
        </tr>
      </thead>
      <tbody className="divide-y divide-neutral-200 bg-white">
        {props.rows.map((row: unknown[], r: number) => (
          <tr key={r} className="hover:bg-neutral-50/70">
            {row.map((cell: unknown, c: number) => (
              <td key={c} className="px-3 py-1.5 text-neutral-800">{String(cell ?? '')}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  </div>
);

export const Table = defineComponent({
  name: 'Table',
  description: 'A simple data table',
  props: z.object({
    headers: z.array(z.any()).describe('Column headers'),
    rows: z.array(z.array(z.any())).describe('Rows of cell text'),
  }),
  component: TableComponent,
});

const ListComponent: React.FC<{ props: { items: unknown[] } }> = ({ props }) => (
  <ul className="my-1 list-disc space-y-1 pl-5 text-sm text-neutral-800 marker:text-neutral-400">
    {props.items.map((it: unknown, i: number) => (
      <li key={i}>{String(it ?? '')}</li>
    ))}
  </ul>
);

export const List = defineComponent({
  name: 'List',
  description: 'A bulleted list',
  props: z.object({
    items: z.array(z.any()).describe('List items'),
  }),
  component: ListComponent,
});

const ImageComponent: React.FC<{ props: { url: string; alt?: string } }> = ({ props }) => (
  <img
    src={props.url}
    alt={props.alt || ''}
    className="my-1 max-h-64 rounded-lg border border-neutral-200 object-cover"
    loading="lazy"
  />
);

export const Image = defineComponent({
  name: 'Image',
  description: 'An image',
  props: z.object({
    url: z.string().describe('Image URL'),
    alt: z.string().optional().describe('Alt text'),
  }),
  component: ImageComponent,
});

const DividerComponent: React.FC<{ props: Record<string, never> }> = () => (
  <hr className="my-2 border-t border-neutral-200" />
);

export const Divider = defineComponent({
  name: 'Divider',
  description: 'A horizontal separator line',
  props: z.object({}),
  component: DividerComponent,
});

/** The library the chat renderer accepts. `Card` is the root component. */
export const opuiLibrary = createLibrary({
  components: [Card, Stack, Text, Heading, Metric, Badge, Button, Table, List, Image, Divider],
  actions: [sendPromptAction],
  root: 'Card',
});

/** Map a Renderer ActionEvent onto this app: send chat prompts, open URLs. */
export function handleOpenUIAction(event: ActionEvent, sendPrompt: (text: string) => void) {
  if (event.type === 'open_url') {
    const url = (event.params as { url?: string }).url;
    if (url) window.open(url, '_blank', 'noopener');
    return;
  }
  if (event.type === 'send_prompt') {
    const prompt = (event.params as { prompt?: string }).prompt || event.humanFriendlyMessage;
    if (prompt) sendPrompt(prompt);
    return;
  }
  if (event.type === 'continue_conversation') {
    const text = event.humanFriendlyMessage || ((event.params as { context?: string }).context ?? '');
    if (text) sendPrompt(text);
  }
}

export default opuiLibrary;
