// Team roles come from the Portal. Mirrors internal/handler/roles.go so the
// UI hides exactly what the API would refuse (D2, D4).
export type TeamRole = 'owner' | 'admin' | 'editor' | 'member' | 'viewer' | string

export function roleRank(role: TeamRole | undefined | null): number {
  switch (role) {
    case 'owner':
    case 'admin':
      return 3
    case 'editor':
    case 'member':
      return 2
    case 'viewer':
      return 1
    default:
      return 0
  }
}

/** editor, admin or owner */
export const canEditRole = (role?: TeamRole | null) => roleRank(role) >= 2
/** admin or owner */
export const isAdminRole = (role?: TeamRole | null) => roleRank(role) >= 3

/** The record's creator(s) or a team admin may manage it. */
export function canManageRecord(role: TeamRole | undefined | null, userId: string | undefined, ...owners: (string | undefined | null)[]) {
  if (isAdminRole(role)) return true
  return !!userId && canEditRole(role) && owners.some(o => !!o && o === userId)
}
