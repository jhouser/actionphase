-- name: CreateCommunity :one
INSERT INTO communities (
    name, slug, description, banner_url, owner_user_id
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetCommunityByID :one
SELECT * FROM communities
WHERE id = $1;

-- name: GetCommunityBySlug :one
SELECT * FROM communities
WHERE slug = $1;

-- name: ListCommunities :many
-- Admin listing: every community, active or not, with the owner's username so
-- the admin table needs no follow-up request per row.
SELECT
    c.*,
    u.username AS owner_username
FROM communities c
JOIN users u ON u.id = c.owner_user_id
ORDER BY c.name ASC;

-- name: ListActiveCommunities :many
-- Public listing: only active communities.
SELECT
    c.*,
    u.username AS owner_username
FROM communities c
JOIN users u ON u.id = c.owner_user_id
WHERE c.is_active
ORDER BY c.name ASC;

-- name: UpdateCommunity :one
-- Partial update: each COALESCE leaves the column untouched when its argument
-- is NULL, so a PATCH body can carry any subset of fields.
--
-- description is nullable and must be CLEARABLE, which a bare COALESCE cannot
-- express -- it would collapse "clear it" into "leave it alone" and make the
-- blurb write-once-then-permanent. The service maps an empty string to SQL
-- NULL before it reaches here, matching how UpdateGame treats a blank
-- description, so "" arrives as a genuine clear rather than an empty value.
--
-- banner_url is deliberately ABSENT. Banners are uploaded objects, not typed-in
-- URLs: the column and the stored file have to stay in sync, so it is written
-- only by a dedicated upload/delete path (see UpdateGameBannerURL for the
-- pattern). Editing it through a general PATCH would let a row point at a file
-- nothing owns and leak the old object.
--
-- slug is deliberately absent. Slugs are immutable after creation because they
-- appear in URLs that communities will have shared externally.
UPDATE communities
SET
    name          = COALESCE(sqlc.narg('name'), name),
    description   = CASE
                        WHEN sqlc.arg('set_description')::boolean
                            THEN sqlc.narg('description')
                        ELSE description
                    END,
    owner_user_id = COALESCE(sqlc.narg('owner_user_id'), owner_user_id),
    is_active     = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at    = NOW()
WHERE id = sqlc.arg('id')
RETURNING *;

-- Moderators -----------------------------------------------------------------

-- name: AddCommunityModerator :one
INSERT INTO community_moderators (
    community_id, user_id, granted_by_user_id
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: RemoveCommunityModerator :exec
DELETE FROM community_moderators
WHERE community_id = $1 AND user_id = $2;

-- name: ListCommunityModerators :many
SELECT
    cm.*,
    u.username    AS username,
    u.display_name AS display_name,
    u.avatar_url  AS avatar_url,
    g.username    AS granted_by_username
FROM community_moderators cm
JOIN users u ON u.id = cm.user_id
LEFT JOIN users g ON g.id = cm.granted_by_user_id
WHERE cm.community_id = $1
ORDER BY u.username ASC;

-- name: GetCommunityRole :one
-- Resolves a user's standing in one round-trip: 'owner' beats 'moderator',
-- and '' means neither.
SELECT CASE
    WHEN EXISTS(
        SELECT 1 FROM communities c
        WHERE c.id = sqlc.arg('community_id') AND c.owner_user_id = sqlc.arg('user_id')
    ) THEN 'owner'
    WHEN EXISTS(
        SELECT 1 FROM community_moderators cm
        WHERE cm.community_id = sqlc.arg('community_id') AND cm.user_id = sqlc.arg('user_id')
    ) THEN 'moderator'
    ELSE ''
END AS role;
