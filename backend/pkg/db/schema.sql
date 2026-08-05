-- ActionPhase Database Schema
-- This file represents the current database schema for sqlc generation

-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    is_admin BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    display_name VARCHAR(255),
    bio TEXT,
    avatar_url VARCHAR(500),
    timezone VARCHAR(50) DEFAULT 'UTC',
    email_notifications BOOLEAN DEFAULT TRUE,
    high_contrast BOOLEAN DEFAULT FALSE,
    is_banned BOOLEAN DEFAULT FALSE NOT NULL,
    banned_at TIMESTAMP WITHOUT TIME ZONE,
    banned_by_user_id INTEGER REFERENCES users(id),
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_change_pending TEXT,
    password_changed_at TIMESTAMP WITH TIME ZONE,
    username_changed_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    deletion_scheduled_for TIMESTAMP WITH TIME ZONE,
    pending_approval BOOLEAN NOT NULL DEFAULT FALSE,
    pending_approval_since TIMESTAMPTZ
);

-- Password Reset Tokens
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE
);

-- Email Verification Tokens
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE
);

-- Registration Attempts (for bot prevention tracking)
CREATE TABLE registration_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT,
    honeypot_triggered BOOLEAN NOT NULL DEFAULT FALSE,
    captcha_passed BOOLEAN NOT NULL DEFAULT FALSE,
    blocked_reason VARCHAR(100),
    successful BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Sessions table
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    data TEXT NOT NULL,
    expires TIMESTAMP WITH TIME ZONE,
    ip_address VARCHAR(45),
    user_agent TEXT,
    fingerprint VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- IP Bans
