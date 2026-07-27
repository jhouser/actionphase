import { BaseApiClient } from './client';
import type {
  Message,
  CreatePostRequest,
  CreateCommentRequest,
  UpdateCommentRequest,
  GetPostsParams,
  ReadMarker,
  PostUnreadInfo,
  MarkPostReadRequest,
  PostUnreadComments,
  ManualCommentReads,
  PaginatedCommentsResponse,
  RecentCommentsResponse,
  CommentWithParent,
  MessageThreadContext
} from '../../types/messages';
import { COMMENT_MAX_DEPTH } from '@/config/comments';

/**
 * Messages API client
 * Handles posts and comments in the common room
 */
export class MessagesApi extends BaseApiClient {
  // Post endpoints
  async createPost(gameId: number, data: CreatePostRequest) {
    return this.client.post<Message>(`/api/v1/games/${gameId}/posts`, data);
  }

  async getGamePosts(gameId: number, params?: GetPostsParams) {
    const queryParams = new URLSearchParams();
    if (params?.phase_id) queryParams.append('phase_id', params.phase_id.toString());
    if (params?.limit) queryParams.append('limit', params.limit.toString());
    if (params?.offset) queryParams.append('offset', params.offset.toString());

    const queryString = queryParams.toString();
    const url = `/api/v1/games/${gameId}/posts${queryString ? `?${queryString}` : ''}`;
    return this.client.get<Message[]>(url);
  }

  async updatePost(gameId: number, postId: number, content: string) {
    return this.client.patch<Message>(`/api/v1/games/${gameId}/posts/${postId}`, { content });
  }

  // Comment endpoints
  async createComment(gameId: number, postId: number, data: CreateCommentRequest) {
    return this.client.post<Message>(`/api/v1/games/${gameId}/posts/${postId}/comments`, data);
  }

  async getPostComments(gameId: number, postId: number) {
    return this.client.get<Message[]>(`/api/v1/games/${gameId}/posts/${postId}/comments`);
  }

  /**
   * Get paginated top-level comments with all nested replies (up to max_depth)
   * Returns flat array with depth field for tree building on frontend
   */
  async getPostCommentsWithThreads(
    gameId: number,
    postId: number,
    limit: number = 15,
    offset: number = 0,
    maxDepth: number = COMMENT_MAX_DEPTH
  ) {
    const queryParams = new URLSearchParams();
    queryParams.append('limit', limit.toString());
    queryParams.append('offset', offset.toString());
    queryParams.append('max_depth', maxDepth.toString());

    const url = `/api/v1/games/${gameId}/posts/${postId}/comments-with-threads?${queryParams.toString()}`;
    return this.client.get<PaginatedCommentsResponse>(url);
  }

  async updateComment(gameId: number, postId: number, commentId: number, data: UpdateCommentRequest) {
    return this.client.patch<Message>(`/api/v1/games/${gameId}/posts/${postId}/comments/${commentId}`, data);
  }

  async deleteComment(gameId: number, postId: number, commentId: number) {
    return this.client.delete<{ message: string; id: number }>(`/api/v1/games/${gameId}/posts/${postId}/comments/${commentId}`);
  }

  // Get a single message by ID (for deep linking to nested comments)
  async getMessage(gameId: number, messageId: number) {
    return this.client.get<Message>(`/api/v1/games/${gameId}/messages/${messageId}`);
  }

  // Get a message plus a bounded slice of its ancestor chain (target + up to
  // maxParents nearest ancestors) and the true root post ID, in a single request.
  // Replaces the per-level getMessage waterfall for deep-linked nested comments.
  // chain is ordered parent-to-child (nearest included ancestor → target).
  async getMessageThreadContext(gameId: number, messageId: number, maxParents?: number) {
    const query = maxParents !== undefined ? `?max_parents=${maxParents}` : '';
    return this.client.get<MessageThreadContext>(`/api/v1/games/${gameId}/messages/${messageId}/thread-context${query}`);
  }

  // Read tracking endpoints
  async markPostAsRead(gameId: number, postId: number, data: MarkPostReadRequest = {}) {
    return this.client.post<ReadMarker>(`/api/v1/games/${gameId}/posts/${postId}/mark-read`, data);
  }

  async getReadMarkers(gameId: number) {
    return this.client.get<ReadMarker[]>(`/api/v1/games/${gameId}/read-markers`);
  }

  async getPostsUnreadInfo(gameId: number) {
    return this.client.get<PostUnreadInfo[]>(`/api/v1/games/${gameId}/posts-unread-info`);
  }

  async getUnreadCommentIDs(gameId: number) {
    return this.client.get<PostUnreadComments[]>(`/api/v1/games/${gameId}/unread-comment-ids`);
  }

  // Manual read tracking endpoints
  async toggleCommentRead(gameId: number, postId: number, commentId: number, read: boolean) {
    return this.client.post<void>(
      `/api/v1/games/${gameId}/posts/${postId}/comments/${commentId}/toggle-read`,
      { read }
    );
  }

  async getManualReadCommentIDs(gameId: number) {
    return this.client.get<ManualCommentReads[]>(`/api/v1/games/${gameId}/manual-read-comment-ids`);
  }

  async markAllCommentsRead(gameId: number, phaseId: number) {
    return this.client.post<void>(`/api/v1/games/${gameId}/phases/${phaseId}/mark-all-comments-read`);
  }

  // Recent comments (New Comments view)
  async getRecentComments(gameId: number, limit: number = 20, offset: number = 0) {
    const queryParams = new URLSearchParams();
    queryParams.append('limit', limit.toString());
    queryParams.append('offset', offset.toString());

    const url = `/api/v1/games/${gameId}/comments/recent?${queryParams.toString()}`;
    const response = await this.client.get<RecentCommentsResponse>(url);

    // Transform the response to flatten the parent object
    return {
      data: {
        ...response.data,
        comments: response.data.comments.map((comment: CommentWithParent) => ({
          ...comment,
          parent_content: comment.parent?.content,
          parent_created_at: comment.parent?.created_at,
          parent_deleted_at: comment.parent?.deleted_at,
          parent_is_deleted: comment.parent?.is_deleted,
          parent_message_type: comment.parent?.message_type,
          parent_author_username: comment.parent?.author_username,
          parent_character_name: comment.parent?.character_name,
          parent_character_avatar_url: comment.parent?.character_avatar_url,
        })),
      },
    };
  }

  async getTotalCommentCount(gameId: number) {
    const response = await this.client.get<{ total: number }>(`/api/v1/games/${gameId}/comments/count`);
    return response.data.total;
  }

  // Draft post endpoints (GM only, phase-scoped)

  async getDraftPost(phaseId: number) {
    return this.client.get<Message>(`/api/v1/phases/${phaseId}/draft-post`);
  }

  async createDraftPost(phaseId: number, characterId: number, content: string) {
    return this.client.post<Message>(`/api/v1/phases/${phaseId}/draft-post`, {
      character_id: characterId,
      content,
    });
  }

  async updateDraftPost(phaseId: number, content: string) {
    return this.client.put<Message>(`/api/v1/phases/${phaseId}/draft-post`, { content });
  }

  async deleteDraftPost(phaseId: number) {
    return this.client.delete<{ message: string }>(`/api/v1/phases/${phaseId}/draft-post`);
  }
}
