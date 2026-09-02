-- Community documents (req 7, 8).
--
-- Two audiences read these: moderators, who see drafts, and everyone else, who
-- sees only published documents. That split is expressed as SEPARATE queries
-- rather than one query with a boolean flag, so a caller cannot accidentally
-- expose drafts by passing the wrong argument -- the handler must choose the
-- privileged query explicitly.

-- name: CreateCommunityDocument :one
INSERT INTO community_documents (
    community_id, title, content, status, sort_order, created_by_user_id
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetCommunityDocument :one
-- Scoped by community as well as id: the id alone would resolve a document
-- belonging to a DIFFERENT community than the one in the request path, letting
-- a moderator of community A read A's URL into B's draft. The pairing makes
-- that a no-rows 404 rather than a permission check the handler has to
-- remember.
SELECT * FROM community_documents
WHERE id = $1 AND community_id = $2;

-- name: ListCommunityDocuments :many
-- Moderator listing: drafts included.
--
-- Ordered by sort_order then id. The id tiebreak is not decoration: sort_order
-- defaults to 0, so a community that never sets it would otherwise get an
-- arbitrary, unstable order that changes between requests.
SELECT * FROM community_documents
WHERE community_id = $1
ORDER BY sort_order ASC, id ASC;

-- name: ListPublishedCommunityDocuments :many
-- Public listing: published only. Backs the community page for ordinary
-- visitors and the community section of the game Info tab.
SELECT * FROM community_documents
WHERE community_id = $1 AND status = 'published'
ORDER BY sort_order ASC, id ASC;

-- name: ListPublishedCommunityDocumentsForGame :many
-- The Info tab's list, resolved from a GAME rather than a community.
--
-- Inner join through games.community_id, so a LEGACY GAME (community_id IS
-- NULL) yields no rows and the Info tab simply renders no community section.
-- That grandfathering lives here rather than in the handler for the same reason
-- IsUserBannedFromGameCommunity does: one forgotten NULL check at a call site
-- would be a crash or an empty section for a reason nobody could explain.
--
-- Returns documents ONLY. The community's name and slug used to ride along on
-- every row, back when GetGameWithDetails did not join communities and the tab
-- had no other way to name the section. It does join them now, so the game
-- payload names its own community and duplicating that identity onto each
-- document row would give the Info tab two sources for one fact -- and leave
-- the section unable to name a community that has published nothing.
SELECT d.*
FROM community_documents d
JOIN games g ON g.community_id = d.community_id
WHERE g.id = $1 AND d.status = 'published'
ORDER BY d.sort_order ASC, d.id ASC;

-- name: UpdateCommunityDocument :one
-- Partial update: each COALESCE leaves its column untouched when the argument
-- is NULL, so a PATCH body may carry any subset of fields.
--
-- content is NOT nullable in the table, so a bare COALESCE is correct here --
-- unlike communities.description, there is no "clear it" state to distinguish
-- from "leave it alone". An empty document body is a blank page, not an absent
-- one, and it is reachable by sending "".
--
-- Scoped by community_id for the same reason GetCommunityDocument is: an id
-- from another community must miss rather than write.
UPDATE community_documents
SET
    title      = COALESCE(sqlc.narg('title'), title),
    content    = COALESCE(sqlc.narg('content'), content),
    status     = COALESCE(sqlc.narg('status'), status),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND community_id = sqlc.arg('community_id')
RETURNING *;

-- name: DeleteCommunityDocument :exec
-- Community-scoped, matching the read and update paths.
DELETE FROM community_documents
WHERE id = $1 AND community_id = $2;
