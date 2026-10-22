import { useMemo, useState } from 'react'
import { formatCount } from '../lib/format'
import type { ExplainStage } from '../lib/types'

const W = 200
const H = 76
const GAP_X = 22
const GAP_Y = 36
const PAD = 8

interface Placed {
  stage: ExplainStage
  x: number
  y: number
  id: string
  parent?: Placed
}

/** Lays the stage tree out top-down: leaves left to right, each parent
 * centred above its children. */
function layout(root: ExplainStage): { nodes: Placed[]; width: number; height: number } {
  const nodes: Placed[] = []
  let nextLeaf = 0
  let maxDepth = 0
  const place = (s: ExplainStage, depth: number, id: string, parent?: Placed): Placed => {
    const node: Placed = { stage: s, x: 0, y: PAD + depth * (H + GAP_Y), id, parent }
    nodes.push(node)
    maxDepth = Math.max(maxDepth, depth)
    if (s.children.length === 0) {
      node.x = PAD + nextLeaf * (W + GAP_X)
      nextLeaf++
    } else {
      const kids = s.children.map((c, i) => place(c, depth + 1, `${id}.${i}`, node))
      node.x = (kids[0]!.x + kids[kids.length - 1]!.x) / 2
    }
    return node
  }
  place(root, 0, '0')
  return { nodes, width: PAD * 2 + nextLeaf * (W + GAP_X) - GAP_X, height: PAD * 2 + (maxDepth + 1) * (H + GAP_Y) - GAP_Y }
}

export function stageTone(name: string): 'bad' | 'warn' | 'good' | '' {
  if (name === 'COLLSCAN') return 'bad'
  if (name === 'SORT') return 'warn'
  if (name.includes('IXSCAN') || name === 'IDHACK' || name === 'PROJECTION_COVERED') return 'good'
  return ''
}

/** The winning plan as an SVG tree: one box per stage with documents and keys
 * examined and time; collection scans are red, in-memory sorts amber and index
 * stages green. Click a stage for its details. */
export function PlanTree({ root, label }: { root: ExplainStage; label?: string }) {
  const { nodes, width, height } = useMemo(() => layout(root), [root])
  const [selected, setSelected] = useState<string | null>(null)
  const sel = nodes.find((n) => n.id === selected)
  return (
    <div className="plan-graph">
      <div className="plan-graph-scroll">
        <svg width={width} height={height} role="img" aria-label={label ?? 'Winning plan'}>
          {nodes
            .filter((n) => n.parent)
            .map((n) => (
              <line key={`e${n.id}`} className="plan-edge" x1={n.parent!.x + W / 2} y1={n.parent!.y + H} x2={n.x + W / 2} y2={n.y} />
            ))}
          {nodes.map((n) => {
            const s = n.stage
            const tone = stageTone(s.name)
            const line2 = [s.docsExamined !== undefined && `docs ${formatCount(s.docsExamined)}`, s.keysExamined !== undefined && `keys ${formatCount(s.keysExamined)}`]
              .filter(Boolean)
              .join(' · ')
            const line3 = [s.nReturned !== undefined && `${formatCount(s.nReturned)} out`, s.timeMs !== undefined && `~${s.timeMs} ms`].filter(Boolean).join(' · ')
            return (
              <g
                key={n.id}
                className={`plan-node ${tone ? `tone-${tone}` : ''} ${selected === n.id ? 'selected' : ''}`}
                transform={`translate(${n.x},${n.y})`}
                onClick={() => setSelected(selected === n.id ? null : n.id)}
                role="button"
                aria-label={`Stage ${s.name}`}
              >
                <rect width={W} height={H} rx={6} />
                <text x={10} y={22} className="plan-node-name">
                  {s.name}
                </text>
                {line2 && (
                  <text x={10} y={44} className="plan-node-stat">
                    {line2}
                  </text>
                )}
                {line3 && (
                  <text x={10} y={64} className="plan-node-stat">
                    {line3}
                  </text>
                )}
              </g>
            )
          })}
        </svg>
      </div>
      {sel && sel.stage.details.length > 0 && (
        <dl className="plan-details plan-graph-details">
          {sel.stage.details.map((d) => (
            <div key={d.key}>
              <dt>{d.key}</dt>
              <dd className="mono">{d.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  )
}

/** Stage names in a tree, depth-first. */
export function stageNames(s: ExplainStage | null | undefined): string[] {
  if (!s) return []
  return [s.name, ...s.children.flatMap(stageNames)]
}
