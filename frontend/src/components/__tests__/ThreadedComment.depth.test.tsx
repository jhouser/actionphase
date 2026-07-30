import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThreadedComment } from '../ThreadedComment';
import type { Message } from '../../types/messages';
import { ToastProvider } from '../../contexts/ToastContext'
import { AdminModeProvider } from '../../contexts/AdminModeContext';
import { ScreenshotModeProvider } from '../../contexts/ScreenshotModeContext';
import { AuthProvider } from '../../contexts/AuthContext'

// Mock fetch globally
global.fetch = vi.fn();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false },
    mutations: { retry: false },
  },
});

const mockComment: Message = {
  id: 1,
  game_id: 1,
  phase_id: 1,
  author_id: 100,
  author_username: 'testuser',
  character_id: 10,
  character_name: 'Test Character',
  character_avatar_url: null,
  content: 'Test comment content',
  message_type: 'comment',
  parent_id: null,
  visibility: 'all',
  created_at: '2025-01-15T12:00:00Z',
  updated_at: '2025-01-15T12:00:00Z',
  is_deleted: false,
  is_edited: false,
  reply_count: 3,
  mentioned_character_ids: [],
};

describe('ThreadedComment - Depth Limiting', () => {
  it('should show reply button when under max depth', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
          <AuthProvider>
            <AdminModeProvider>
          <ScreenshotModeProvider>
              <ThreadedComment
                comment={mockComment}
                gameId={1}
                characters={[]}
                controllableCharacters={[{ id: 10, name: 'Test Character', game_id: 1, owner_id: 100, avatar_url: null, character_sheet: null, is_gm: false, created_at: '', updated_at: '' }]}
                onCreateReply={vi.fn()}
                depth={0}
                maxDepth={5}
              />
            </ScreenshotModeProvider>
          </AdminModeProvider>
          </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.getByText('Reply')).toBeInTheDocument();
  });

  it('should hide reply button at max depth', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={mockComment}
              gameId={1}
              characters={[]}
              controllableCharacters={[{ id: 10, name: 'Test Character', game_id: 1, owner_id: 100, avatar_url: null, character_sheet: null, is_gm: false, created_at: '', updated_at: '' }]}
              onCreateReply={vi.fn()}
              depth={5}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.queryByText('Reply')).not.toBeInTheDocument();
  });

  it('should show "Continue thread" link at max depth with replies', () => {
    const commentWithReplies: Message = {
      ...mockComment,
      reply_count: 3,
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithReplies}
              gameId={1}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              depth={4}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    const continueLink = screen.getAllByText(/Continue this thread/)[0];
    expect(continueLink).toBeInTheDocument();
    expect(screen.getAllByText(/3 replies/)[0]).toBeInTheDocument();
  });

  it('should not show "Continue thread" link at max depth without replies', () => {
    const commentWithoutReplies: Message = {
      ...mockComment,
      reply_count: 0,
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithoutReplies}
              gameId={1}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              depth={5}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.queryByText(/Continue this thread/)).not.toBeInTheDocument();
  });

  it('should call onOpenThread when continue thread button clicked', () => {
    const commentWithReplies: Message = {
      ...mockComment,
      id: 123,
      reply_count: 2,
    };

    const onOpenThread = vi.fn();

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithReplies}
              gameId={42}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              onOpenThread={onOpenThread}
              depth={4}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    const button = screen.getAllByText(/Continue this thread/)[0].closest('button');
    expect(button).toBeInTheDocument();
    button?.click();
    expect(onOpenThread).toHaveBeenCalledWith(commentWithReplies);
  });

  it('should hide replies collapse button at max depth', () => {
    const commentWithReplies: Message = {
      ...mockComment,
      reply_count: 5,
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithReplies}
              gameId={1}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              depth={5}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    // The collapse/expand button should not be present at max depth
    expect(screen.queryByText('▼')).not.toBeInTheDocument();
    expect(screen.queryByText('▶')).not.toBeInTheDocument();
  });

  it('should use default maxDepth of 5', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={mockComment}
              gameId={1}
              characters={[]}
              controllableCharacters={[{ id: 10, name: 'Test Character', game_id: 1, owner_id: 100, avatar_url: null, character_sheet: null, is_gm: false, created_at: '', updated_at: '' }]}
              onCreateReply={vi.fn()}
              depth={4}
              // maxDepth not specified - should default to 5
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    // At depth 4 with default maxDepth 5, reply button should show
    expect(screen.getByText('Reply')).toBeInTheDocument();
  });

  it('should respect custom maxDepth prop', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={mockComment}
              gameId={1}
              characters={[]}
              controllableCharacters={[{ id: 10, name: 'Test Character', game_id: 1, owner_id: 100, avatar_url: null, character_sheet: null, is_gm: false, created_at: '', updated_at: '' }]}
              onCreateReply={vi.fn()}
              depth={3}
              maxDepth={3}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    // At depth 3 with maxDepth 3, reply button should NOT show
    expect(screen.queryByText('Reply')).not.toBeInTheDocument();
  });

  it('should display correct reply count in continue thread link', () => {
    const commentWithManyReplies: Message = {
      ...mockComment,
      reply_count: 15,
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithManyReplies}
              gameId={1}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              depth={4}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.getAllByText(/15 replies/)[0]).toBeInTheDocument();
  });

  it('should show singular "reply" for single reply', () => {
    const commentWithOneReply: Message = {
      ...mockComment,
      reply_count: 1,
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
        <ToastProvider>
        <AuthProvider>
        <AdminModeProvider>
          <ScreenshotModeProvider>
            <ThreadedComment
              comment={commentWithOneReply}
              gameId={1}
              characters={[]}
              controllableCharacters={[]}
              onCreateReply={vi.fn()}
              depth={4}
              maxDepth={5}
            />
        </ScreenshotModeProvider>
          </AdminModeProvider>
        </AuthProvider>
        </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.getAllByText(/1 reply/)[0]).toBeInTheDocument();
    expect(screen.queryByText(/1 replies/)).not.toBeInTheDocument();
  });
});
