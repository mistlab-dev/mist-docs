// Types and helpers for GET /deadlines/:id/explain (rule-based explainer,
// internal/schedule/explain.go).

export type Verdict = 'done' | 'overdue' | 'late' | 'tight' | 'on_track' | 'started' | 'no_signal'
export type Confidence = 'high' | 'medium' | 'low'
export type EvidenceType = 'event' | 'load' | 'history' | 'plan' | 'progress' | 'flag'

export interface EvidenceItem {
  type: EvidenceType
  text: string
  ref?: string
}

export interface SuggestedPreview {
  deadline_id: string
  priority: string
  due_date: string
}

export interface Explanation {
  deadline_id: string
  order_no: string
  verdict: Verdict
  conclusion: string
  confidence: Confidence
  due_date: string
  planned_finish?: string
  late_days: number
  evidence: EvidenceItem[]
  missing: string[]
  assumptions: string[]
  suggestion: string
  preview?: SuggestedPreview
}

/** Element Plus tag type for the verdict. */
export function verdictType(v: Verdict): 'danger' | 'warning' | 'success' | 'info' {
  switch (v) {
    case 'overdue':
    case 'late':
      return 'danger'
    case 'tight':
    case 'started':
      return 'warning'
    case 'done':
    case 'on_track':
      return 'success'
    default:
      return 'info'
  }
}

const confKeys: Record<Confidence, string> = { high: 'explain.confHigh', medium: 'explain.confMedium', low: 'explain.confLow' }
export function confidenceKey(c: string): string {
  return confKeys[c as Confidence] || 'explain.confLow'
}

const evKeys: Record<EvidenceType, string> = {
  event: 'explain.evEvent', load: 'explain.evLoad', history: 'explain.evHistory',
  plan: 'explain.evPlan', progress: 'explain.evProgress', flag: 'explain.evFlag',
}
export function evidenceKey(t: string): string {
  return evKeys[t as EvidenceType] || 'explain.evHistory'
}

/** Whether a "why?" shortcut is worth showing on a board card. */
export function worthExplaining(riskLevel: string, status: string): boolean {
  return status !== 'done' && riskLevel !== 'ok'
}

/** True when the explanation admits it lacks data (conclusion starts with 数据不足). */
export function isInsufficient(ex: Pick<Explanation, 'conclusion'>): boolean {
  return ex.conclusion.startsWith('数据不足')
}
