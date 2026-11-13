import React from 'react';

// Lane-based SVG graph cell — renders one commit row's `git log --graph`
// prefix (e.g. "* ", "| ", "|\ ", "|/ ") as colored branch lanes with a
// commit dot, mimicking VS Code's Git Graph extension. Colors come from the
// --graph-c0..c9 palette defined in index.css (light + dark variants).
const LANE_W = 14;
const ROW_H = 22;
const PALETTE_LEN = 10;

const laneColor = (lane: number) => `var(--graph-c${lane % PALETTE_LEN})`;

function commitDot(x: number, y: number, color: string, isMerge: boolean, isHead: boolean, key?: number) {
  return (
    <g key={key}>
      {/* halo ring */}
      <circle cx={x} cy={y} r={isMerge ? 5 : 4.4} fill={color} fillOpacity={0.22} />
      {/* main dot */}
      <circle cx={x} cy={y} r={isMerge ? 3.2 : 2.8} fill={color} stroke="rgba(0,0,0,0.5)" strokeWidth={0.75} />
      {/* inner highlight */}
      <circle cx={x} cy={y} r={1.1} fill={isHead ? '#fff' : 'rgba(255,255,255,0.7)'} />
    </g>
  );
}

export function GitGraphLane({ prefix, isMerge, isHead }: { prefix?: string; isMerge?: boolean; isHead?: boolean }) {
  const lines: React.ReactNode[] = [];
  const dots: { x: number; y: number; color: string; r: number; merge: boolean; head: boolean }[] = [];

  const commit = (lane: number) =>
    dots.push({ x: lane * LANE_W + LANE_W / 2, y: ROW_H / 2, color: laneColor(lane), r: 0, merge: !!isMerge && lane === 0, head: !!isHead });

  if (!prefix) {
    // No graph prefix available (e.g. backend returned a plain list) — draw a
    // single root-lane commit dot so the tree still reads as one line.
    commit(0);
  } else {
    const lanes = Math.ceil(prefix.length / 2);
    const cx = (lane: number) => lane * LANE_W + LANE_W / 2;

    for (let L = 0; L < lanes; L++) {
      const c0 = L * 2 < prefix.length ? prefix[L * 2] : ' ';
      const c1 = L * 2 + 1 < prefix.length ? prefix[L * 2 + 1] : ' ';
      const col = laneColor(L);

      if (c0 === '|' || c1 === '|') {
        lines.push(<line key={`v${L}`} x1={cx(L)} y1={0} x2={cx(L)} y2={ROW_H} stroke={col} strokeWidth={1.5} />);
      }
      if (c0 === '\\' || c1 === '\\') {
        // Branch line leaving this lane down-right into the next lane.
        lines.push(<line key={`br${L}`} x1={cx(L)} y1={0} x2={cx(L + 1)} y2={ROW_H} stroke={col} strokeWidth={1.5} />);
      }
      if (c0 === '/' || c1 === '/') {
        // Line from the right lane converging down-left; keeps its own lane color.
        lines.push(<line key={`bl${L}`} x1={cx(L + 1)} y1={0} x2={cx(L)} y2={ROW_H} stroke={laneColor(L + 1)} strokeWidth={1.5} />);
      }
      if (c0 === '_' || c1 === '_') {
        // Horizontal jog git draws before a diagonal across several lanes.
        lines.push(<line key={`h${L}`} x1={cx(L)} y1={1.5} x2={cx(L) + LANE_W} y2={1.5} stroke={col} strokeWidth={1.5} />);
      }
      if (c0 === '-' || c1 === '-') {
        // Merge commit horizontal connector between lanes.
        lines.push(<line key={`m${L}`} x1={cx(L)} y1={ROW_H / 2} x2={cx(L) + LANE_W} y2={ROW_H / 2} stroke={col} strokeWidth={1.5} />);
      }

      if (c0 === '*' || c1 === '*' || c0 === '@' || c1 === '@') {
        commit(L);
      }

      // Junction dot where a diagonal meets a vertical (merge indicator).
      const hasVert = c0 === '|' || c1 === '|';
      const hasDiag = c0 === '/' || c1 === '/' || c0 === '\\' || c1 === '\\';
      if (hasVert && hasDiag) {
        const y = c0 === '/' || c1 === '/' ? ROW_H : 0;
        dots.push({ x: cx(L), y, color: col, r: 2, merge: false, head: false });
      }
    }
  }

  const width = Math.max(LANE_W, Math.ceil((prefix || ' ').length / 2) * LANE_W);

  return (
    <svg width={width} height={ROW_H} className="block shrink-0 select-none mt-0.5">
      {lines}
      {dots.map((d, i) => commitDot(d.x, d.y, d.color, d.merge, d.head, i))}
    </svg>
  );
}
