-- Assign every demo game to a community.
--
-- Done as a backfill rather than a community_id column on each INSERT: the
-- demo game fixtures are spread across several files and adding the column to
-- each one invites a miss, which would silently leave a game NULL and quietly
-- untested. Assigning here means "every demo game has a community" is one
-- statement that is true by construction.
--
-- Distribution is by GM so it is stable and legible rather than arbitrary:
-- a GM's games sit in their own community, which is how real usage looks.
-- The inactive community (the-long-road) intentionally receives NO games --
-- it exists to be refused by the create-game flow, not to host anything.

BEGIN;

DO $$
DECLARE
  ravens_id  INTEGER;
  harbor_id  INTEGER;
  gm_id      INTEGER;
  unassigned INTEGER;
BEGIN
  SELECT id INTO ravens_id FROM communities WHERE slug = 'midnight-ravens';
  SELECT id INTO harbor_id FROM communities WHERE slug = 'harbor-lights';
  SELECT id INTO gm_id     FROM users WHERE email = 'test_gm@example.com';

  -- TestGM's games -> Midnight Ravens (that community's owner is TestGM).
  UPDATE games SET community_id = ravens_id
  WHERE community_id IS NULL AND gm_user_id = gm_id;

  -- Anything else demo-owned -> Harbor Lights.
  UPDATE games SET community_id = harbor_id
  WHERE community_id IS NULL
    AND gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com');

  -- Every demo game is GM-run, so the rule above would leave Harbor Lights
  -- empty and the demo would show a multi-community install where only one
  -- community has anything in it. Move the two recruiting games across: a
  -- second populated community is what makes "banned here, fine there"
  -- visible, and recruiting games are the ones the apply flow acts on.
  UPDATE games SET community_id = harbor_id
  WHERE community_id = ravens_id
    AND state = 'recruitment';

  -- Fail loudly rather than leaving a silent gap. A demo game with no
  -- community would exercise the grandfathering path instead of the ban paths,
  -- which is the opposite of what this dataset is for -- and it would do so
  -- without anyone noticing.
  SELECT COUNT(*) INTO unassigned
  FROM games
  WHERE community_id IS NULL
    AND gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com');

  IF unassigned > 0 THEN
    RAISE EXCEPTION 'demo backfill left % game(s) with no community', unassigned;
  END IF;
END $$;

COMMIT;
