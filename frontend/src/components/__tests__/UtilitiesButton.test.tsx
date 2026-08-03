import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { UtilitiesButton } from '../UtilitiesButton';
import {
  UtilityDrawerProvider,
  useUtilityDrawer,
} from '../../contexts/UtilityDrawerContext';

/** Surfaces the drawer's open state so the test can assert on it. */
function DrawerStateProbe() {
  const { isOpen } = useUtilityDrawer();
  return <span data-testid="drawer-state">{isOpen ? 'open' : 'closed'}</span>;
}

describe('UtilitiesButton', () => {
  it('opens the drawer when clicked', async () => {
    const user = userEvent.setup();

    render(
      <UtilityDrawerProvider>
        <UtilitiesButton />
        <DrawerStateProbe />
      </UtilityDrawerProvider>
    );

    expect(screen.getByTestId('drawer-state')).toHaveTextContent('closed');

    await user.click(screen.getByRole('button', { name: 'Utilities' }));

    expect(screen.getByTestId('drawer-state')).toHaveTextContent('open');
  });

  it('renders nothing when no drawer provider is mounted', () => {
    // Host overlays render in contexts without the drawer (and in tests that
    // don't mount the provider); the entry point should disappear rather than
    // take the whole modal down with it.
    render(<UtilitiesButton />);
    expect(screen.queryByRole('button', { name: 'Utilities' })).not.toBeInTheDocument();
  });
});
