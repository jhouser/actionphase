/**
 * Theme definitions containing actual color values.
 *
 * Each theme is a set of CSS variable values that map to semantic tokens.
 * RGB values (e.g., "255 255 255") are used for compatibility with Tailwind's
 * opacity modifiers (e.g., bg-surface-base/50).
 *
 * To add a new theme:
 * 1. Add theme name to this file
 * 2. Define all CSS variables with RGB values
 * 3. Theme will automatically work with all components
 *
 * @example
 * themes.highContrast = {
 *   '--color-surface-base': '0 0 0',
 *   '--color-content-primary': '255 255 255',
 *   // ... other variables
 * };
 */

export const themes = {
  /**
   * Light theme - default daytime theme
   * Uses grays and blues for a clean, professional look
   */
  light: {
    // Surface colors - backgrounds and containers
    '--color-surface-base': '255 255 255',        // white - cards, modals
    '--color-surface-raised': '249 250 251',      // gray-50 - hover states, active tabs
    '--color-surface-overlay': '255 255 255',     // white - dropdowns, popovers
    '--color-surface-sunken': '243 244 246',      // gray-100 - input backgrounds

    // Content colors - text and foreground
    '--color-content-primary': '17 24 39',        // gray-900 - headings, body text
    '--color-content-secondary': '75 85 99',      // gray-600 - supporting text
    '--color-content-tertiary': '107 114 128',    // gray-500 - muted text
    '--color-content-disabled': '156 163 175',    // gray-400 - disabled state
    '--color-content-inverse': '255 255 255',     // white - text on dark backgrounds

    // Interactive colors - buttons and actions
    '--color-interactive-primary': '37 99 235',         // blue-600 - primary buttons
    '--color-interactive-primary-hover': '29 78 216',   // blue-700 - primary button hover
    '--color-interactive-primary-subtle': '239 246 255', // blue-50 - subtle backgrounds for badges
    '--color-interactive-secondary': '243 244 246',     // gray-100 - secondary buttons
    '--color-interactive-secondary-hover': '229 231 235', // gray-200 - secondary hover

    // Semantic colors - status and feedback
    '--color-semantic-danger': '220 38 38',             // red-600
    '--color-semantic-danger-subtle': '254 226 226',    // red-100
    '--color-semantic-warning': '217 119 6',            // yellow-600
    '--color-semantic-warning-subtle': '254 249 195',   // yellow-100
    '--color-semantic-success': '22 163 74',            // green-600
    '--color-semantic-success-subtle': '220 252 231',   // green-100
    '--color-semantic-info': '37 99 235',               // blue-600
    '--color-semantic-info-subtle': '219 234 254',      // blue-100

    // Border colors
    '--color-border-default': '229 231 235',      // gray-200 - standard borders
    '--color-border-subtle': '243 244 246',       // gray-100 - light dividers
    '--color-border-strong': '156 163 175',       // gray-400 - emphasized borders

    // Avatar colors - diverse palette for character avatars
    '--color-avatar-1': '239 68 68',              // red-500
    '--color-avatar-2': '249 115 22',             // orange-500
    '--color-avatar-3': '245 158 11',             // yellow-500
    '--color-avatar-4': '34 197 94',              // green-500
    '--color-avatar-5': '20 184 166',             // teal-500
    '--color-avatar-6': '59 130 246',             // blue-500
    '--color-avatar-7': '139 92 246',             // violet-500
    '--color-avatar-8': '236 72 153',             // pink-500
  },

  /**
   * Dark theme - nighttime theme
   * Uses darker grays with slightly brighter interactive colors
   */
  dark: {
    // Surface colors - backgrounds and containers
    '--color-surface-base': '31 41 55',           // gray-800 - cards, modals
    '--color-surface-raised': '17 24 39',         // gray-900 - hover states, active tabs
    '--color-surface-overlay': '55 65 81',        // gray-700 - dropdowns, popovers
    '--color-surface-sunken': '17 24 39',         // gray-900 - input backgrounds

    // Content colors - text and foreground
    '--color-content-primary': '255 255 255',     // white - headings, body text
    '--color-content-secondary': '209 213 219',   // gray-300 - supporting text
    '--color-content-tertiary': '156 163 175',    // gray-400 - muted text
    '--color-content-disabled': '107 114 128',    // gray-500 - disabled state
    '--color-content-inverse': '17 24 39',        // gray-900 - text on light backgrounds

    // Interactive colors - buttons and actions
    '--color-interactive-primary': '59 130 246',        // blue-500 - primary buttons
    '--color-interactive-primary-hover': '37 99 235',   // blue-600 - primary button hover
    '--color-interactive-primary-subtle': '30 58 138',  // blue-900 - subtle backgrounds for badges
    '--color-interactive-secondary': '55 65 81',        // gray-700 - secondary buttons
    '--color-interactive-secondary-hover': '75 85 99',  // gray-600 - secondary hover

    // Semantic colors - status and feedback
    '--color-semantic-danger': '239 68 68',             // red-500
    '--color-semantic-danger-subtle': '185 28 28',      // red-700 - less washed out than red-900
    '--color-semantic-warning': '245 158 11',           // yellow-500
    '--color-semantic-warning-subtle': '161 98 7',      // yellow-700 - less washed out than yellow-900
    '--color-semantic-success': '34 197 94',            // green-500
    '--color-semantic-success-subtle': '20 83 45',      // green-900
    '--color-semantic-info': '59 130 246',              // blue-500
    '--color-semantic-info-subtle': '30 58 138',        // blue-900

    // Border colors
    '--color-border-default': '55 65 81',         // gray-700 - standard borders
    '--color-border-subtle': '75 85 99',          // gray-600 - light dividers
    '--color-border-strong': '107 114 128',       // gray-500 - emphasized borders

    // Avatar colors - slightly brighter for dark theme
    '--color-avatar-1': '248 113 113',            // red-400
    '--color-avatar-2': '251 146 60',             // orange-400
    '--color-avatar-3': '250 204 21',             // yellow-400
    '--color-avatar-4': '74 222 128',             // green-400
    '--color-avatar-5': '45 212 191',             // teal-400
    '--color-avatar-6': '96 165 250',             // blue-400
    '--color-avatar-7': '167 139 250',            // violet-400
    '--color-avatar-8': '244 114 182',            // pink-400
  },

  /**
   * High Contrast Light theme - maximum contrast for accessibility
   * Uses pure black and white with bold accent colors
   * WCAG AAA compliant contrast ratios
   */
  highContrast: {
    // Surface colors - pure whites and black
    '--color-surface-base': '255 255 255',        // pure white - cards, modals
    '--color-surface-raised': '245 245 245',      // very light gray - hover states
    '--color-surface-overlay': '255 255 255',     // pure white - dropdowns
    '--color-surface-sunken': '240 240 240',      // light gray - inputs
    '--color-surface-page': '250 250 250',        // very light gray - page background

    // Content colors - maximum contrast
    '--color-content-primary': '0 0 0',           // pure black - headings, body
    '--color-content-secondary': '30 30 30',      // near black - supporting text
    '--color-content-tertiary': '70 70 70',       // dark gray - muted text
    '--color-content-disabled': '150 150 150',    // mid gray - disabled
    '--color-content-inverse': '255 255 255',     // pure white - text on dark

    // Interactive colors - bold and distinct
    '--color-interactive-primary': '0 0 180',           // strong blue
    '--color-interactive-primary-hover': '0 0 140',     // darker blue
    '--color-interactive-primary-subtle': '230 240 255', // very light blue - subtle backgrounds for badges
    '--color-interactive-secondary': '230 230 230',     // light gray
    '--color-interactive-secondary-hover': '210 210 210', // medium gray

    // Semantic colors - maximum contrast and distinct
    '--color-semantic-danger': '180 0 0',               // strong red
    '--color-semantic-danger-subtle': '255 230 230',    // light red
    '--color-semantic-warning': '180 120 0',            // strong orange
    '--color-semantic-warning-subtle': '255 245 220',   // light orange
    '--color-semantic-success': '0 130 0',              // strong green
    '--color-semantic-success-subtle': '230 255 230',   // light green
    '--color-semantic-info': '0 0 180',                 // strong blue
    '--color-semantic-info-subtle': '230 240 255',      // light blue

    // Border colors - strong contrast
    '--color-border-default': '0 0 0',            // black - standard borders
    '--color-border-subtle': '180 180 180',       // mid gray - light dividers
    '--color-border-strong': '0 0 0',             // black - emphasized borders

    // Avatar colors - bold and saturated for high contrast
    '--color-avatar-1': '200 0 0',                // strong red
    '--color-avatar-2': '200 100 0',              // strong orange
    '--color-avatar-3': '180 140 0',              // strong yellow-orange
    '--color-avatar-4': '0 150 0',                // strong green
    '--color-avatar-5': '0 140 140',              // strong teal
    '--color-avatar-6': '0 0 200',                // strong blue
    '--color-avatar-7': '100 0 180',              // strong purple
    '--color-avatar-8': '180 0 120',              // strong magenta
  },

  /**
   * High Contrast Dark theme - maximum contrast dark mode
   * Uses pure black background with bright colors
   * WCAG AAA compliant contrast ratios
   */
  highContrastDark: {
    // Surface colors - pure blacks and dark grays
    '--color-surface-base': '0 0 0',              // pure black - cards, modals
    '--color-surface-raised': '20 20 20',         // very dark gray - hover states
    '--color-surface-overlay': '30 30 30',        // dark gray - dropdowns
    '--color-surface-sunken': '0 0 0',            // pure black - inputs
    '--color-surface-page': '0 0 0',              // pure black - page background

    // Content colors - maximum contrast
    '--color-content-primary': '255 255 255',     // pure white - headings, body
    '--color-content-secondary': '230 230 230',   // light gray - supporting text
    '--color-content-tertiary': '180 180 180',    // medium gray - muted text
    '--color-content-disabled': '100 100 100',    // dark gray - disabled
    '--color-content-inverse': '0 0 0',           // pure black - text on light

    // Interactive colors - bright and bold
    '--color-interactive-primary': '100 150 255',       // bright blue
    '--color-interactive-primary-hover': '140 180 255', // brighter blue
    '--color-interactive-primary-subtle': '20 30 60',   // dark blue - subtle backgrounds for badges
    '--color-interactive-secondary': '60 60 60',        // dark gray
    '--color-interactive-secondary-hover': '80 80 80',  // lighter dark gray

    // Semantic colors - bright and distinct
    '--color-semantic-danger': '255 100 100',           // bright red
    '--color-semantic-danger-subtle': '127 29 29',      // red-900 - more saturated than 60 20 20
    '--color-semantic-warning': '255 200 100',          // bright orange
    '--color-semantic-warning-subtle': '120 53 15',     // orange-900 - more saturated than 60 50 20
    '--color-semantic-success': '100 255 100',          // bright green
    '--color-semantic-success-subtle': '20 60 20',      // dark green
    '--color-semantic-info': '100 150 255',             // bright blue
    '--color-semantic-info-subtle': '20 30 60',         // dark blue

    // Border colors - bright contrast
    '--color-border-default': '255 255 255',      // white - standard borders
    '--color-border-subtle': '80 80 80',          // dark gray - light dividers
    '--color-border-strong': '255 255 255',       // white - emphasized borders

    // Avatar colors - very bright for high contrast dark
    '--color-avatar-1': '255 120 120',            // bright red
    '--color-avatar-2': '255 180 100',            // bright orange
    '--color-avatar-3': '255 220 100',            // bright yellow
    '--color-avatar-4': '120 255 120',            // bright green
    '--color-avatar-5': '100 240 240',            // bright cyan
    '--color-avatar-6': '120 160 255',            // bright blue
    '--color-avatar-7': '180 140 255',            // bright purple
    '--color-avatar-8': '255 120 200',            // bright pink
  },

  /**
   * Colorblind-friendly theme
   * Optimized for all types of color vision deficiency
   * Uses blue-orange-yellow palette which is distinguishable for deuteranopia, protanopia, and tritanopia
   * Avoids red-green combinations
   */
  colorblind: {
    // Surface colors - warm tinted backgrounds to differentiate from light theme
    '--color-surface-base': '255 253 247',        // warm white - cards, modals
    '--color-surface-raised': '254 249 238',      // warm beige - hover states
    '--color-surface-overlay': '255 253 247',     // warm white - dropdowns
    '--color-surface-sunken': '250 245 235',      // light warm - inputs
    '--color-surface-page': '252 247 237',        // warm beige - page background

    // Content colors - high contrast blacks and grays
    '--color-content-primary': '17 24 39',        // gray-900 - headings
    '--color-content-secondary': '55 65 81',      // gray-700 - supporting (darker for contrast)
    '--color-content-tertiary': '75 85 99',       // gray-600 - muted
    '--color-content-disabled': '156 163 175',    // gray-400 - disabled
    '--color-content-inverse': '255 255 255',     // white - text on dark

    // Interactive colors - strong cyan-blue (universally distinguishable)
    '--color-interactive-primary': '6 182 212',         // cyan-600 - very distinct
    '--color-interactive-primary-hover': '8 145 178',   // cyan-700 - hover
    '--color-interactive-primary-subtle': '207 250 254', // cyan-100 - light cyan subtle backgrounds for badges
    '--color-interactive-secondary': '254 249 238',     // warm beige - secondary
    '--color-interactive-secondary-hover': '250 245 235', // darker warm - hover

    // Semantic colors - colorblind-safe palette with maximum distinction
    // Danger: Strong Orange (not red) - highly visible and distinct from blue
    '--color-semantic-danger': '249 115 22',            // orange-500 - bright and distinct
    '--color-semantic-danger-subtle': '255 237 213',    // orange-100
    // Warning: Bright Yellow - maximum brightness contrast
    '--color-semantic-warning': '234 179 8',            // yellow-500 - very bright
    '--color-semantic-warning-subtle': '254 252 232',   // yellow-50
    // Success: Teal (not green) - distinct from cyan primary
    '--color-semantic-success': '20 184 166',           // teal-500 - clearly different from cyan
    '--color-semantic-success-subtle': '204 251 241',   // teal-100
    // Info: Deep Purple - completely different from other colors
    '--color-semantic-info': '124 58 237',              // violet-600 - strong purple
    '--color-semantic-info-subtle': '237 233 254',      // violet-100

    // Border colors - warmer tones
    '--color-border-default': '217 119 6',        // orange-600 - warm accent borders
    '--color-border-subtle': '253 230 138',       // yellow-200 - warm light dividers
    '--color-border-strong': '6 182 212',         // cyan-600 - emphasized borders match primary

    // Avatar colors - colorblind-safe distinctive colors
    '--color-avatar-1': '249 115 22',             // orange-500 (not red - colorblind safe)
    '--color-avatar-2': '217 119 6',              // orange-600 (darker variant)
    '--color-avatar-3': '234 179 8',              // yellow-500 (bright, distinct)
    '--color-avatar-4': '20 184 166',             // teal-500 (not green - colorblind safe)
    '--color-avatar-5': '6 182 212',              // cyan-600 (distinct from teal)
    '--color-avatar-6': '59 130 246',             // blue-500 (distinct from cyan)
    '--color-avatar-7': '124 58 237',             // violet-600 (purple - distinct)
    '--color-avatar-8': '161 98 7',               // yellow-700 (brown - very distinct)
  },
} as const;

export type ThemeName = keyof typeof themes;
