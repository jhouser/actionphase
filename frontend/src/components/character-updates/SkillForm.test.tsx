import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SkillForm } from './SkillForm';

describe('SkillForm', () => {
  describe('Description field uses markdown editor', () => {
    it('renders Write/Preview tabs for description field', () => {
      render(
        <SkillForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // CommentEditor renders Write/Preview tabs — plain Textarea does not
      expect(screen.getByRole('button', { name: /^write$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^preview$/i })).toBeInTheDocument();
    });

    it('pre-populates description editor with initial value', () => {
      render(
        <SkillForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
          initialValues={{ description: 'Mastery of blades' }}
        />
      );

      expect(screen.getByDisplayValue('Mastery of blades')).toBeInTheDocument();
    });
  });
});
