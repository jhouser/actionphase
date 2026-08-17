import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { useState } from 'react';
import { useReportDirty } from '../useReportDirty';
import { useDirtyChildren } from '../useDirtyChildren';

/**
 * Exercises the real wiring end to end: a form reports dirty, a manager aggregates
 * per child, and an ancestor stores the result in state — whose setState re-renders
 * the subtree and hands the manager a brand-new arrow callback each time.
 *
 * This is the shape that would sustain a render feedback loop, so these tests assert
 * the chain settles and propagates correctly. They are not a proven regression test
 * for the "Maximum update depth exceeded" error seen once during manual testing:
 * that error was never reproduced afterwards, on this code or on baseline, so its
 * cause remains unattributed.
 */

const Form: React.FC<{ dirty: boolean; onDirtyChange?: (d: boolean) => void }> = ({
  dirty,
  onDirtyChange,
}) => {
  useReportDirty(dirty, onDirtyChange);
  return <div data-testid="form" />;
};

const Manager: React.FC<{
  dirty: boolean;
  onDirtyChange?: (d: boolean) => void;
}> = ({ dirty, onDirtyChange }) => {
  const { report } = useDirtyChildren(onDirtyChange);
  // Inline arrow, recreated every render — exactly what the real managers pass.
  return <Form dirty={dirty} onDirtyChange={(d) => report('child:1', d)} />;
};

const Ancestor: React.FC<{ dirty: boolean }> = ({ dirty }) => {
  const [hasUncommittedEdit, setHasUncommittedEdit] = useState(false);
  return (
    <div>
      <span data-testid="flag">{hasUncommittedEdit ? 'dirty' : 'clean'}</span>
      <Manager dirty={dirty} onDirtyChange={setHasUncommittedEdit} />
    </div>
  );
};

describe('dirty reporting through the real component chain', () => {
  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

  afterEach(() => {
    errorSpy.mockClear();
  });

  it('settles without an infinite update loop when mounted clean', () => {
    render(<Ancestor dirty={false} />);

    expect(screen.getByTestId('flag')).toHaveTextContent('clean');
    expect(errorSpy).not.toHaveBeenCalledWith(
      expect.stringContaining('Maximum update depth exceeded'),
      ...([] as unknown[]),
    );
    // Belt and braces: no React error of any kind mentioning the loop.
    const messages = errorSpy.mock.calls.flat().join(' ');
    expect(messages).not.toContain('Maximum update depth exceeded');
  });

  it('propagates dirty to the ancestor and settles', () => {
    const { rerender } = render(<Ancestor dirty={false} />);

    act(() => {
      rerender(<Ancestor dirty={true} />);
    });

    expect(screen.getByTestId('flag')).toHaveTextContent('dirty');
    const messages = errorSpy.mock.calls.flat().join(' ');
    expect(messages).not.toContain('Maximum update depth exceeded');
  });

  it('returns to clean when the form unmounts while dirty', () => {
    const Wrapper: React.FC<{ show: boolean }> = ({ show }) => {
      const [hasUncommittedEdit, setHasUncommittedEdit] = useState(false);
      return (
        <div>
          <span data-testid="flag">{hasUncommittedEdit ? 'dirty' : 'clean'}</span>
          {show && <Manager dirty={true} onDirtyChange={setHasUncommittedEdit} />}
        </div>
      );
    };

    const { rerender } = render(<Wrapper show={true} />);
    expect(screen.getByTestId('flag')).toHaveTextContent('dirty');

    act(() => {
      rerender(<Wrapper show={false} />);
    });

    expect(screen.getByTestId('flag')).toHaveTextContent('clean');
  });
});
