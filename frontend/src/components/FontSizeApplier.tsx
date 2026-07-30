import { useEffect } from 'react';
import { useFontSize } from '../hooks/useUserPreferences';
import type { FontSize } from '../lib/api/auth';

// Multiplies Tailwind's --text-* size tokens (see index.css) without touching
// root font-size, so only typography scales — spacing/sizing (rem-based
// padding, gaps, widths, etc.) stays fixed.
const FONT_SCALE: Record<FontSize, string> = {
  small: '0.875',
  medium: '1',
  large: '1.25',
};

/**
 * Applies the user's font size preference by scaling Tailwind's text-size
 * CSS variables via --font-scale. Deliberately does not touch the root
 * font-size, which would also resize rem-based layout (padding, gaps, card
 * widths, etc.), not just text.
 */
export function FontSizeApplier() {
  const fontSize = useFontSize();

  useEffect(() => {
    document.documentElement.style.setProperty('--font-scale', FONT_SCALE[fontSize]);
    return () => {
      document.documentElement.style.removeProperty('--font-scale');
    };
  }, [fontSize]);

  return null;
}
