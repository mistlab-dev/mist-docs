import { ref } from 'vue'

/** Persisted in localStorage. Keep in sync with the inline script in index.html. */
export type ThemeMode = 'light' | 'dark' | 'system'

export const THEME_STORAGE_KEY = 'mistdocs-theme'

const mode = ref<ThemeMode>('system')
const isDark = ref(false)
let media: MediaQueryList | null = null
let bound = false

export function readThemeMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  } catch {
    /* private browsing or non-browser test */
  }
  return 'system'
}

export function prefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveDark(theme: ThemeMode): boolean {
  if (theme === 'dark') return true
  if (theme === 'light') return false
  return prefersDark()
}

/** Toggle `html.dark` and `color-scheme`. Returns whether dark is active. */
export function applyTheme(theme: ThemeMode): boolean {
  const dark = resolveDark(theme)
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  return dark
}

function onSystemChange() {
  if (mode.value === 'system') isDark.value = applyTheme('system')
}

/** Apply the saved theme and follow OS changes while mode is `system`. */
export function bindTheme() {
  mode.value = readThemeMode()
  isDark.value = applyTheme(mode.value)
  if (bound) return
  media = window.matchMedia('(prefers-color-scheme: dark)')
  media.addEventListener('change', onSystemChange)
  bound = true
}

export function setThemeMode(next: ThemeMode) {
  mode.value = next
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next)
  } catch {
    /* ignore quota / private mode */
  }
  isDark.value = applyTheme(next)
}

export function useTheme() {
  bindTheme()
  return { mode, isDark, setThemeMode }
}

export { mode as themeMode, isDark }
