// Local draft fallback for DocEditor.
//
// Yjs keeps offline edits only in memory (MistWSProvider.pendingLocalUpdates);
// a refresh or crash during disconnect loses them. This store persists a
// debounced snapshot of the editor HTML in localStorage so a newer draft can
// be offered for recovery on the next open. Recovery goes through the normal
// collaborative channel (editor.setContent inside the synced Y.Doc), so it
// merges with other users' edits instead of overwriting them.
//
// Storage strategy: one key per document, `mist-docs-draft:{doc_id}`. The
// record carries the server version/updated_at it was based on plus the time
// the draft was written, so loadDoc can tell "draft newer than server" from
// "stale draft from an old visit" and silently drop the latter.

const PREFIX = 'mist-docs-draft:'

export interface DraftRecord {
  /** Editor HTML snapshot at the time of writing. */
  content: string
  /** Server version the draft is based on (0 when unknown). */
  baseVersion: number
  /** Server updated_at the draft is based on ('' when unknown). */
  baseUpdatedAt: string
  /** Epoch ms when the draft was written. */
  savedAt: number
  /** Optional title snapshot; the title saves through its own endpoint. */
  title?: string
}

// A draft older than this is considered abandoned (server has almost
// certainly been edited since) and is dropped instead of offered.
export const DRAFT_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000

function storage(): Storage | null {
  try {
    // Accessing localStorage can throw (private mode, disabled storage).
    return window.localStorage
  } catch {
    return null
  }
}

export function draftKey(docId: string): string {
  return PREFIX + docId
}

export function saveDraft(docId: string, record: DraftRecord): boolean {
  const ls = storage()
  if (!ls || !docId) return false
  try {
    ls.setItem(draftKey(docId), JSON.stringify(record))
    return true
  } catch {
    // Quota exceeded: drop older drafts of other docs, then retry once.
    try {
      pruneAllDraftsExcept(docId)
      ls.setItem(draftKey(docId), JSON.stringify(record))
      return true
    } catch {
      return false
    }
  }
}

export function readDraft(docId: string): DraftRecord | null {
  const ls = storage()
  if (!ls || !docId) return null
  try {
    const raw = ls.getItem(draftKey(docId))
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed.content !== 'string' || !parsed.content) return null
    if (typeof parsed.savedAt !== 'number') return null
    if (Date.now() - parsed.savedAt > DRAFT_MAX_AGE_MS) {
      // Too old to trust; clean up so it never reappears.
      removeDraft(docId)
      return null
    }
    return parsed as DraftRecord
  } catch {
    return null
  }
}

export function removeDraft(docId: string): void {
  const ls = storage()
  if (!ls) return
  try {
    ls.removeItem(draftKey(docId))
  } catch {}
}

// parseServerTime converts the API's updated_at into epoch ms.
//
// The MySQL DSN runs with parseTime=true&loc=Local, so timestamps arrive as
// "YYYY-MM-DD HH:MM:SS" with NO offset — the server's local wall clock.
// Blindly appending "Z" reinterprets that wall clock as UTC and skews the
// comparison by the deployment's UTC offset (8h in Asia/Shanghai), which made
// fresh offline drafts look stale and never be offered for recovery.
// Values that already carry an offset (Z or ±HH:MM) are parsed as given;
// offset-less values are parsed as local time, matching how the server wrote them.
export function parseServerTime(value: string): number {
  const s = value.trim()
  if (!s) return NaN
  if (/(?:Z|[+-]\d{2}:?\d{2})$/.test(s)) return Date.parse(s)
  // Normalize the space separator to "T" so ES date-time forms apply
  // (offset-less date-times are local time per spec).
  return Date.parse(s.replace(' ', 'T'))
}

// isDraftRecoverable reports whether the draft should be offered for recovery
// against the freshly loaded server document.
//
// A draft is recoverable when its content differs from the server content and
// it was written at/after the base snapshot (draft is genuinely newer than
// what the server has). When baseUpdatedAt is missing we only compare content
// and rely on the max-age guard.
export function isDraftRecoverable(
  draft: DraftRecord,
  server: { content: string; version?: number; updated_at?: string },
): boolean {
  if (!draft.content || draft.content === server.content) return false
  if (draft.baseUpdatedAt && server.updated_at && draft.baseUpdatedAt !== server.updated_at) {
    // Server has moved past the draft's base AND the draft was made after
    // that base — both sides changed; still offer recovery but the user
    // decides. Server-newer-only cases fall through to savedAt comparison.
    if (draft.baseUpdatedAt > server.updated_at) return true
  }
  if (server.updated_at && draft.savedAt) {
    // Compare against the server timestamp only when we can parse it; a draft
    // saved before the server's last write is stale.
    const serverMs = parseServerTime(server.updated_at)
    if (!Number.isNaN(serverMs) && draft.savedAt < serverMs) return false
  }
  return true
}

// pruneAllDraftsExcept removes every stored draft except keepId — used when
// localStorage is full.
function pruneAllDraftsExcept(keepId: string): void {
  const ls = storage()
  if (!ls) return
  const doomed: string[] = []
  for (let i = 0; i < ls.length; i++) {
    const key = ls.key(i)
    if (key && key.startsWith(PREFIX) && key !== draftKey(keepId)) doomed.push(key)
  }
  for (const key of doomed) {
    try {
      ls.removeItem(key)
    } catch {}
  }
}
