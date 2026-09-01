-- Community bans: the driver for the whole Communities feature.
--
-- Three real communities share this deployment, and each needs to exclude a
-- user from ITS games without touching the others. Membership is otherwise
-- open, so this banlist is the entire access-control mechanism -- negative
-- space only.

CREATE TABLE community_bans (
    id SERIAL PRIMARY KEY,
    community_id INTEGER NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT,
    -- SET NULL, not CASCADE: deleting the moderator who issued a ban must not
    -- lift the ban. The ban belongs to the community, not to its author.
    banned_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- NULL = permanent. Mirrors ip_bans, which already supports temporary bans;
    -- cheaper to carry from day one than to retrofit onto a live banlist.
    -- Every enforcement path must test (expires_at IS NULL OR expires_at > NOW()).
    expires_at TIMESTAMPTZ,
    -- One live ban per user per community. Re-banning an already-banned user
    -- updates the existing row rather than stacking a second one.
    UNIQUE(community_id, user_id)
);

CREATE INDEX idx_community_bans_lookup ON community_bans(community_id, user_id);
-- Supports "which communities is this user banned from", used when filtering
-- the community picker on game creation.
CREATE INDEX idx_community_bans_user ON community_bans(user_id);

-- Audit log, deliberately SEPARATE from community_bans.
--
-- Lifting a ban DELETES its row, but the history has to survive that -- three
-- communities on one site will have disputes about who banned whom, and this is
-- impossible to reconstruct after the fact. Rows here are append-only: nothing
-- in the application updates or deletes them.
CREATE TABLE community_ban_events (
    id SERIAL PRIMARY KEY,
    community_id INTEGER NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    target_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    -- 'banned' | 'unbanned' | 'modified'. Not a CHECK constraint or enum: the
    -- canonical list lives in core.ValidBanEventActions, matching how phase
    -- types and character statuses are handled elsewhere in this schema.
    action VARCHAR(20) NOT NULL,
    -- Snapshotted at event time, NOT a reference to community_bans: the ban row
    -- is gone by the time an 'unbanned' event is read, and a 'modified' event
    -- must show what the values were then, not what they are now.
    reason TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ban_events_community ON community_ban_events(community_id, created_at DESC);
