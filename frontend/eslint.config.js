import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { globalIgnores } from 'eslint/config'

export default tseslint.config([
  globalIgnores(['dist', 'coverage']),
  // Main source code rules (strict)
  {
    files: ['src/**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs['recommended-latest'],
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // Prevent console statements in production code
      // Use logger service from @/services/LoggingService instead
      'no-console': 'error',

      // Prevent unused variables (except those prefixed with _)
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],

      // Prevent explicit any types (forces proper typing)
      '@typescript-eslint/no-explicit-any': 'error',

      // Warn on debugger statements (should be removed before commit)
      'no-debugger': 'error',

      // Prevent alert/confirm/prompt (use proper UI components)
      'no-alert': 'error',

      // Prevent var usage (enforce let/const)
      'no-var': 'error',

      // Prefer const when variables are never reassigned
      'prefer-const': 'error',

      // Prevent == and != (enforce === and !==)
      'eqeqeq': ['error', 'always'],
    },
  },
  // E2E tests have relaxed rules
  {
    files: ['e2e/**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // Allow unused vars in E2E tests (test helpers often have unused params)
      '@typescript-eslint/no-unused-vars': 'warn',
      // Allow any in E2E tests (Playwright types are often loose)
      '@typescript-eslint/no-explicit-any': 'warn',
      // Warn on console in E2E tests (useful for debugging but discouraged)
      'no-console': 'warn',
    },
  },
])
