// The collection tab's query editor shows the query as one mongosh
// statement, as Studio 3T does: db.getCollection("orders").find({ … }).sort({ … })

export interface QueryParts {
  filter: string
  projection: string
  sort: string
  skip: number
  limit: number
}

const blank = (s: string) => !s.trim() || /^\{\s*\}$/.test(s.trim())

/** Renders query parts as db.getCollection("coll").find(filter, projection).sort(…).skip(n).limit(n). */
export function buildQueryText(coll: string, q: QueryParts): string {
  const filter = blank(q.filter) ? '{}' : q.filter.trim()
  const args = blank(q.projection) ? filter : `${filter}, ${q.projection.trim()}`
  let text = `db.getCollection(${JSON.stringify(coll)}).find(${args})`
  if (!blank(q.sort)) text += `\n  .sort(${q.sort.trim()})`
  if (q.skip > 0) text += `\n  .skip(${q.skip})`
  if (q.limit > 0) text += `\n  .limit(${q.limit})`
  return text
}
