import type { Stage } from './types'

export interface EditorStage extends Stage {
  key: string
}

export const STAGE_TEMPLATES: { operator: string; body: string; hint: string }[] = [
  { operator: '$match', body: '{\n  \n}', hint: 'Filter documents' },
  { operator: '$project', body: '{\n  _id: 1\n}', hint: 'Include, exclude or compute fields' },
  { operator: '$addFields', body: '{\n  newField: "$existing"\n}', hint: 'Add computed fields' },
  { operator: '$set', body: '{\n  field: "value"\n}', hint: 'Alias of $addFields' },
  { operator: '$unset', body: '"field"', hint: 'Remove fields' },
  { operator: '$group', body: '{\n  _id: "$field",\n  count: { $sum: 1 }\n}', hint: 'Group and accumulate' },
  { operator: '$sort', body: '{\n  field: -1\n}', hint: 'Order documents' },
  { operator: '$limit', body: '10', hint: 'Keep the first n documents' },
  { operator: '$skip', body: '10', hint: 'Skip n documents' },
  { operator: '$unwind', body: '"$arrayField"', hint: 'One document per array element' },
  {
    operator: '$lookup',
    body: '{\n  from: "otherCollection",\n  localField: "field",\n  foreignField: "_id",\n  as: "joined"\n}',
    hint: 'Join another collection',
  },
  { operator: '$count', body: '"count"', hint: 'Count documents' },
  { operator: '$sample', body: '{\n  size: 10\n}', hint: 'Random sample' },
  { operator: '$facet', body: '{\n  facetName: [\n    { $limit: 5 }\n  ]\n}', hint: 'Multiple sub-pipelines' },
  { operator: '$bucket', body: '{\n  groupBy: "$field",\n  boundaries: [0, 10, 100],\n  default: "other"\n}', hint: 'Bucket by ranges' },
  { operator: '$replaceRoot', body: '{\n  newRoot: "$subDocument"\n}', hint: 'Promote a sub-document' },
  { operator: '$sortByCount', body: '"$field"', hint: 'Group, count and sort' },
  {
    operator: '$search',
    body: '{\n  index: "default",\n  text: { query: "words to find", path: "title" }\n}',
    hint: 'Atlas Search full-text query (must be first; needs a search index)',
  },
  {
    operator: '$vectorSearch',
    body: '{\n  index: "vector_index",\n  path: "embedding",\n  queryVector: [0.1, 0.2, 0.3],\n  numCandidates: 100,\n  limit: 10\n}',
    hint: 'Atlas Vector Search: nearest neighbours (must be first; needs a vector index)',
  },
  { operator: '$out', body: '"outputCollection"', hint: 'Write results to a collection (must be last)' },
  { operator: '$merge', body: '{\n  into: "outputCollection"\n}', hint: 'Merge results into a collection (must be last)' },
]

export function templateFor(operator: string): string {
  return STAGE_TEMPLATES.find((t) => t.operator === operator)?.body ?? '{\n  \n}'
}

let n = 0
export function stageKey(): string {
  n += 1
  return `s${n}`
}

export function newStage(operator = '$match'): EditorStage {
  return { key: stageKey(), operator, body: templateFor(operator), enabled: true }
}

export type PipelineAction =
  | { type: 'add'; operator?: string; after?: number }
  | { type: 'remove'; index: number }
  | { type: 'move'; from: number; to: number }
  | { type: 'toggle'; index: number }
  | { type: 'update'; index: number; patch: Partial<Stage> }
  | { type: 'duplicate'; index: number }
  | { type: 'replace'; stages: Stage[] }

export function pipelineReducer(state: EditorStage[], action: PipelineAction): EditorStage[] {
  switch (action.type) {
    case 'add': {
      const s = newStage(action.operator)
      const at = action.after === undefined ? state.length : action.after + 1
      return [...state.slice(0, at), s, ...state.slice(at)]
    }
    case 'remove':
      return state.filter((_, i) => i !== action.index)
    case 'move': {
      const { from, to } = action
      if (from === to || from < 0 || to < 0 || from >= state.length || to >= state.length) return state
      const next = [...state]
      const [item] = next.splice(from, 1)
      next.splice(to, 0, item)
      return next
    }
    case 'toggle':
      return state.map((s, i) => (i === action.index ? { ...s, enabled: !s.enabled } : s))
    case 'update':
      return state.map((s, i) => {
        if (i !== action.index) return s
        const next = { ...s, ...action.patch }
        // Switching operator on an untouched template swaps in the new template.
        if (action.patch.operator && action.patch.body === undefined && s.body === templateFor(s.operator)) {
          next.body = templateFor(action.patch.operator)
        }
        return next
      })
    case 'duplicate': {
      const src = state[action.index]
      if (!src) return state
      const copy = { ...src, key: stageKey() }
      return [...state.slice(0, action.index + 1), copy, ...state.slice(action.index + 1)]
    }
    case 'replace':
      return action.stages.map((s) => ({ ...s, key: stageKey() }))
  }
}

export function toStages(state: EditorStage[]): Stage[] {
  return state.map(({ operator, body, enabled }) => ({ operator, body, enabled }))
}
