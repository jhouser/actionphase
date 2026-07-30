import { useState, useRef, useEffect } from 'react';
import { Copy, Check, Dices } from 'lucide-react';
import { Button, Input } from '../../ui';
import { rollDice, formatRollMarkdown } from '../../../utils/dice';
import type { RollResult } from '../../../utils/dice';
import { copyToClipboard } from '../../../utils/clipboard';
import { logger } from '@/services/LoggingService';
import type { UtilityPanelProps } from '../types';

const QUICK_DICE = ['d4', 'd6', 'd8', 'd10', 'd12', 'd20', 'd100'];

/**
 * Cosmetic (client-side) dice roller. Produces a formatted markdown line the
 * player can copy into a common-room reply. Rolls are not server-verified.
 */
export function DiceRollerPanel({ ctx }: UtilityPanelProps) {
  const userCharacters = ctx.game?.userCharacters ?? [];
  // Attribute rolls to the sole controlled character, if there's exactly one.
  // Outside a game there is no character to attribute to, so rolls are unnamed.
  const characterName =
    userCharacters.length === 1 ? userCharacters[0].name : undefined;

  const [notation, setNotation] = useState('d20');
  const [result, setResult] = useState<RollResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const copyResetRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (copyResetRef.current) clearTimeout(copyResetRef.current);
    };
  }, []);

  const roll = (input: string) => {
    const r = rollDice(input);
    if (!r) {
      setError('Enter valid dice notation, e.g. d20, 2d6+3.');
      setResult(null);
      return;
    }
    setError(null);
    setResult(r);
    setCopied(false);
  };

  const markdown = result ? formatRollMarkdown(result, characterName) : '';

  const handleCopy = async () => {
    if (!markdown) return;
    try {
      await copyToClipboard(markdown);
      setCopied(true);
      if (copyResetRef.current) clearTimeout(copyResetRef.current);
      copyResetRef.current = setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      logger.error('Failed to copy dice roll', { error: err });
      setError('Could not copy to clipboard.');
    }
  };

  return (
    <div className="flex flex-col gap-4 p-4">
      {/* Quick-roll buttons */}
      <div className="flex flex-wrap gap-2">
        {QUICK_DICE.map((d) => (
          <Button
            key={d}
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => {
              setNotation(d);
              roll(d);
            }}
            data-faro-user-action-name="roll-quick-dice"
          >
            {d}
          </Button>
        ))}
      </div>

      {/* Custom notation */}
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          roll(notation);
        }}
      >
        <div className="flex-1">
          <Input
            label="Custom roll"
            value={notation}
            onChange={(e) => setNotation(e.target.value)}
            placeholder="e.g. 2d6+3"
            aria-label="Dice notation"
          />
        </div>
        <Button
          type="submit"
          variant="primary"
          data-faro-user-action-name="roll-custom-dice"
        >
          <Dices className="w-4 h-4" />
          Roll
        </Button>
      </form>

      {error && <p className="text-sm text-status-danger">{error}</p>}

      {/* Result */}
      {result && (
        <div className="rounded-lg border border-theme-default surface-raised p-4">
          <div className="text-center">
            <div className="text-3xl font-bold text-content-primary" data-testid="dice-total">
              {result.total}
            </div>
            <div className="mt-1 text-xs text-content-secondary">
              {result.notation}
              {(result.dice.length > 1 || result.modifier !== 0) && (
                <> → {result.dice.map((d) => d.value).join(', ')}
                  {result.modifier > 0 && ` +${result.modifier}`}
                  {result.modifier < 0 && ` ${result.modifier}`}
                </>
              )}
            </div>
          </div>

          <div className="mt-3 flex items-center gap-2 rounded-md surface-base px-3 py-2">
            <code className="flex-1 text-xs text-content-secondary break-words" data-testid="dice-markdown">
              {markdown}
            </code>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleCopy}
              aria-label="Copy roll"
              data-faro-user-action-name="copy-dice-roll"
              className="!px-2 shrink-0"
            >
              {copied ? (
                <>
                  <Check className="w-4 h-4" />
                  <span className="hidden sm:inline">Copied</span>
                </>
              ) : (
                <>
                  <Copy className="w-4 h-4" />
                  <span className="hidden sm:inline">Copy</span>
                </>
              )}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
