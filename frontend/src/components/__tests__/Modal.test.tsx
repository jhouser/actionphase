import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, render, fireEvent } from '@testing-library/react'
import { Modal } from '../Modal'

describe('Modal', () => {
  const mockOnClose = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Visibility', () => {
    it('should render nothing when isOpen is false', () => {
      const { container } = render(
        <Modal isOpen={false} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      expect(container.firstChild).toBeNull()
      expect(screen.queryByTestId('modal-content')).not.toBeInTheDocument()
    })

    it('should render modal when isOpen is true', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      expect(screen.getByTestId('modal-content')).toBeInTheDocument()
    })

    it('should change visibility when isOpen prop changes', () => {
      const { rerender } = render(
        <Modal isOpen={false} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      expect(screen.queryByTestId('modal-content')).not.toBeInTheDocument()

      rerender(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      expect(screen.getByTestId('modal-content')).toBeInTheDocument()
    })
  })

  describe('Title', () => {
    it('should display title when provided', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test Modal">
          <div>Content</div>
        </Modal>
      )

      expect(screen.getByRole('heading', { name: 'Test Modal' })).toBeInTheDocument()
    })

    it('should not display title section when title is not provided', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      expect(screen.queryByRole('heading')).not.toBeInTheDocument()
    })

    it('should display close button when title is provided', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test Modal">
          <div>Content</div>
        </Modal>
      )

      const closeButton = screen.getByRole('button')
      expect(closeButton).toBeInTheDocument()

      // Check for the SVG close icon
      const svgElement = closeButton.querySelector('svg')
      expect(svgElement).toBeInTheDocument()
    })

    it('should not display close button when title is not provided', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div>Content</div>
        </Modal>
      )

      expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })
  })

  describe('Close functionality', () => {
    it('should call onClose when backdrop is clicked', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      // Find backdrop by className
      const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement
      expect(backdrop).toBeInTheDocument()

      fireEvent.click(backdrop)

      expect(mockOnClose).toHaveBeenCalledOnce()
    })

    it('should call onClose when close button is clicked', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test Modal">
          <div>Content</div>
        </Modal>
      )

      const closeButton = screen.getByRole('button')
      fireEvent.click(closeButton)

      expect(mockOnClose).toHaveBeenCalledOnce()
    })

    it('should not close when modal content is clicked', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test Modal">
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      const modalContent = screen.getByTestId('modal-content')
      fireEvent.click(modalContent)

      expect(mockOnClose).not.toHaveBeenCalled()
    })

    /**
     * Modals holding editable content opt out of backdrop dismiss. A backdrop click is
     * a slip rather than a decision — nothing was aimed at — and on those surfaces it
     * would discard typing that only lives in local component state.
     */
    it('should not call onClose on backdrop click when dismissOnBackdrop is false', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} dismissOnBackdrop={false}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement
      fireEvent.click(backdrop)

      expect(mockOnClose).not.toHaveBeenCalled()
    })

    it('should still close via the X button when dismissOnBackdrop is false', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test Modal" dismissOnBackdrop={false}>
          <div>Content</div>
        </Modal>
      )

      fireEvent.click(screen.getByRole('button'))

      expect(mockOnClose).toHaveBeenCalledOnce()
    })
  })

  describe('Content rendering', () => {
    it('should render children content', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">
            <p>This is the modal content</p>
          </div>
        </Modal>
      )

      expect(screen.getByText('This is the modal content')).toBeInTheDocument()
    })

    it('should render complex children with multiple elements', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Complex Modal">
          <div>
            <h3>Subtitle</h3>
            <p>Paragraph 1</p>
            <p>Paragraph 2</p>
            <button>Action Button</button>
          </div>
        </Modal>
      )

      expect(screen.getByRole('heading', { name: 'Subtitle' })).toBeInTheDocument()
      expect(screen.getByText('Paragraph 1')).toBeInTheDocument()
      expect(screen.getByText('Paragraph 2')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Action Button' })).toBeInTheDocument()
    })

    it('should render form as children', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Form Modal">
          <form>
            <label htmlFor="email">Email</label>
            <input id="email" type="email" />
            <button type="submit">Submit</button>
          </form>
        </Modal>
      )

      expect(screen.getByLabelText('Email')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Submit' })).toBeInTheDocument()
    })
  })

  describe('Styling and classes', () => {
    it('should have fixed positioning for overlay', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div>Content</div>
        </Modal>
      )

      const overlay = container.querySelector('.fixed.inset-0.z-50')
      expect(overlay).toBeInTheDocument()
    })

    it('should have proper modal container styling', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div>Content</div>
        </Modal>
      )

      const modalContainer = container.querySelector('.surface-raised.rounded-lg.shadow-2xl')
      expect(modalContainer).toBeInTheDocument()
      expect(modalContainer).toHaveClass('max-w-4xl', 'w-full', 'max-h-[90vh]')
    })

    it('should have backdrop with opacity', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div>Content</div>
        </Modal>
      )

      const backdrop = container.querySelector('.bg-black\\/60')
      expect(backdrop).toBeInTheDocument()
    })

    it('should have title section with border when title is provided', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test">
          <div>Content</div>
        </Modal>
      )

      const titleSection = container.querySelector('.border-b.border-theme-default')
      expect(titleSection).toBeInTheDocument()
    })

    it('should have content padding', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div data-testid="modal-content">Content</div>
        </Modal>
      )

      const contentWrapper = screen.getByTestId('modal-content').parentElement
      expect(contentWrapper).toHaveClass('p-3')
      expect(contentWrapper).toHaveClass('sm:p-6')
    })
  })

  describe('Edge cases', () => {
    it('should handle empty children', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <></>
        </Modal>
      )

      expect(container.querySelector('.p-3')).toBeInTheDocument()
    })

    it('should handle null children', () => {
      const { container } = render(
        <Modal isOpen={true} onClose={mockOnClose}>
          {null}
        </Modal>
      )

      expect(container.querySelector('.p-3')).toBeInTheDocument()
    })

    it('should handle empty string title', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="">
          <div>Content</div>
        </Modal>
      )

      // Empty string is falsy in React, so title section should NOT render
      expect(screen.queryByRole('heading')).not.toBeInTheDocument()
      expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })

    it('should handle multiple onClose calls', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Test">
          <div>Content</div>
        </Modal>
      )

      const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement
      const closeButton = screen.getByRole('button')

      fireEvent.click(backdrop)
      fireEvent.click(closeButton)

      expect(mockOnClose).toHaveBeenCalledTimes(2)
    })
  })

  describe('Accessibility', () => {
    it('should have semantic heading for title', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Accessible Modal">
          <div>Content</div>
        </Modal>
      )

      const heading = screen.getByRole('heading', { name: 'Accessible Modal' })
      expect(heading.tagName).toBe('H2')
    })

    it('should have clickable backdrop for dismissal', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose}>
          <div>Content</div>
        </Modal>
      )

      const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement
      expect(backdrop).toBeInTheDocument()

      fireEvent.click(backdrop)
      expect(mockOnClose).toHaveBeenCalled()
    })

    it('should have close button when title is present', () => {
      render(
        <Modal isOpen={true} onClose={mockOnClose} title="Modal Title">
          <div>Content</div>
        </Modal>
      )

      const button = screen.getByRole('button')
      expect(button).toBeInTheDocument()
    })
  })
})
