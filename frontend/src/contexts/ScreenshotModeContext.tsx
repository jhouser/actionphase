import { createContext, useContext, useState, useCallback } from 'react';
import type { ReactNode } from 'react';

interface ScreenshotModeContextValue {
  screenshotModeEnabled: boolean;
  toggleScreenshotMode: () => void;
}

const ScreenshotModeContext = createContext<ScreenshotModeContextValue | undefined>(undefined);

interface ScreenshotModeProviderProps {
  children: ReactNode;
}

/**
 * Session-only (not persisted) toggle that hides real usernames on posts/comments
 * so players in anonymous games can screenshot the Common Room without doxxing
 * who's playing which character. Resets on reload so it's never left on by accident.
 */
export function ScreenshotModeProvider({ children }: ScreenshotModeProviderProps) {
  const [screenshotModeEnabled, setScreenshotModeEnabled] = useState(false);

  const toggleScreenshotMode = useCallback(() => {
    setScreenshotModeEnabled((prev) => !prev);
  }, []);

  return (
    <ScreenshotModeContext.Provider value={{ screenshotModeEnabled, toggleScreenshotMode }}>
      {children}
    </ScreenshotModeContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useScreenshotMode(): ScreenshotModeContextValue {
  const context = useContext(ScreenshotModeContext);
  if (!context) {
    throw new Error('useScreenshotMode must be used within ScreenshotModeProvider');
  }
  return context;
}
