import { useState } from 'react';
import { Button, Input, Select } from './ui';
import { CommentEditor } from './CommentEditor';
import type { Handout, UpdateHandoutRequest } from '../types/handouts';
import { Modal } from './Modal';

interface EditHandoutModalProps {
  handout: Handout;
  onClose: () => void;
  onSubmit: (data: UpdateHandoutRequest) => void;
  isSubmitting: boolean;
}

export function EditHandoutModal({ handout, onClose, onSubmit, isSubmitting }: EditHandoutModalProps) {
  const [formData, setFormData] = useState<UpdateHandoutRequest>({
    title: handout.title,
    content: handout.content,
    status: handout.status
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
    <Modal isOpen={true} onClose={onClose} title="Edit Handout">
      <form onSubmit={handleSubmit}>
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
            {isSubmitting ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