CREATE TABLE ip_bans (
    id SERIAL PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL UNIQUE,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason TEXT,
    expires_at TIMESTAMPTZ,
    banned_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

-- Device Fingerprint Bans
CREATE TABLE fingerprint_bans (
    id SERIAL PRIMARY KEY,
    fingerprint VARCHAR(255) NOT NULL UNIQUE,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason TEXT,
    banned_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

-- Games table
CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    gm_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50) DEFAULT 'setup',
    genre VARCHAR(100),
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    recruitment_deadline TIMESTAMP WITH TIME ZONE,
    max_players INTEGER DEFAULT 6,
    is_public BOOLEAN DEFAULT TRUE,
    is_anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    auto_accept_audience BOOLEAN DEFAULT FALSE NOT NULL,
    allow_group_conversations BOOLEAN NOT NULL DEFAULT TRUE,
    portrait_avatars BOOLEAN NOT NULL DEFAULT FALSE,
    banner_url TEXT,
    common_room_open_day   SMALLINT,
    common_room_open_time  TIME,
    common_room_close_day  SMALLINT,
    common_room_close_time TIME,
    schedule_timezone      VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Game participants
CREATE TABLE game_participants (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    removed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    removed_by_user_id INTEGER DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    is_former_player BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(game_id, user_id)
);

-- Game applications
CREATE TABLE game_applications (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    message TEXT,
    status VARCHAR(50) DEFAULT 'pending',
    reviewed_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP WITH TIME ZONE,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(game_id, user_id)
);

-- Game logs
CREATE TABLE game_logs (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Characters
CREATE TABLE characters (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    character_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    avatar_url TEXT NULL,
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    original_owner_user_id INTEGER DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Character data (modular system)
CREATE TABLE character_data (
    id SERIAL PRIMARY KEY,
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    module_type VARCHAR(50) NOT NULL,
    field_name VARCHAR(100) NOT NULL,
    field_value TEXT,
    field_type VARCHAR(50) DEFAULT 'text',
    is_public BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- NPC assignments
CREATE TABLE npc_assignments (
    id SERIAL PRIMARY KEY,
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    assigned_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Game phases
CREATE TABLE game_phases (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    phase_type VARCHAR(20) NOT NULL CHECK (phase_type IN ('common_room', 'action')),
    phase_number INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT 'Untitled Phase',
    description TEXT,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    deadline TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT FALSE,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    activated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(game_id, phase_number)
);

-- Action submissions
CREATE TABLE action_submissions (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phase_id INTEGER NOT NULL REFERENCES game_phases(id) ON DELETE CASCADE,
    character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    is_draft BOOLEAN DEFAULT TRUE,
    submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(game_id, user_id, phase_id)
);

-- Action results
CREATE TABLE action_results (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phase_id INTEGER NOT NULL REFERENCES game_phases(id) ON DELETE CASCADE,
    character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,
    action_submission_id INTEGER REFERENCES action_submissions(id) ON DELETE CASCADE,
    gm_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_published BOOLEAN DEFAULT FALSE,
    sent_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Draft character updates tied to action results
CREATE TABLE action_result_character_updates (
    id SERIAL PRIMARY KEY,
    action_result_id INTEGER NOT NULL REFERENCES action_results(id) ON DELETE CASCADE,
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    module_type VARCHAR(50) NOT NULL,
    field_name VARCHAR(100) NOT NULL,
    field_value TEXT,
    field_type VARCHAR(20) NOT NULL DEFAULT 'text',
    operation VARCHAR(20) NOT NULL DEFAULT 'upsert',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Phase transitions log
CREATE TABLE phase_transitions (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    from_phase_id INTEGER REFERENCES game_phases(id) ON DELETE SET NULL,
    to_phase_id INTEGER NOT NULL REFERENCES game_phases(id) ON DELETE CASCADE,
    initiated_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Communication tables
CREATE TABLE conversations (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    conversation_type VARCHAR(20) NOT NULL DEFAULT 'direct',
    title VARCHAR(255),
    created_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CHECK (conversation_type IN ('direct', 'group'))
);

CREATE TABLE conversation_participants (
    id SERIAL PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(conversation_id, user_id, character_id)
);

CREATE TABLE private_messages (
    id SERIAL PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sender_character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    is_deleted BOOLEAN DEFAULT FALSE,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    edited_at TIMESTAMP WITH TIME ZONE,
    edit_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE conversation_reads (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    last_read_message_id INTEGER REFERENCES private_messages(id) ON DELETE SET NULL,
    last_read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, conversation_id)
);

-- Threads (for common room discussions)
CREATE TABLE threads (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    phase_id INTEGER REFERENCES game_phases(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_pinned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE thread_posts (
    id SERIAL PRIMARY KEY,
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    parent_post_id INTEGER REFERENCES thread_posts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Notifications
CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    related_type VARCHAR(50),
    related_id INTEGER,
    link_url VARCHAR(500),
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_game_phases_game_id ON game_phases(game_id);
CREATE INDEX idx_game_phases_active ON game_phases(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_game_phases_published ON game_phases(game_id, is_published) WHERE is_published = TRUE AND phase_type = 'action';
CREATE INDEX idx_action_submissions_phase_id ON action_submissions(phase_id);
CREATE INDEX idx_action_submissions_user_id ON action_submissions(user_id);
CREATE INDEX idx_action_results_phase_id ON action_results(phase_id);
CREATE INDEX idx_action_results_user_id ON action_results(user_id);
CREATE INDEX idx_action_results_character_id ON action_results(character_id);
CREATE INDEX idx_action_results_submission_id ON action_results(action_submission_id);
CREATE INDEX idx_phase_transitions_game_id ON phase_transitions(game_id);
CREATE INDEX idx_game_participants_game_id ON game_participants(game_id);
CREATE INDEX idx_game_participants_user_id ON game_participants(user_id);
CREATE INDEX idx_game_participants_removed_at ON game_participants(game_id, removed_at) WHERE removed_at IS NULL;
CREATE INDEX idx_game_participants_role ON game_participants(game_id, role);
CREATE INDEX idx_characters_game_id ON characters(game_id);
CREATE INDEX idx_characters_active ON characters(game_id, is_active);
CREATE INDEX idx_characters_audience ON characters(game_id, character_type) WHERE character_type = 'npc_audience';
CREATE INDEX idx_character_data_character_id ON character_data(character_id);
CREATE INDEX idx_conversations_game_id ON conversations(game_id);
CREATE INDEX idx_conversation_participants_conversation_id ON conversation_participants(conversation_id);
CREATE INDEX idx_conversation_participants_user_id ON conversation_participants(user_id);
CREATE INDEX idx_private_messages_conversation_id ON private_messages(conversation_id);
CREATE INDEX idx_conversation_reads_user_conversation ON conversation_reads(user_id, conversation_id);
CREATE INDEX idx_conversation_reads_conversation ON conversation_reads(conversation_id);

-- Messages System (Common Room and Private Messages)
CREATE TYPE message_visibility AS ENUM ('game', 'private');
CREATE TYPE message_type AS ENUM ('post', 'comment', 'private_message');

CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    phase_id INTEGER REFERENCES game_phases(id) ON DELETE SET NULL,
    author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    message_type message_type NOT NULL DEFAULT 'post',
    parent_id INTEGER REFERENCES messages(id) ON DELETE CASCADE,
    thread_depth INTEGER NOT NULL DEFAULT 0,
    visibility message_visibility NOT NULL DEFAULT 'game',
    mentioned_character_ids INTEGER[] NOT NULL DEFAULT '{}',
    is_edited BOOLEAN NOT NULL DEFAULT false,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    is_draft BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    deleted_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    edited_at TIMESTAMPTZ,
    edit_count INTEGER NOT NULL DEFAULT 0,
    -- Avatar the authoring character had when this message was created. Written
    -- once at insert, never updated on edit. NULL for messages predating this
    -- column; readers COALESCE to the live characters.avatar_url.
    character_avatar_url_at_post VARCHAR(500)
);

CREATE TABLE message_recipients (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(message_id, recipient_id)
);

CREATE TABLE message_reactions (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(message_id, user_id, reaction_type)
);

CREATE INDEX idx_messages_game_id ON messages(game_id);
CREATE INDEX idx_messages_phase_id ON messages(phase_id);
CREATE INDEX idx_messages_is_draft ON messages(game_id, is_draft) WHERE is_draft = true;
CREATE INDEX idx_messages_author_id ON messages(author_id);
CREATE INDEX idx_messages_parent_id ON messages(parent_id);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_messages_game_phase ON messages(game_id, phase_id);
CREATE INDEX idx_messages_thread ON messages(game_id, parent_id, created_at) WHERE is_deleted = false;
CREATE INDEX idx_messages_mentioned_characters ON messages USING GIN (mentioned_character_ids);
CREATE INDEX idx_message_recipients_message_id ON message_recipients(message_id);
CREATE INDEX idx_message_recipients_recipient_id ON message_recipients(recipient_id);
CREATE INDEX idx_message_recipients_unread ON message_recipients(recipient_id, is_read) WHERE is_read = false;
CREATE INDEX idx_message_reactions_message_id ON message_reactions(message_id);
CREATE INDEX idx_message_reactions_user_id ON message_reactions(user_id);

-- Common Room Read Tracking
CREATE TABLE user_common_room_reads (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    post_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    last_read_comment_id INTEGER REFERENCES messages(id) ON DELETE SET NULL,
    last_read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

CREATE INDEX idx_user_common_room_reads_user_game ON user_common_room_reads(user_id, game_id);
CREATE INDEX idx_user_common_room_reads_post ON user_common_room_reads(post_id);
CREATE INDEX idx_user_common_room_reads_updated ON user_common_room_reads(updated_at DESC);

-- User Preferences table
CREATE TABLE user_preferences (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  UNIQUE(user_id)
);

CREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);
CREATE INDEX idx_user_preferences_jsonb ON user_preferences USING GIN (preferences);

-- Handouts table for GM informational documents
CREATE TABLE handouts (
    id SERIAL PRIMARY KEY,
    game_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'draft' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT fk_handouts_game
        FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
);

CREATE INDEX idx_handouts_game_id ON handouts(game_id);
CREATE INDEX idx_handouts_status ON handouts(game_id, status);

-- Handout comments table for GM-only comments on handouts
CREATE TABLE handout_comments (
    id SERIAL PRIMARY KEY,
    handout_id INT NOT NULL,
    user_id INT NOT NULL,
    parent_comment_id INT DEFAULT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    edited_at TIMESTAMPTZ DEFAULT NULL,
    edit_count INT DEFAULT 0 NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    deleted_by_user_id INT DEFAULT NULL,

    CONSTRAINT fk_handout_comments_handout
        FOREIGN KEY (handout_id) REFERENCES handouts(id) ON DELETE CASCADE,
    CONSTRAINT fk_handout_comments_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_handout_comments_parent
        FOREIGN KEY (parent_comment_id) REFERENCES handout_comments(id) ON DELETE CASCADE,
    CONSTRAINT fk_handout_comments_deleted_by
        FOREIGN KEY (deleted_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_handout_comments_handout ON handout_comments(handout_id);
CREATE INDEX idx_handout_comments_parent ON handout_comments(parent_comment_id);

-- Game deadlines table
-- Allows GMs to create arbitrary deadlines separate from phase transitions
CREATE TABLE game_deadlines (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    title VARCHAR(100) NOT NULL,
    description TEXT,
    deadline TIMESTAMPTZ NOT NULL,

    -- Metadata
    created_by_user_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Soft delete for history
    deleted_at TIMESTAMPTZ
);

-- Index for querying active deadlines for a specific game
CREATE INDEX idx_game_deadlines_game_active
    ON game_deadlines(game_id, deadline)
    WHERE deleted_at IS NULL;

-- Index for querying deadlines by timestamp
CREATE INDEX idx_game_deadlines_deadline
    ON game_deadlines(deadline)
    WHERE deleted_at IS NULL;

-- Common Room Polling System
-- Enables GMs to create polls for player voting and consensus-building

-- Main polls table
CREATE TABLE common_room_polls (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    phase_id INTEGER REFERENCES game_phases(id) ON DELETE CASCADE,

    -- Creator information
    created_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by_character_id INTEGER REFERENCES characters(id) ON DELETE SET NULL,

    -- Poll content
    question VARCHAR(500) NOT NULL,
    description TEXT,

    -- Configuration
    deadline TIMESTAMPTZ NOT NULL,
    show_individual_votes BOOLEAN DEFAULT FALSE,
    allow_other_option BOOLEAN DEFAULT TRUE,
    hide_results_from_players BOOLEAN NOT NULL DEFAULT FALSE,
    allow_audience_voting BOOLEAN NOT NULL DEFAULT FALSE,
    show_running_totals_to_players BOOLEAN NOT NULL DEFAULT FALSE,

    -- Metadata
    is_deleted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT poll_hidden_results_excludes_individual_votes
        CHECK (NOT (hide_results_from_players AND COALESCE(show_individual_votes, FALSE))),
    CONSTRAINT poll_hidden_results_excludes_running_totals
        CHECK (NOT (hide_results_from_players AND show_running_totals_to_players))
);

-- Poll options table
CREATE TABLE poll_options (
    id SERIAL PRIMARY KEY,
    poll_id INTEGER NOT NULL REFERENCES common_room_polls(id) ON DELETE CASCADE,
    option_text VARCHAR(200) NOT NULL,
    display_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Ensure unique ordering within a poll
    UNIQUE (poll_id, display_order)
);

-- Poll votes table
CREATE TABLE poll_votes (
    id SERIAL PRIMARY KEY,
    poll_id INTEGER NOT NULL REFERENCES common_room_polls(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Vote data
    selected_option_id INTEGER REFERENCES poll_options(id) ON DELETE CASCADE,
    other_response TEXT,

    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    UNIQUE (poll_id, user_id),

    -- Must select an option OR provide "other" response
    CHECK (
        (selected_option_id IS NOT NULL AND other_response IS NULL) OR
        (selected_option_id IS NULL AND other_response IS NOT NULL)
    )
);

-- Indexes for performance

-- Query polls by game and phase
CREATE INDEX idx_polls_game_phase
    ON common_room_polls(game_id, phase_id)
    WHERE is_deleted = FALSE;

-- Query active polls by deadline
CREATE INDEX idx_polls_deadline
    ON common_room_polls(deadline)
    WHERE is_deleted = FALSE;

-- Query votes by poll
CREATE INDEX idx_votes_poll
    ON poll_votes(poll_id);

-- Query user's votes
CREATE INDEX idx_votes_user
    ON poll_votes(user_id);

-- Query poll options by poll
CREATE INDEX idx_options_poll
    ON poll_options(poll_id, display_order);

-- Manual per-comment read tracking (for users in "manual" read mode)
CREATE TABLE user_comment_reads (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    post_id    INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    game_id    INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, comment_id)
);

CREATE INDEX idx_user_comment_reads_user_game ON user_comment_reads(user_id, game_id);
CREATE INDEX idx_user_comment_reads_user_post ON user_comment_reads(user_id, post_id);
CREATE INDEX idx_user_comment_reads_comment   ON user_comment_reads(comment_id);

-- Discord OAuth account linking
CREATE TABLE user_discord_accounts (
  id               SERIAL PRIMARY KEY,
  user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  discord_user_id  TEXT NOT NULL,
  discord_username TEXT NOT NULL,
  access_token     TEXT NOT NULL,
  refresh_token    TEXT,
  token_expires_at TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id),
  UNIQUE(discord_user_id)
);
