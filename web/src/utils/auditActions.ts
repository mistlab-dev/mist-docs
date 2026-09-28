// Audit actions the API writes today (see handler audit(c, ...) calls), in
// the order the filter shows them. Keep in sync with the backend; the
// vitest checks every action has a label in both languages.
export const AUDIT_ACTIONS = [
  'create_doc', 'edit_doc', 'delete_doc', 'purge_doc', 'restore_doc', 'move', 'view', 'export', 'import_doc',
  'create_share', 'delete_share', 'create_comment', 'delete_comment',
  'add_collaborator', 'update_collaborator', 'remove_collaborator', 'set_permission', 'remove_permission',
  'lock_doc', 'unlock_doc', 'create_folder', 'delete_folder', 'delete_media',
] as const

// Names found in older rows; shown with the label of the current action.
export const LEGACY_ACTIONS: Record<string, string> = {
  restore: 'restore_doc', share: 'create_share', comment: 'create_comment', edit: 'edit_doc', delete: 'delete_doc',
  create: 'create_doc', access_share: 'view',
}

type TagType = 'primary' | 'success' | 'warning' | 'danger' | 'info' | ''
export function auditActionColor(action: string): TagType {
  const a = LEGACY_ACTIONS[action] || action
  if (/^(delete|purge|remove)/.test(a)) return 'danger'
  if (/^(create|import|add)/.test(a)) return 'primary'
  if (/^(restore|unlock)/.test(a)) return 'success'
  if (/^(edit|update|set|lock|move)/.test(a)) return 'warning'
  return 'info'
}

export function auditActionKey(action: string): string {
  return `admin.audits.act.${LEGACY_ACTIONS[action] || action}`
}
