export interface Notification {
  id: number;
  user_id: number;
  game_id?: number;
  type: string;
  title: string;
  content?: string;
  related_type?: string;
  related_id?: number;
  link_url?: string;
  /** The container this notification belongs to (e.g. 'conversation').
   * Marking the notification read clears every sibling sharing this context. */
  context_type?: string;
  context_id?: number;
  is_read: boolean;
  read_at?: string;
  created_at: string;
}

export interface NotificationListResponse {
  data: Notification[];
  pagination: {
    total: number;
    limit: number;
    offset: number;
  };
}

export interface UnreadCountResponse {
  unread_count: number;
}

export interface MarkAllReadResponse {
  marked_count: number;
}

export interface GetNotificationsParams {
  limit?: number;
  offset?: number;
  unread?: boolean;
}
