import { beforeEach, describe, expect, it } from 'vitest';
import { PostCachingService } from './PostCachingService';

describe('PostCachingService', () => {
	let service: PostCachingService;

	beforeEach(() => {
		localStorage.clear();
		service = new PostCachingService();
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
});
