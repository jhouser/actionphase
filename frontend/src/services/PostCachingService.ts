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
        var result = new Date(date);
        result.setDate(result.getDate() + days);
        return result;
    }

    private static savedPostDeserializeHelper(key: any, value: any) {
        if (key === 'lastEdit' && typeof value === 'string') {
            return new Date(value);
        }
        return value;
    }

    cleanup() {
        const staleKeys: string[] = [];
        for(let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && this._types.find(t => key.startsWith(t))) {
                const l = localStorage.getItem(key);
                if (l) {
                    try {
                        const saved = JSON.parse(l, PostCachingService.savedPostDeserializeHelper) as savedPost;
                        if (saved.lastEdit < PostCachingService.addDays(new Date(), -7)) {
                            //older than a week, mark for deletion
                            staleKeys.push(key);
                        }                        
                    } catch (error) {
                        logger.warn(`Found invalid cached post in localStorage, key: ${key}`);
                        localStorage.removeItem(key);
                        return undefined;                        
                    }
                } else {
                    localStorage.removeItem(key);
                }
            }
        }
        for(let key of staleKeys) {
            localStorage.removeItem(key);
        }
    }

    createAutosaveId(type: postType, id: string | number | null | undefined) : string | undefined {
        if (id === undefined || id === null){
            return undefined;
        }
        return `${type}-${id}`;
    }

    save(autosaveId: string, content: string | undefined) {
        if (content) {
            localStorage.setItem(`${autosaveId}`, JSON.stringify({lastEdit: new Date(), content: content}));
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
            logger.warn(`Found invalid cached post in localStorage, key: ${autosaveId}`);
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

//automatic cleanup on first module load
postCachingService.cleanup();