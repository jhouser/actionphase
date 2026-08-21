import { logger } from "./LoggingService";

export type postType = 'cr-main-post' | 'post-reply' | 'action' | 'action-result' | 'conversation';

type savedPost = { lastEdit: Date, content: string };


export class PostCachingService {

    private _types : postType[] = [
        'cr-main-post', 
        'post-reply', 
        'action', 
        'action-result', 
        'conversation' ];

    private static addDays(date :Date, days :number) {
        const result = new Date(date);
        result.setDate(result.getDate() + days);
        return result;
    }

    private static savedPostDeserializeHelper(key: string, value: unknown) {
        if (key === 'lastEdit' && typeof value === 'string') {
            return new Date(value);
        }
        return value;
    }

    /**
     * Sweeps cached drafts older than a week.
     *
     * Every removal is deferred to a second pass: deleting during the index
     * walk shifts the remaining keys down and silently skips entries. A single
     * unparseable entry must not abort the sweep either, so parse failures are
     * collected and skipped rather than returned on.
     */
    cleanup() {
        const staleKeys: string[] = [];
        for(let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && this._types.find(t => key.startsWith(t))) {
                const l = localStorage.getItem(key);
                if (l) {
                    try {
                        const saved = JSON.parse(l, PostCachingService.savedPostDeserializeHelper) as savedPost;
                        if (!(saved?.lastEdit instanceof Date) || Number.isNaN(saved.lastEdit.getTime())) {
                            logger.warn(`Found cached post with no usable lastEdit in localStorage, key: ${key}`);
                            staleKeys.push(key);
                        } else if (saved.lastEdit < PostCachingService.addDays(new Date(), -7)) {
                            //older than a week, mark for deletion
                            staleKeys.push(key);
                        }
                    } catch (error) {
                        logger.warn(`Found invalid cached post in localStorage, key: ${key} - ${error}`);
                        staleKeys.push(key);
                    }
                } else {
                    staleKeys.push(key);
                }
            }
        }
        for(const key of staleKeys) {
            localStorage.removeItem(key);
        }
    }

    /**
     * Drops every cached draft. Called on logout: drafts are private content
     * (actions, PMs) and must not survive into the next session on a shared
     * browser.
     */
    clearAll() {
        const keys: string[] = [];
        for(let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && this._types.find(t => key.startsWith(t))) {
                keys.push(key);
            }
        }
        for(const key of keys) {
            localStorage.removeItem(key);
        }
    }

    createAutosaveId(type: postType, id: string | number | null | undefined) : string | undefined {
        if (id === undefined || id === null){
            return undefined;
        }
        return `${type}-${id}`;
    }

    /**
     * Autosave runs on every keystroke, so a storage failure (quota exhausted,
     * Safari private mode) must never propagate into the editor's onChange and
     * break typing. A dropped draft is recoverable; a dead textarea is not.
     */
    save(autosaveId: string, content: string | undefined) {
        if (content) {
            try {
                localStorage.setItem(`${autosaveId}`, JSON.stringify({lastEdit: new Date(), content: content}));
            } catch (error) {
                logger.warn(`Failed to autosave post to localStorage, key: ${autosaveId} - ${error}`);
            }
        }
        else {
            this.remove(autosaveId);
        }
    }

    get(autosaveId: string) {
        const l = localStorage.getItem(autosaveId);
        if (!l) {
            return undefined;
        }
        try {
            const saved = JSON.parse(l, PostCachingService.savedPostDeserializeHelper) as (savedPost | undefined);
            return saved?.content;
        } catch (error) {
            logger.warn(`Found invalid cached post in localStorage, key: ${autosaveId} - ${error}`);
            localStorage.removeItem(autosaveId);
            return undefined;
        }
    }

    remove(autosaveId: string | undefined) {
        if (!autosaveId) return;
        localStorage.removeItem(autosaveId);
    }
}

export const postCachingService = new PostCachingService();

// Automatic cleanup on first module load. Guarded because this runs at import
// time: if localStorage is unavailable (disabled cookies, sandboxed iframe),
// an unguarded throw here would break every component importing this module.
try {
    postCachingService.cleanup();
} catch (error) {
    logger.warn(`Failed to run cached post cleanup on load - ${error}`);
}