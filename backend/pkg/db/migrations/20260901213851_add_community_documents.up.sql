-- Community documents: a community's own rules, etiquette, and reference pages
-- (req 7), surfaced on the community page and linked from the Info tab of every
-- game that community owns (req 8).
--
-- Mirrors `handouts` deliberately -- same draft/published gate, same markdown
-- content rendered through MarkdownPreview -- minus the comment thread. A
-- community's rules are an announcement, not a discussion; games already have
-- handouts for the discussable kind.
--
-- Addressed by ID. A per-community slug was considered and rejected: titles are
-- edited as rules evolve, so a slug frozen at creation would strand a renamed
-- document at its old URL, and it would need collision handling plus a fallback
-- for titles with no Latin characters.

CREATE TABLE community_documents (
    id SERIAL PRIMARY KEY,
    community_id INTEGER NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    -- 'draft' | 'published'. A plain VARCHAR with no CHECK constraint, matching
    -- handouts.status and the rest of this schema: the canonical list lives in
    -- core.ValidDocumentStatuses. Drafts let a moderator write rules over
    -- several sittings before they bind anyone.
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    -- Display order, lowest first. A community's rules have a deliberate
    -- reading order that neither title nor creation date expresses -- "Read
    -- this first" is rarely written first and rarely sorts first.
    sort_order INTEGER NOT NULL DEFAULT 0,
    -- SET NULL, not CASCADE: deleting the moderator who wrote a document must
    -- not delete the community's rules. The document belongs to the community.
    created_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Covers the two reads that matter: the moderator's full list and the public
-- published list, both ordered for display. Ordering columns are in the index
-- so neither read needs a sort.
CREATE INDEX idx_community_documents_listing
    ON community_documents(community_id, status, sort_order, id);
