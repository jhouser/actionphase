import { describe, it, expect, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { ChangePasswordForm } from './ChangePasswordForm';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

describe('ChangePasswordForm', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  it('renders change password form with all required fields', () => {
    renderWithProviders(<ChangePasswordForm />);

    expect(screen.getByLabelText('Current Password')).toBeInTheDocument();
    expect(screen.getByLabelText('New Password')).toBeInTheDocument();
    expect(screen.getByLabelText('Confirm New Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Change Password' })).toBeInTheDocument();
  });

  it('displays help text for password requirements', () => {
    renderWithProviders(<ChangePasswordForm />);

    expect(
      screen.getByText(/Must be at least 8 characters with uppercase, lowercase, number, and special character/)
    ).toBeInTheDocument();
  });

  it('handles form input changes', () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123!' } });

    expect(currentPasswordInput).toHaveValue('oldPassword123!');
    expect(newPasswordInput).toHaveValue('NewPassword123!');
    expect(confirmPasswordInput).toHaveValue('NewPassword123!');
  });

  it('shows validation error when current password is empty', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Current password is required')).toBeInTheDocument();
    });
  });

  it('shows validation error when new password is too short', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPass123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'Short1!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'Short1!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
    });
  });

  it('shows validation error when new password lacks uppercase letter', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'newpassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'newpassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one uppercase letter')).toBeInTheDocument();
    });
  });

  it('shows validation error when new password lacks lowercase letter', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NEWPASSWORD123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NEWPASSWORD123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one lowercase letter')).toBeInTheDocument();
    });
  });

  it('shows validation error when new password lacks number', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one number')).toBeInTheDocument();
    });
  });

  it('shows validation error when new password lacks special character', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one special character')).toBeInTheDocument();
    });
  });

  it('shows validation error when passwords do not match', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'DifferentPassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
    });
  });

  it('clears error when user starts typing in field with error', async () => {
    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    // Submit to trigger validation error
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Current password is required')).toBeInTheDocument();
    });

    // Start typing to clear error
    fireEvent.change(currentPasswordInput, { target: { value: 'oldPassword123!' } });

    await waitFor(() => {
      expect(screen.queryByText('Current password is required')).not.toBeInTheDocument();
    });
  });

  it('submits form with valid data successfully', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/change-password', async () => {
        return HttpResponse.json({ message: 'Password changed successfully' });
      })
    );

    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'OldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.click(submitButton);

    // Toast message should appear in the DOM
    await waitFor(() => {
      expect(screen.getByText('Password changed successfully')).toBeInTheDocument();
    });

    // Form should be cleared after successful submission
    await waitFor(() => {
      expect(currentPasswordInput).toHaveValue('');
      expect(newPasswordInput).toHaveValue('');
      expect(confirmPasswordInput).toHaveValue('');
    });
  });

  it('shows error toast when API call fails', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/change-password', async () => {
        return HttpResponse.json(
          { detail: 'Current password is incorrect' },
          { status: 400 }
        );
      })
    );

    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'WrongPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.click(submitButton);

    // Error toast message should appear in the DOM
    await waitFor(() => {
      expect(screen.getByText('Current password is incorrect')).toBeInTheDocument();
    });
  });

  it('disables form inputs while submitting', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/change-password', async () => {
        // Simulate slow API
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json({ message: 'Password changed successfully' });
      })
    );

    renderWithProviders(<ChangePasswordForm />);

    const currentPasswordInput = screen.getByLabelText('Current Password');
    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Change Password' });

    fireEvent.change(currentPasswordInput, { target: { value: 'OldPassword123!' } });
    fireEvent.change(newPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewPassword123!' } });
    fireEvent.click(submitButton);

    // Inputs should be disabled during submission
    expect(currentPasswordInput).toBeDisabled();
    expect(newPasswordInput).toBeDisabled();
    expect(confirmPasswordInput).toBeDisabled();

    await waitFor(() => {
      expect(currentPasswordInput).not.toBeDisabled();
    });
  });
});
