import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PostCachingService } from './PostCachingService';

describe('PostCachingService', () => {
	let service: PostCachingService;

	beforeEach(() => {
		localStorage.clear();
		service = new PostCachingService();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('createAutosaveId', () => {
		it('formats the id as type-id', () => {
			expect(service.createAutosaveId('post-reply', 42)).toBe('post-reply-42');
			expect(service.createAutosaveId('action', 'abc')).toBe('action-abc');
		});

		it('returns undefined for null or undefined ids', () => {
			expect(service.createAutosaveId('action', null)).toBeUndefined();
			expect(service.createAutosaveId('action', undefined)).toBeUndefined();
		});
	});

	describe('save', () => {
		it('stores a savedPost under the supplied id', () => {
			service.save('post-reply-42', 'saved content');

			const saved = JSON.parse(localStorage.getItem('post-reply-42')!);
			expect(saved.content).toBe('saved content');
			expect(new Date(saved.lastEdit).toString()).not.toBe('Invalid Date');
		});

		it('removes the entry for undefined or empty content', () => {
			localStorage.setItem('post-reply-42', JSON.stringify({ content: 'old' }));
			service.save('post-reply-42', undefined);
			expect(localStorage.getItem('post-reply-42')).toBeNull();

			localStorage.setItem('post-reply-42', JSON.stringify({ content: 'old' }));
			service.save('post-reply-42', '');
			expect(localStorage.getItem('post-reply-42')).toBeNull();
		});
	});

	it('removes content using the supplied id', () => {
		localStorage.setItem('action-7', 'cached content');

		service.remove('action-7');

		expect(localStorage.getItem('action-7')).toBeNull();
	});

	it('removes saved content older than one week during cleanup', () => {
		const oldDate = new Date();
		oldDate.setDate(oldDate.getDate() - 8);
		localStorage.setItem('post-reply-old', JSON.stringify({
			lastEdit: oldDate,
			content: 'old content',
		}));
		localStorage.setItem('post-reply-new', JSON.stringify({
			lastEdit: new Date(),
			content: 'new content',
		}));
		localStorage.setItem('unrelated-key', JSON.stringify({
			lastEdit: oldDate,
			content: 'keep content',
		}));

		service.cleanup();

		expect(localStorage.getItem('post-reply-old')).toBeNull();
		expect(localStorage.getItem('post-reply-new')).not.toBeNull();
		expect(localStorage.getItem('unrelated-key')).not.toBeNull();
	});

	describe('get', () => {
		it('returns the saved content for a stored draft', () => {
			service.save('post-reply-42', 'saved content');

			expect(service.get('post-reply-42')).toBe('saved content');
		});

		it('returns undefined for a key that was never saved', () => {
			expect(service.get('post-reply-missing')).toBeUndefined();
		});

		it('discards and returns undefined for an unparseable entry', () => {
			localStorage.setItem('post-reply-42', '{not valid json');

			expect(service.get('post-reply-42')).toBeUndefined();
			// The bad entry must not be left behind to fail again on every read.
			expect(localStorage.getItem('post-reply-42')).toBeNull();
		});

		it('returns undefined for valid JSON that is not a savedPost', () => {
			localStorage.setItem('post-reply-42', JSON.stringify({ somethingElse: true }));

			expect(service.get('post-reply-42')).toBeUndefined();
		});

		it('round-trips content containing newlines and quotes', () => {
			const content = 'line one\n"quoted"\n\ttabbed';
			service.save('action-1', content);

			expect(service.get('action-1')).toBe(content);
		});
	});

	describe('save', () => {
		it('does not throw when storage rejects the write', () => {
			vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
				const error = new Error('quota');
				error.name = 'QuotaExceededError';
				throw error;
			});

			// Autosave fires on every keystroke; a storage failure must never
			// propagate into the editor's onChange and break typing.
			expect(() => service.save('action-1', 'text')).not.toThrow();
		});

		it('overwrites an existing draft rather than appending', () => {
			service.save('action-1', 'first');
			service.save('action-1', 'second');

			expect(service.get('action-1')).toBe('second');
		});
	});

	describe('remove', () => {
		it('ignores an undefined id', () => {
			expect(() => service.remove(undefined)).not.toThrow();
		});
	});

	describe('cleanup', () => {
		const staleDate = () => {
			const d = new Date();
			d.setDate(d.getDate() - 8);
			return d;
		};

		it('keeps sweeping after hitting an unparseable entry', () => {
			// Regression: the catch block used to `return`, aborting the whole
			// sweep so every later stale draft leaked forever.
			localStorage.setItem('action-corrupt', '{not valid json');
			localStorage.setItem('action-stale', JSON.stringify({
				lastEdit: staleDate(),
				content: 'stale',
			}));

			service.cleanup();

			expect(localStorage.getItem('action-corrupt')).toBeNull();
			expect(localStorage.getItem('action-stale')).toBeNull();
		});

		it('sweeps every stale entry when an empty-valued key is present', () => {
			// Regression: removing during the index walk shifted the remaining
			// keys down, so one entry was skipped per mid-loop deletion.
			localStorage.setItem('action-empty', '');
			localStorage.setItem('action-stale1', JSON.stringify({ lastEdit: staleDate(), content: 'a' }));
			localStorage.setItem('action-stale2', JSON.stringify({ lastEdit: staleDate(), content: 'b' }));
			localStorage.setItem('action-stale3', JSON.stringify({ lastEdit: staleDate(), content: 'c' }));

			service.cleanup();

			expect(localStorage.getItem('action-empty')).toBeNull();
			expect(localStorage.getItem('action-stale1')).toBeNull();
			expect(localStorage.getItem('action-stale2')).toBeNull();
			expect(localStorage.getItem('action-stale3')).toBeNull();
		});

		it('drops entries whose lastEdit is missing or unusable', () => {
			localStorage.setItem('action-nodate', JSON.stringify({ content: 'no date' }));
			localStorage.setItem('action-baddate', JSON.stringify({ lastEdit: 'not-a-date', content: 'bad' }));

			service.cleanup();

			expect(localStorage.getItem('action-nodate')).toBeNull();
			expect(localStorage.getItem('action-baddate')).toBeNull();
		});

		it('preserves fresh drafts across every cached type', () => {
			const types = ['cr-main-post', 'post-reply', 'action', 'action-result', 'conversation'];
			for (const type of types) {
				localStorage.setItem(`${type}-1`, JSON.stringify({ lastEdit: new Date(), content: 'fresh' }));
			}

			service.cleanup();

			for (const type of types) {
				expect(localStorage.getItem(`${type}-1`)).not.toBeNull();
			}
		});

		it('leaves keys belonging to other features alone', () => {
			localStorage.setItem('app-theme', 'dark');
			localStorage.setItem('auth_token', 'a-token');

			service.cleanup();

			expect(localStorage.getItem('app-theme')).toBe('dark');
			expect(localStorage.getItem('auth_token')).toBe('a-token');
		});
	});

	describe('clearAll', () => {
		it('drops cached drafts of every type', () => {
			service.save('action-1', 'private action');
			service.save('conversation-2', 'private message');
			service.save('post-reply-3', 'a reply');

			service.clearAll();

			expect(service.get('action-1')).toBeUndefined();
			expect(service.get('conversation-2')).toBeUndefined();
			expect(service.get('post-reply-3')).toBeUndefined();
		});

		it('leaves unrelated keys intact', () => {
			service.save('action-1', 'private action');
			localStorage.setItem('app-theme', 'dark');

			service.clearAll();

			expect(localStorage.getItem('app-theme')).toBe('dark');
		});
	});
});
