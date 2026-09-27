// Server timestamps arrive as RFC 3339 strings with a zone ("…Z" or "+08:00").
// Show them in the viewer's own time zone.

type Translate = (key: string, args?: unknown[]) => string

export function parseTime(ts: string | number | Date | null | undefined): Date | null {
  if (ts === null || ts === undefined || ts === '') return null
  const d = ts instanceof Date ? ts : new Date(ts)
  return Number.isNaN(d.getTime()) ? null : d
}

/** "2026-09-27 22:54:03" in the given (default: browser) time zone. */
export function formatDateTime(ts: string | number | Date | null | undefined, opts: { timeZone?: string; seconds?: boolean } = {}): string {
  const d = parseTime(ts)
  if (!d) return typeof ts === 'string' ? ts : ''
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: opts.timeZone,
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(d)
  const get = (type: string) => parts.find(p => p.type === type)?.value ?? ''
  const time = `${get('hour')}:${get('minute')}` + (opts.seconds === false ? '' : `:${get('second')}`)
  return `${get('year')}-${get('month')}-${get('day')} ${time}`
}

/** 刚刚 / N 分钟前 / N 小时前 / N 天前, then the local date. */
export function formatRelative(
  ts: string | number | Date | null | undefined,
  t: Translate,
  opts: { now?: number; locale?: string; timeZone?: string } = {},
): string {
  const d = parseTime(ts)
  if (!d) return ''
  const diff = (opts.now ?? Date.now()) - d.getTime()
  if (diff < 60_000) return t('common.justNow')
  if (diff < 3_600_000) return t('common.minutesAgo', [Math.floor(diff / 60_000)])
  if (diff < 86_400_000) return t('common.hoursAgo', [Math.floor(diff / 3_600_000)])
  if (diff < 604_800_000) return t('common.daysAgo', [Math.floor(diff / 86_400_000)])
  return d.toLocaleDateString(opts.locale, { timeZone: opts.timeZone })
}
