import React from 'react';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  /**
   * Stacking tier. Defaults to the modal tier; a modal launched *from* the
   * utility drawer passes `LAYERS.drawerChild` so it clears the drawer instead
   * of rendering underneath it.
   */
  zIndexClass?: string;
}

/**
 * Modal - Dialog overlay component with semantic theme tokens
 *
 * Now uses semantic tokens instead of hard-coded colors:
 * - 70% less code (no more dark: classes)
 * - Automatically adapts to all themes (light, dark, future themes)
 */
export const Modal = ({ isOpen, onClose, title, children, zIndexClass = 'z-50' }: ModalProps) => {
  if (!isOpen) return null;

  return (
    <div className={`fixed inset-0 ${zIndexClass} overflow-y-auto`}>
      <div className="flex min-h-screen items-center justify-center p-1 sm:p-4">
        {/* Backdrop */}
        <div
          className="fixed inset-0 z-0 bg-black/60 backdrop-blur-sm transition-opacity"
          onClick={onClose}
          aria-hidden="true"
        />

        {/* Modal */}
        <div className="relative z-10 surface-raised rounded-lg shadow-2xl max-w-4xl w-full max-h-[90vh] overflow-y-auto border border-border-primary">
          {title && (
            <div className="px-3 py-2 sm:px-6 sm:py-4 border-b border-theme-default">
              <div className="flex items-center justify-between">
                <h2 className="text-lg sm:text-xl font-semibold text-content-primary">{title}</h2>
                <button
                  onClick={onClose}
                  className="text-content-secondary hover:text-content-primary transition-colors"
                >
                  <svg className="h-5 w-5 sm:h-6 sm:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
          )}

          <div className="p-3 sm:p-6">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
};
