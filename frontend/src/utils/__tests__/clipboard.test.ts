import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { copyToClipboard } from '../clipboard';

/**
 * copyToClipboard has two strategies: the async Clipboard API in secure
 * contexts, and a hidden-textarea + execCommand fallback everywhere else.
 *
 * The fallback is the reason this module exists, and it never runs during
 * local development — localhost is always a secure context — so these tests
 * are the only thing exercising it. They also pin the cleanup contract: the
 * temporary textarea must never be left behind in the DOM, on any path.
 */

const originalClipboard = navigator.clipboard;
const originalIsSecureContext = window.isSecureContext;

function setContext({
  secure,
  clipboard,
}: {
  secure: boolean;
  clipboard: { writeText: ReturnType<typeof vi.fn> } | undefined;
}) {
  Object.defineProperty(window, 'isSecureContext', {
    value: secure,
    configurable: true,
  });
  Object.defineProperty(navigator, 'clipboard', {
    value: clipboard,
    configurable: true,
  });
}

function textAreasInDom() {
  return document.body.querySelectorAll('textarea').length;
}

describe('copyToClipboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = '';
  });

  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      configurable: true,
    });
    Object.defineProperty(window, 'isSecureContext', {
      value: originalIsSecureContext,
      configurable: true,
    });
  });

  describe('secure context (Clipboard API available)', () => {
    it('writes via the Clipboard API and does not touch the DOM', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      setContext({ secure: true, clipboard: { writeText } });

      await expect(copyToClipboard('hello')).resolves.toBe(true);

      expect(writeText).toHaveBeenCalledWith('hello');
      // The fallback must not run when the real API is available.
      expect(textAreasInDom()).toBe(0);
    });

    it('propagates a Clipboard API rejection rather than silently falling back', async () => {
      const writeText = vi.fn().mockRejectedValue(new Error('denied'));
      setContext({ secure: true, clipboard: { writeText } });

      await expect(copyToClipboard('hello')).rejects.toThrow('denied');
    });
  });

  describe('insecure context (execCommand fallback)', () => {
    it('copies via execCommand when the Clipboard API is absent', async () => {
      setContext({ secure: false, clipboard: undefined });
      const execCommand = vi.fn().mockReturnValue(true);
      document.execCommand = execCommand;

      await expect(copyToClipboard('fallback text')).resolves.toBe(true);

      expect(execCommand).toHaveBeenCalledWith('copy');
      expect(textAreasInDom()).toBe(0);
    });

    it('falls back when isSecureContext is false even if clipboard exists', async () => {
      // http origins can expose navigator.clipboard while forbidding its use.
      const writeText = vi.fn();
      setContext({ secure: false, clipboard: { writeText } });
      document.execCommand = vi.fn().mockReturnValue(true);

      await expect(copyToClipboard('text')).resolves.toBe(true);

      expect(writeText).not.toHaveBeenCalled();
    });

    it('puts the text in the textarea and selects it before copying', async () => {
      setContext({ secure: false, clipboard: undefined });
      let valueAtCopyTime: string | undefined;
      document.execCommand = vi.fn(() => {
        // Capture DOM state at the moment execCommand runs — afterwards the
        // textarea is removed, so this is the only point it can be observed.
        valueAtCopyTime = document.body.querySelector('textarea')?.value;
        return true;
      });

      await copyToClipboard('selected content');

      expect(valueAtCopyTime).toBe('selected content');
    });

    it('throws and cleans up when execCommand reports failure', async () => {
      setContext({ secure: false, clipboard: undefined });
      document.execCommand = vi.fn().mockReturnValue(false);

      await expect(copyToClipboard('text')).rejects.toThrow(
        'execCommand copy returned false'
      );
      expect(textAreasInDom()).toBe(0);
    });

    it('throws and cleans up when execCommand itself throws', async () => {
      setContext({ secure: false, clipboard: undefined });
      document.execCommand = vi.fn(() => {
        throw new Error('execCommand unavailable');
      });

      await expect(copyToClipboard('text')).rejects.toThrow(
        'execCommand unavailable'
      );
      // A leaked textarea would accumulate on every failed copy.
      expect(textAreasInDom()).toBe(0);
    });
  });
});
