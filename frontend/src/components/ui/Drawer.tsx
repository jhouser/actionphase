import { Fragment } from 'react';
import type { ReactNode } from 'react';
import { Dialog, DialogPanel, DialogTitle, Transition, TransitionChild } from '@headlessui/react';
import { XMarkIcon } from '@heroicons/react/24/outline';

export interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  /**
   * 'responsive' (default): sidebar on lg+, bottom sheet below lg.
   * 'right': always sidebar.
   * 'bottom': always bottom sheet.
   */
  side?: 'bottom' | 'right' | 'responsive';
  /**
   * Stacking tier. Defaults to the modal tier so existing callers are
   * unaffected; the utility drawer passes `LAYERS.drawer` to sit above modals.
   */
  zIndexClass?: string;
  /**
   * Hide the drawer's own backdrop. Set when opening over an overlay that is
   * already dimming the page, where a second scrim just muddies it.
   */
  hideBackdrop?: boolean;
  /**
   * Extra classes for the panel — used to cap the mobile sheet height when the
   * drawer stacks over another overlay.
   */
  panelClassName?: string;
}

export function Drawer({
  open,
  onClose,
  title,
  children,
  side = 'responsive',
  zIndexClass = 'z-50',
  hideBackdrop = false,
  panelClassName = '',
}: DrawerProps) {
  // Panel positioning classes
  const panelClasses = {
    right: 'fixed inset-y-0 right-0 flex flex-col w-80 max-w-full surface-raised border-l border-theme-default shadow-xl',
    bottom: 'fixed bottom-0 inset-x-0 flex flex-col max-h-[80vh] rounded-t-2xl surface-raised border-t border-theme-default shadow-xl',
    responsive: 'fixed flex flex-col surface-raised shadow-xl ' +
      // Desktop: right sidebar (lg:left-auto cancels the non-prefixed inset-x-0)
      'lg:inset-y-0 lg:right-0 lg:left-auto lg:w-80 lg:max-w-full lg:border-l lg:border-theme-default ' +
      // Mobile: bottom sheet
      'bottom-0 inset-x-0 max-h-[80vh] rounded-t-2xl border-t border-theme-default lg:rounded-none lg:max-h-full',
  }[side];

  // Transition: right panels slide from right; bottom panels slide from bottom
  const enterFrom = side === 'bottom' ? 'opacity-0 translate-y-full' :
    side === 'right' ? 'opacity-0 translate-x-full' :
      // responsive: translate-y on mobile, translate-x on desktop — use a combined approach
      'opacity-0 translate-y-full lg:translate-y-0 lg:translate-x-full';
  const enterTo = side === 'bottom' ? 'opacity-100 translate-y-0' :
    side === 'right' ? 'opacity-100 translate-x-0' :
      'opacity-100 translate-y-0 lg:translate-x-0';
  const leaveFrom = enterTo;
  const leaveTo = enterFrom;

  return (
    <Transition show={open} as={Fragment}>
      <Dialog as="div" className={`relative ${zIndexClass}`} onClose={onClose}>
        {/* Backdrop. The scrim is suppressed when stacking over an overlay that
            already dims the page, but the element still renders: Headless UI
            owns outside-click-to-close, and it measures the click against this
            container. */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-200"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-150"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div
            className={`fixed inset-0 ${hideBackdrop ? '' : 'bg-black/40'}`}
            aria-hidden="true"
            data-testid="drawer-backdrop"
          />
        </TransitionChild>

        {/* Panel */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-200"
          enterFrom={enterFrom}
          enterTo={enterTo}
          leave="ease-in duration-150"
          leaveFrom={leaveFrom}
          leaveTo={leaveTo}
        >
          <DialogPanel className={`${panelClasses} ${panelClassName}`}>
            {/* Drag handle — only visible on bottom sheet */}
            {(side === 'bottom' || side === 'responsive') && (
              <div className={`flex justify-center pt-3 pb-1 shrink-0 ${side === 'responsive' ? 'lg:hidden' : ''}`}>
                <div className="w-10 h-1 rounded-full bg-theme-default" />
              </div>
            )}

            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-theme-default shrink-0">
              {title ? (
                <DialogTitle as="h2" className="text-base font-semibold text-content-primary">
                  {title}
                </DialogTitle>
              ) : (
                <span />
              )}
              <button
                type="button"
                onClick={onClose}
                className="p-1 rounded-md text-content-secondary hover:text-content-primary hover:surface-base transition-colors"
                aria-label="Close"
              >
                <XMarkIcon className="h-5 w-5" />
              </button>
            </div>

            {/* Scrollable content */}
            <div className="flex-1 overflow-y-auto">
              {children}
            </div>
          </DialogPanel>
        </TransitionChild>
      </Dialog>
    </Transition>
  );
}
