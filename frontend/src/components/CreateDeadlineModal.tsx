import { useState, useEffect } from 'react';
import { Modal, Input, Textarea, DateTimeInput, Button, Alert } from './ui';
import type { CreateDeadlineRequest } from '../types/deadlines';
import { localDateTimeToUTC } from '../utils/timezone';

export interface CreateDeadlineModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: CreateDeadlineRequest) => void;
  isLoading?: boolean;
  error?: string;
}

/**
 * CreateDeadlineModal - Modal for creating new game deadlines
 *
 * Features:
 * - Form validation
 * - Date/time picker
 * - Error display
 * - Loading state
 *
 * @example
 * ```tsx
 * <CreateDeadlineModal
 *   isOpen={isOpen}
 *   onClose={handleClose}
 *   onSubmit={handleCreate}
 *   isLoading={createMutation.isPending}
 *   error={createMutation.error?.message}
 * />
 * ```
 */
export function CreateDeadlineModal({
  isOpen,
  onClose,
  onSubmit,
  isLoading = false,
  error,
}: CreateDeadlineModalProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [deadline, setDeadline] = useState('');
  const [validationErrors, setValidationErrors] = useState<{ [key: string]: string }>({});

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setTitle('');
      setDescription('');
      setDeadline('');
      setValidationErrors({});
    }
  }, [isOpen]);

  const handleClose = () => {
    // Reset form on close
    setTitle('');
    setDescription('');
    setDeadline('');
    setValidationErrors({});
    onClose();
  };

  const validateForm = (): boolean => {
    const errors: { [key: string]: string } = {};

    if (!title.trim()) {
      errors.title = 'Title is required';
    } else if (title.trim().length > 100) {
      errors.title = 'Title must be 100 characters or less';
    }

    if (!description.trim()) {
      errors.description = 'Description is required';
    }

    if (!deadline) {
      errors.deadline = 'Deadline is required';
    } else {
      // Validate deadline is in the future
      const deadlineDate = new Date(deadline);
      if (deadlineDate <= new Date()) {
        errors.deadline = 'Deadline must be in the future';
      }
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    // The datetime-local input gives us a string in local time (e.g., "2024-11-15T18:00")
    // Convert to UTC for storage using timezone utilities
    const isoDeadline = localDateTimeToUTC(deadline);

    onSubmit({
      title: title.trim(),
      description: description.trim(),
      deadline: isoDeadline,
    });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Create New Deadline"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleSubmit}
            loading={isLoading}
            disabled={isLoading}
          >
            Create Deadline
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <Alert variant="danger" title="Error">
            {error}
          </Alert>
        )}

        <Input
          name="title"
          label="Title"
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          error={validationErrors.title}
          placeholder="e.g., Action Submission Deadline"
          disabled={isLoading}
          required
        />

        <Textarea
          name="description"
          label="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          error={validationErrors.description}
          placeholder="Provide details about this deadline..."
          rows={4}
          disabled={isLoading}
          required
        />

        <DateTimeInput
          name="deadline"
          label="Deadline"
          value={deadline}
          onChange={(e) => setDeadline(e.target.value)}
          error={validationErrors.deadline}
          placeholder="Select deadline date and time"
          disabled={isLoading}
          required
        />
      </form>
    </Modal>
  );
}
