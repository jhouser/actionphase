-- Communities: tenant-like groupings that own games, a moderator roster, a
-- banlist, and their own documentation.
--
-- Membership is deliberately OPEN: there is no roster and no allowlist. Anyone
-- not banned may join or create games in a community. The banlist (added in a
-- later migration) is the entire access-control mechanism.

CREATE TABLE communities (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    banner_url TEXT,
    -- RESTRICT, not CASCADE: deleting a user must never silently orphan or
    -- destroy a community. Reassign the owner first.
    owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_communities_slug ON communities(slug);
CREATE INDEX idx_communities_owner ON communities(owner_user_id);
CREATE INDEX idx_communities_active ON communities(is_active) WHERE is_active;

-- Moderators. The OWNER IS NOT A ROW HERE -- ownership lives in
-- communities.owner_user_id and the permission helpers treat owner as a
-- superset of moderator. That two-tier split is what makes "moderators can do
-- everything except add moderators" a clean check rather than a role enum with
-- an implicit ordering.
CREATE TABLE community_moderators (
    id SERIAL PRIMARY KEY,
    community_id INTEGER NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(community_id, user_id)
);

CREATE INDEX idx_community_moderators_community ON community_moderators(community_id);
CREATE INDEX idx_community_moderators_user ON community_moderators(user_id);

-- Game association.
--
-- 🔴 THIS COLUMN IS NULLABLE ON PURPOSE AND MUST STAY THAT WAY.
-- Games created before Communities existed are grandfathered in with no
-- community. New games require one, but that rule is enforced in the
-- APPLICATION create path, not by a NOT NULL constraint, precisely so legacy
-- rows remain valid. Adding NOT NULL here would break every pre-existing game.
-- Ban enforcement must likewise treat community_id IS NULL as "never blocked".
ALTER TABLE games ADD COLUMN community_id INTEGER
    REFERENCES communities(id) ON DELETE RESTRICT;

CREATE INDEX idx_games_community ON games(community_id);
