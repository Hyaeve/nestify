export type ThemeMode = 'light' | 'dark'

const THEME_STORAGE_KEY = 'nestify-theme'

export function getStoredTheme(): ThemeMode {
  if (typeof window === 'undefined') {
    return 'light'
  }

  const stored = window.localStorage.getItem(THEME_STORAGE_KEY)
  if (stored === 'dark' || stored === 'appletv') {
    if (stored === 'appletv') {
      window.localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    }
    return 'dark'
  }
  return 'light'
}

export function applyTheme(theme: ThemeMode | 'appletv') {
  if (typeof document === 'undefined') {
    return
  }

  document.documentElement.setAttribute('data-theme', theme === 'appletv' ? 'dark' : theme)
}

export function setTheme(theme: ThemeMode) {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme)
  }

  applyTheme(theme)
}
