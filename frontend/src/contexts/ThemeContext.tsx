import React, { createContext, useContext, useEffect, useState } from 'react';
import { themes, type ThemeName } from '../lib/theme/themes';
import { useAuth } from './AuthContext';
import { useUserPreferences, useUpdateUserPreferences } from '../hooks/useUserPreferences';

/**
 * Theme mode can be a specific theme name or 'system' to auto-detect
 */
type ThemeMode = ThemeName | 'system';

/**
 * Theme context value
 */
interface ThemeContextType {
  /** Current theme setting ('light', 'dark', or 'system') */
  theme: ThemeMode;

  /** Set the theme */
  setTheme: (theme: ThemeMode) => void;

  /** The actual theme being used (resolved from 'system' if needed) */
  resolvedTheme: ThemeName;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

/**
 * ThemeProvider - Manages theme state and applies CSS variables
 *
 * Features:
 * - Persists theme preference to localStorage
 * - Detects system theme preference (prefers-color-scheme)
 * - Applies CSS variables to document root
 * - Maintains backwards compatibility with existing dark: classes
 *
 * @example
 * // Wrap your app
 * <ThemeProvider>
 *   <App />
 * </ThemeProvider>
 *
 * // Use in components
 * function MyComponent() {
 *   const { theme, setTheme, resolvedTheme } = useTheme();
 *   return <button onClick={() => setTheme('dark')}>Dark Mode</button>
 * }
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // Get saved theme from localStorage, default to 'system'. This applies
  // instantly (including pre-auth), then gets overridden below by the
  // server-stored preference once it's fetched, so theme stays in sync
  // across devices for a signed-in user.
  const [theme, setThemeState] = useState<ThemeMode>(() => {
    const saved = localStorage.getItem('app-theme') as ThemeMode;
    return saved || 'system';
  });

  const { isAuthenticated } = useAuth();
  const { data: serverPreferences } = useUserPreferences({ enabled: isAuthenticated });
  const updatePreferences = useUpdateUserPreferences();

  // Sync from the server preference once it's loaded for a signed-in user.
  // The server stores 'system' as 'auto'.
  useEffect(() => {
    if (isAuthenticated && serverPreferences?.theme) {
      setThemeState(serverPreferences.theme === 'auto' ? 'system' : (serverPreferences.theme as ThemeMode));
    }
  }, [isAuthenticated, serverPreferences?.theme]);

  const setTheme = (newTheme: ThemeMode) => {
    setThemeState(newTheme);
    if (isAuthenticated) {
      updatePreferences.mutate({
        theme: newTheme === 'system' ? 'auto' : newTheme,
        comment_read_mode: serverPreferences?.comment_read_mode ?? 'manual',
        font_size: serverPreferences?.font_size ?? 'medium',
      });
    }
  };

  // Track system theme preference
  const [systemTheme, setSystemTheme] = useState<'light' | 'dark'>('light');

  // Detect system theme preference
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    // Update system theme state
    const updateSystemTheme = (e: MediaQueryListEvent | MediaQueryList) => {
      setSystemTheme(e.matches ? 'dark' : 'light');
    };

    // Initial check
    updateSystemTheme(mediaQuery);

    // Listen for changes
    const handler = (e: MediaQueryListEvent) => updateSystemTheme(e);
    mediaQuery.addEventListener('change', handler);

    return () => mediaQuery.removeEventListener('change', handler);
  }, []);

  // Resolve theme - if 'system', use system preference
  const resolvedTheme: ThemeName = theme === 'system' ? systemTheme : theme;

  // Apply theme CSS variables and class
  useEffect(() => {
    const root = document.documentElement;
    const themeValues = themes[resolvedTheme];

    // Apply all CSS variables to :root
    Object.entries(themeValues).forEach(([key, value]) => {
      root.style.setProperty(key, value);
    });

    // Apply theme class for backwards compatibility with existing dark: classes
    // This ensures both old and new systems work simultaneously
    root.classList.remove('light', 'dark');
    root.classList.add(resolvedTheme);

    // Persist theme preference to localStorage
    if (theme !== 'system') {
      localStorage.setItem('app-theme', theme);
    } else {
      // If using system, don't persist (so system changes are respected)
      localStorage.removeItem('app-theme');
    }
  }, [theme, resolvedTheme]);

  const value: ThemeContextType = {
    theme,
    setTheme,
    resolvedTheme,
  };

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}

/**
 * useTheme - Hook to access theme context
 *
 * @throws Error if used outside ThemeProvider
 *
 * @example
 * function MyComponent() {
 *   const { theme, setTheme, resolvedTheme } = useTheme();
 *
 *   return (
 *     <div>
 *       <p>Current theme: {resolvedTheme}</p>
 *       <button onClick={() => setTheme('light')}>Light</button>
 *       <button onClick={() => setTheme('dark')}>Dark</button>
 *       <button onClick={() => setTheme('system')}>System</button>
 *     </div>
 *   );
 * }
 */
// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = (): ThemeContextType => {
  const context = useContext(ThemeContext);

  if (!context) {
    throw new Error('useTheme must be used within ThemeProvider');
  }

  return context;
};
