import { useState } from 'react';
import { apiClient } from '../lib/api';
import { useToast } from '../contexts/ToastContext';
import { Input, Button } from './ui';
import type { ChangePasswordRequest } from '../types/auth';
import { extractApiErrorMessage } from '@/lib/errors';

export function ChangePasswordForm() {
  const { showToast } = useToast();
  const [formData, setFormData] = useState<ChangePasswordRequest>({
    current_password: '',
    new_password: '',
    confirm_password: '',
  });
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Partial<Record<keyof ChangePasswordRequest, string>>>({});

  const validateForm = (): boolean => {
    const newErrors: Partial<Record<keyof ChangePasswordRequest, string>> = {};

    if (!formData.current_password) {
      newErrors.current_password = 'Current password is required';
    }

    if (!formData.new_password) {
      newErrors.new_password = 'New password is required';
    } else if (formData.new_password.length < 8) {
      newErrors.new_password = 'Password must be at least 8 characters';
    } else if (!/[A-Z]/.test(formData.new_password)) {
      newErrors.new_password = 'Password must contain at least one uppercase letter';
    } else if (!/[a-z]/.test(formData.new_password)) {
      newErrors.new_password = 'Password must contain at least one lowercase letter';
    } else if (!/[0-9]/.test(formData.new_password)) {
      newErrors.new_password = 'Password must contain at least one number';
    } else if (!/[!@#$%^&*(),.?":{}|<>]/.test(formData.new_password)) {
      newErrors.new_password = 'Password must contain at least one special character';
    }

    if (!formData.confirm_password) {
      newErrors.confirm_password = 'Please confirm your new password';
    } else if (formData.new_password !== formData.confirm_password) {
      newErrors.confirm_password = 'Passwords do not match';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setIsLoading(true);
    setErrors({});

    try {
      await apiClient.auth.changePassword(formData);

      showToast('Password changed successfully', 'success');

      // Clear form
      setFormData({
        current_password: '',
        new_password: '',
        confirm_password: '',
      });
    } catch (error: unknown) {
      const errorMessage = extractApiErrorMessage(error) || 'Failed to change password. Please try again.';
      showToast(errorMessage, 'danger');
    } finally {
      setIsLoading(false);
    }
  };

  const handleChange = (field: keyof ChangePasswordRequest) => (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData((prev) => ({ ...prev, [field]: e.target.value }));
    // Clear error for this field when user starts typing
    if (errors[field]) {
      setErrors((prev) => ({ ...prev, [field]: undefined }));
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <Input
        label="Current Password"
        id="current_password"
        type="password"
        value={formData.current_password}
        onChange={handleChange('current_password')}
        error={errors.current_password}
        disabled={isLoading}
      />

      <Input
        label="New Password"
        id="new_password"
        type="password"
        value={formData.new_password}
        onChange={handleChange('new_password')}
        error={errors.new_password}
        disabled={isLoading}
        helperText="Must be at least 8 characters with uppercase, lowercase, number, and special character"
      />

      <Input
        label="Confirm New Password"
        id="confirm_password"
        type="password"
        value={formData.confirm_password}
        onChange={handleChange('confirm_password')}
        error={errors.confirm_password}
        disabled={isLoading}
      />

      <div className="pt-4">
        <Button
          type="submit"
          variant="primary"
          loading={isLoading}
        >
          Change Password
        </Button>
      </div>
    </form>
  );
}
