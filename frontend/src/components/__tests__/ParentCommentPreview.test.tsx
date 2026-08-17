import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ParentCommentPreview } from '../ParentCommentPreview';
import { stubRenderedHeight } from '../../test-utils/renderedHeight';

describe('ParentCommentPreview', () => {
  // CollapsibleMarkdown decides overflow from measured height; jsdom reports 0.
  stubRenderedHeight(500);

  const mockNavigate = vi.fn();

  it('renders parent preview with all information', () => {
    render(
      <ParentCommentPreview
        content="This is the parent comment content"
        createdAt="2025-10-22T10:00:00Z"
        isDeleted={false}
        messageType="post"
        authorUsername="testuser"
        characterName="Test Character"
        onNavigateToParent={mockNavigate}
      />
    );

    expect(screen.getByText('Test Character')).toBeInTheDocument();
    expect(screen.getByText(/this is the parent comment content/i)).toBeInTheDocument();
    expect(screen.getByText(/view in thread/i)).toBeInTheDocument();
  });

  it('shows deleted marker when parent is deleted', () => {
    render(
      <ParentCommentPreview
        content="Deleted content"
        isDeleted={true}
        messageType="post"
        onNavigateToParent={mockNavigate}
      />
    );

    expect(screen.getByText('[deleted]')).toBeInTheDocument();
    expect(screen.queryByText('Deleted content')).not.toBeInTheDocument();
  });

  it('hides navigation button when parent is deleted', () => {
    render(
      <ParentCommentPreview
        content="Deleted content"
        isDeleted={true}
        onNavigateToParent={mockNavigate}
      />
    );

    expect(screen.queryByText(/view in thread/i)).not.toBeInTheDocument();
  });

  it('hides navigation button when onNavigateToParent is not provided', () => {
    render(
      <ParentCommentPreview
        content="Parent content"
        isDeleted={false}
      />
    );

    expect(screen.queryByText(/view in thread/i)).not.toBeInTheDocument();
  });

  it('calls onNavigateToParent when navigation button is clicked', async () => {
    const user = userEvent.setup();
    render(
      <ParentCommentPreview
        content="Parent content"
        isDeleted={false}
        onNavigateToParent={mockNavigate}
      />
    );

    const button = screen.getByText(/view in thread/i);
    await user.click(button);

    expect(mockNavigate).toHaveBeenCalledTimes(1);
  });

  it('truncates long content with line-clamp-2', () => {
    const longContent = 'This is a very long content that should be truncated. '.repeat(10);

    const { container } = render(
      <ParentCommentPreview
        content={longContent}
        isDeleted={false}
      />
    );

    // Collapsed to a fixed height with a fade (previously line-clamp-2). The
    // full text stays in the DOM — clipping is visual, so markdown can't be cut
    // mid-syntax.
    const contentElement = container.querySelector('[data-collapsed="true"]');
    expect(contentElement).toBeInTheDocument();
    expect(contentElement?.textContent).toContain('This is a very long content that should be truncated.');
  });

  it('returns null when no content and not deleted', () => {
    const { container } = render(
      <ParentCommentPreview
        content={null}
        isDeleted={false}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it('renders when content is null but isDeleted is true', () => {
    render(
      <ParentCommentPreview
        content={null}
        isDeleted={true}
      />
    );

    expect(screen.getByText('[deleted]')).toBeInTheDocument();
  });

  it('renders content when createdAt is provided', () => {
    const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();

    render(
      <ParentCommentPreview
        content="Parent content"
        createdAt={oneHourAgo}
      />
    );

    expect(screen.getByText(/parent content/i)).toBeInTheDocument();
  });

  it('handles missing optional fields gracefully', () => {
    render(
      <ParentCommentPreview
        content="Parent content"
      />
    );

    expect(screen.getByText(/parent content/i)).toBeInTheDocument();
  });

  it('starts collapsed by default', () => {
    const { container } = render(
      <ParentCommentPreview
        content="Parent content"
      />
    );

    expect(screen.getByText('Expand')).toBeInTheDocument();
    expect(screen.queryByText('Collapse')).not.toBeInTheDocument();
    expect(container.querySelector('[data-collapsed="true"]')).toBeInTheDocument();
  });

  it('expands when expand button is clicked', async () => {
    const user = userEvent.setup();
    render(
      <ParentCommentPreview
        content="Parent content that should be expanded"
      />
    );

    const expandButton = screen.getByText('Expand');
    await user.click(expandButton);

    expect(screen.getByText('Collapse')).toBeInTheDocument();
    expect(screen.queryByText('Expand')).not.toBeInTheDocument();
  });

  it('collapses when collapse button is clicked', async () => {
    const user = userEvent.setup();
    render(
      <ParentCommentPreview
        content="Parent content"
        defaultExpanded={true}
      />
    );

    // Should start expanded
    expect(screen.getByText('Collapse')).toBeInTheDocument();

    const collapseButton = screen.getByText('Collapse');
    await user.click(collapseButton);

    expect(screen.getByText('Expand')).toBeInTheDocument();
    expect(screen.queryByText('Collapse')).not.toBeInTheDocument();
  });

  it('starts expanded when defaultExpanded is true', () => {
    render(
      <ParentCommentPreview
        content="Parent content"
        defaultExpanded={true}
      />
    );

    expect(screen.getByText('Collapse')).toBeInTheDocument();
    expect(screen.queryByText('Expand')).not.toBeInTheDocument();
  });

  it('hides expand button when parent is deleted', () => {
    render(
      <ParentCommentPreview
        content="Deleted content"
        isDeleted={true}
      />
    );

    expect(screen.queryByText('Expand')).not.toBeInTheDocument();
    expect(screen.queryByText('Collapse')).not.toBeInTheDocument();
  });
});
