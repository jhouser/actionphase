import { Page } from '@playwright/test';
import { getWorkerIndex } from './game-helpers';

/**
 * Community Helper Functions for E2E Tests
 *
 * Communities are keyed by a UNIQUE slug, so the fixtures give each worker its
 * own copy suffixed with the worker index (see
 * backend/pkg/db/test_fixtures/e2e/00_communities.sql). Worker 0 keeps the bare
 * slug. Every spec must resolve slugs through here rather than hardcoding
 * 'midnight-ravens', or workers 1-5 would all assert against worker 0's rows.
 */

/** Base slugs of the three fixture communities, before the worker suffix. */
export const COMMUNITY_SLUGS = {
  /** Owned by TestGM. TestPlayer1 moderates. TestPlayer5 is banned (permanent). */
  RAVENS: 'midnight-ravens',
  /** Owned by TestPlayer2. TestAudience is banned (active, future-dated). */
  HARBOR: 'harbor-lights',
  /** Owned by TestPlayer3, and INACTIVE — the create-game flow must refuse it. */
  LONG_ROAD: 'the-long-road',

  // Per-spec communities (01_communities_e2e_owned.sql). Each is READ AND
  // WRITTEN by exactly one spec file, which is what makes them safe to mutate:
  // Playwright shards by file, so anything a spec changes in a SHARED community
  // can be observed mid-change by another spec running at the same time.
  /** community-moderators.spec.ts only. Owned by TestGM; TestPlayer1 moderates. */
  ROSTER: 'e2e-roster',
  /** community-moderation.spec.ts only. Bans, documents, and renames live here. */
  MODTOOLS: 'e2e-modtools',
} as const;

/** Display names, which are NOT worker-suffixed — every worker's copy shares them. */
export const COMMUNITY_NAMES = {
  RAVENS: 'Midnight Ravens',
  HARBOR: 'Harbor Lights',
  LONG_ROAD: 'The Long Road',
  ROSTER: 'E2E Roster Community',
  MODTOOLS: 'E2E Mod Tools Community',
} as const;

/**
 * Resolve a fixture community's slug for the current worker.
 *
 * @param key - Key from COMMUNITY_SLUGS
 * @returns e.g. 'midnight-ravens' on worker 0, 'midnight-ravens-w3' on worker 3
 */
export function getCommunitySlug(key: keyof typeof COMMUNITY_SLUGS): string {
  const workerIndex = getWorkerIndex();
  const base = COMMUNITY_SLUGS[key];
  return workerIndex === 0 ? base : `${base}-w${workerIndex}`;
}

/** One community as the API returns it, including the per-request fields. */
export interface CommunitySummary {
  id: number;
  name: string;
  slug: string;
  owner_user_id: number;
  owner_username: string | null;
  is_active: boolean;
  your_role?: string;
  is_banned?: boolean;
}

/**
 * Fetch a community through the browser so the HTTP-only JWT cookie is sent.
 *
 * Used to turn a slug into an id, and to read `your_role` / `is_banned` — both
 * computed per request, so they cannot be derived from fixture data alone.
 */
export async function getCommunityBySlug(
  page: Page,
  slug: string
): Promise<CommunitySummary> {
  await page.waitForLoadState('networkidle');

  return page.evaluate(async (s) => {
    const response = await fetch(`/api/v1/communities/${s}`, { credentials: 'include' });
    if (!response.ok) {
      throw new Error(`Failed to fetch community ${s}: ${response.status}`);
    }
    return response.json();
  }, slug);
}

/**
 * Resolve a fixture community's numeric id for the current worker.
 *
 * The manage UI keys its rows by user id, and the games list filters by
 * community id, so specs need the number rather than the slug.
 */
export async function getCommunityId(
  page: Page,
  key: keyof typeof COMMUNITY_SLUGS
): Promise<number> {
  const community = await getCommunityBySlug(page, getCommunitySlug(key));
  return community.id;
}

/**
 * Look up a user's id by username, through the browser's cookie session.
 *
 * Moderator and ban rows carry data-testid values keyed by user id
 * (`moderator-row-{id}`, `unban-{id}`), and fixture user ids are assigned by a
 * sequence rather than fixed — so they have to be resolved at run time.
 */
export async function getUserIdByUsername(page: Page, username: string): Promise<number> {
  await page.waitForLoadState('networkidle');

  const users = await page.evaluate(async (name) => {
    const response = await fetch(`/api/v1/auth/users/search?q=${encodeURIComponent(name)}`, {
      credentials: 'include',
    });
    if (!response.ok) {
      throw new Error(`Failed to search users for ${name}: ${response.status}`);
    }
    return response.json();
  }, username);

  const list: { id: number; username: string }[] = Array.isArray(users)
    ? users
    : (users?.users ?? []);

  // Exact match: searching "TestPlayer1" also returns "TestPlayer1_2" etc.
  const match = list.find((u) => u.username === username);
  if (!match) {
    throw new Error(`User not found: ${username}`);
  }
  return match.id;
}
