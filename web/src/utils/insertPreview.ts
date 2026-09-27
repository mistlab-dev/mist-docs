// Types and small helpers for the deadline insert preview
// (POST /deadlines/preview-insert, computed by internal/schedule).

export interface PreviewItem {
  id: string
  order_no: string
  title: string
  customer?: string
  priority: string
  due_date: string
  old_finish?: string
  new_finish: string
  delay_days: number
  late_days: number
  outcome: 'new_breach' | 'delayed' | 'unaffected' | 'started'
  flags?: string[]
}

export interface PreviewResult {
  insert: PreviewItem
  new_breaches: PreviewItem[]
  delayed: PreviewItem[]
  unaffected: PreviewItem[]
  started: PreviewItem[]
  conclusion: 'breach' | 'delay' | 'none'
  per_day: number
  proposal_id?: string
  changes?: ProposalChange[]
}

/** One write that confirming the proposal makes (stored server-side). */
export interface ProposalChange {
  deadline_id?: string
  order_no: string
  title?: string
  field: '__create__' | 'due_date' | 'priority'
  old?: string
  new?: string
}

/** i18n key for a change line. */
export function changeKey(field: ProposalChange['field']): string {
  return field === '__create__' ? 'insertPreview.chCreate' : field === 'priority' ? 'insertPreview.chPriority' : 'insertPreview.chDue'
}

/** Status tag type for the proposal history. */
export function proposalStatusType(s: string): 'warning' | 'success' | 'info' | 'danger' {
  return s === 'pending' ? 'warning' : s === 'applied' ? 'success' : s === 'stale' ? 'danger' : 'info'
}

/** True when the API answered 409 because the preview is out of date. */
export function isStaleError(e: any): boolean {
  return e?.response?.status === 409
}

/** Tag type for the conclusion banner. */
export function conclusionType(c: PreviewResult['conclusion']): 'danger' | 'warning' | 'success' {
  return c === 'breach' ? 'danger' : c === 'delay' ? 'warning' : 'success'
}

/** i18n key + args for the one-line conclusion. */
export function conclusionText(r: PreviewResult): { key: string; args: Record<string, number> } {
  const breach = r.new_breaches.length
  const delay = r.delayed.length
  if (breach && delay) return { key: 'insertPreview.sumBreachDelay', args: { breach, delay } }
  if (breach) return { key: 'insertPreview.sumBreach', args: { breach } }
  if (delay) return { key: 'insertPreview.sumDelay', args: { delay } }
  return { key: 'insertPreview.sumNone', args: {} }
}

/** "09-28" from "2026-09-28"; keeps the year when it differs from `thisYear`. */
export function shortDate(iso: string | undefined, thisYear = new Date().getFullYear()): string {
  if (!iso) return '—'
  const [y, m, d] = iso.split('-')
  if (!y || !m || !d) return iso
  return Number(y) === thisYear ? `${m}-${d}` : `${y}-${m}-${d}`
}

/** Flag → i18n key. Unknown flags are dropped. */
export function flagKey(flag: string): string | null {
  switch (flag) {
    case 'penalty': return 'insertPreview.flagPenalty'
    case 'key_customer': return 'insertPreview.flagKeyCustomer'
    case 'already_late': return 'insertPreview.flagAlreadyLate'
    default: return null
  }
}
