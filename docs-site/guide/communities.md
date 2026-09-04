# Communities

A community is a group that runs its own games. Each one has its own guidelines, its own moderators, and its own ban list — so a group can set expectations for its games without those rules applying to the whole site.

Browse them from **Communities** in the top navigation.

## What a Community Page Shows

- **About** — The community's description
- **Documents** — Published guidelines, house rules, or anything else the moderators have written
- **Games** — The **Games** button opens the main games list filtered to that community

Every game belongs to at most one community, shown in the **Community** section of the game's **Game Info** tab, along with links to that community's published documents.

## Joining In

**Membership is open.** There is no roster to join, no application, and no approval step. Anyone who is not banned can play in a community's games or create their own game there.

The one thing a community controls is who is *excluded* — see [Bans](#bans) below.

## Choosing a Community for Your Game

When you create a game, **Community** is a required field. It sets whose rules and bans apply to that game.

If only one community is available to you, it's selected automatically.

**You can change a game's community only while the game is still in Setup.** Once you move it to Recruitment, the community is locked — players have joined under one community's rules, and moving the game would silently swap those rules out from under them. Trying to change it later gives you an error explaining this.

## Bans

A community moderator can ban a user from that community.

**What a ban blocks** — Everything that would get you *into* one of that community's games:

- Applying to a game
- Being added to a game by a GM
- Joining as an audience member
- Having your application approved
- Creating a new game in that community

If you're banned and try to apply, you'll be told you are banned from that community.

**What a ban does not do:**

- **It is not retroactive.** A ban never removes you from a game you're already playing in. Whether to remove an existing player is the GM's call, using the normal participant-removal tools.
- **It does not reach other communities.** A ban is scoped to one community. You can still play in any other community's games.
- **It is not a site ban.** Your account, profile, and existing games are untouched.

Bans can be permanent or set to expire on a date. An expired ban stops blocking you automatically — no one has to lift it.

---

## Moderator: Managing a Community

If you own or moderate a community, a **Manage** button appears on its page. It has six tabs.

### Moderators

Lists the community's moderators.

**Only the owner can add or remove moderators.** Moderators can do everything else described on this page, but they cannot change the roster. Deciding who holds moderator powers stays with the one person who is accountable for the community.

### Bans

Add a ban by searching for a user, then giving:

- **Reason** — Shown to moderators and kept in the ban history
- **Expires** — Leave empty for a permanent ban; a date must be in the future

Removing a ban restores the user's access immediately.

### Ban History

An audit log of every ban action taken in the community — bans issued, extended, and lifted, with who did it and when.

The history is kept separately from the ban list, so **lifting a ban does not erase the record of it**. A user with no active ban may still have history here.

### Documents

Guidelines, house rules, and reference material for the community. Documents are written in Markdown.

Each document is either:

- **Draft** — Visible only to moderators. New documents start here.
- **Published** — Visible to everyone on the community page and linked from the Game Info tab of every game in the community

Publishing is deliberate: a new document is a draft until you say otherwise.

### Discord

Post game announcements to a Discord channel using a webhook.

Choose which game states trigger a post. **Setup is not available.** A game in Setup can still change communities, and it may be months from recruiting anyone — there's nothing worth announcing yet. The available events are: Recruitment, Character Creation, In Progress, Paused, Epilogue, Completed, and Cancelled.

A webhook with no events selected will never fire.

**Treat the webhook URL as a password.** Anyone holding it can post to your channel. ActionPhase never shows it back to you in full — the list shows a masked version, and editing a webhook leaves the URL alone unless you deliberately enter a new one. If a URL leaks, delete the webhook in Discord and add a fresh one here.

Each webhook shows its delivery status. **Never used** means no game event has matched it yet — that's not a failure. If delivery has been failing, the most recent error appears here, which is usually how you find out a webhook was deleted on the Discord side.

Announcements are best-effort: a Discord outage will never block a game from changing state, and a missed announcement is not retried indefinitely.

### Settings

The community's name, description, and banner image.

**The community's address cannot be changed.** It appears in links that have been shared elsewhere, and changing it would break them.

The banner accepts JPG, PNG, or WebP up to 5MB. Uploading a new one replaces the old file; removing it clears the banner entirely.
