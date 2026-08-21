import { useState } from 'react';
import { Button, Input, Select } from './ui';
import { CommentEditor } from './CommentEditor';
import type { CreateHandoutRequest } from '../types/handouts';

interface CreateHandoutModalProps {
  onClose: () => void;
  onSubmit: (data: CreateHandoutRequest) => void;
  isSubmitting: boolean;
}

export function CreateHandoutModal({ onClose, onSubmit, isSubmitting }: CreateHandoutModalProps) {
  const [formData, setFormData] = useState<CreateHandoutRequest>({
    title: '',
    content: '',
    status: 'draft'
  });
  // Content is marked required in the label but the browser cannot enforce it:
  // CommentEditor is not a native input, so `required` has nothing to attach to.
  // The backend rejects blank content at Bind with a 400, so guard here rather
  // than let the submit round-trip into an error banner.
  const isContentEmpty = !formData.content.trim();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (isContentEmpty) return;
    onSubmit({ ...formData, content: formData.content.trim() });
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="surface-base rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <form onSubmit={handleSubmit} className="p-6" data-testid="handout-form">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-content-primary">Create New Handout</h3>
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
              className="text-content-tertiary hover:text-content-secondary h-auto p-0"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </Button>
          </div>

          <div className="space-y-4">
            <div>
              <Input
                id="handout-title"
                label="Title"
                type="text"
                value={formData.title}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  title: e.target.value
                }))}
                placeholder="e.g., 'Combat Rules' or 'World Lore'"
                required
                data-testid="handout-title-input"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-content-primary mb-1">
                Content <span className="text-danger">*</span>
              </label>
              <CommentEditor
                value={formData.content}
                onChange={(content) => setFormData(prev => ({ ...prev, content }))}
                placeholder="Write your handout content here... (Markdown supported)"
                rows={10}
                warnOnUnsavedChanges
                textareaTestId="handout-content-input"
              />
            </div>

            <div>
              <Select
                id="handout-status"
                label="Status"
                value={formData.status}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  status: e.target.value as 'draft' | 'published'
                }))}
                required
                helperText="Draft handouts are only visible to you. Published handouts are visible to all players."
                data-testid="handout-status-select"
              >
                <option value="draft">Draft</option>
                <option value="published">Published</option>
              </Select>
            </div>
          </div>

          <div className="flex justify-end space-x-3 mt-6">
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              disabled={isSubmitting || isContentEmpty}
            >
              {isSubmitting ? 'Creating...' : 'Create Handout'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
