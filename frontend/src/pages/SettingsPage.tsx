import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTheme } from '../contexts/ThemeContext';
import { ProfileSection } from '../components/ProfileSection';
import { ChangePasswordForm } from '../components/ChangePasswordForm';
import { ActiveSessions } from '../components/ActiveSessions';
import { ChangeUsernameForm } from '../components/ChangeUsernameForm';
import { ChangeEmailForm } from '../components/ChangeEmailForm';
import { SettingsSidebar } from '../components/SettingsSidebar';
import { DiscordNotificationsSection } from '../components/DiscordNotificationsSection';
import { Radio } from '@/components/ui';
import { useUserPreferences, useUpdateUserPreferences } from '../hooks/useUserPreferences';
import { useQueryClient } from '@tanstack/react-query';
import type { CommentReadMode, FontSize } from '../lib/api/auth';

const VALID_TABS = ['profile', 'appearance', 'security', 'account', 'reading', 'notifications'] as const;
type SettingsTab = typeof VALID_TABS[number];

function isValidTab(tab: string | null): tab is SettingsTab {
  return VALID_TABS.includes(tab as SettingsTab);
}

export function SettingsPage() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: preferences } = useUserPreferences();
  const updatePreferences = useUpdateUserPreferences();
  const queryClient = useQueryClient();

  const rawTab = searchParams.get('tab');
  const activeSection: SettingsTab = isValidTab(rawTab) ? rawTab : 'profile';

  // When the Discord OAuth callback redirects back with ?discord=linked,
  // invalidate the Discord status query and switch to notifications tab.
  useEffect(() => {
    if (searchParams.get('discord') === 'linked') {
      queryClient.invalidateQueries({ queryKey: ['discordStatus'] });
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('discord');
        next.set('tab', 'notifications');
        return next;
      }, { replace: true });
    }
  }, [queryClient, searchParams, setSearchParams]);

  const handleCommentReadModeChange = (mode: CommentReadMode) => {
    updatePreferences.mutate({
      theme: preferences?.theme ?? 'auto',
      comment_read_mode: mode,
      font_size: preferences?.font_size ?? 'medium',
    });
  };

  const handleFontSizeChange = (size: FontSize) => {
    updatePreferences.mutate({
      theme: preferences?.theme ?? 'auto',
      comment_read_mode: preferences?.comment_read_mode ?? 'manual',
      font_size: size,
    });
  };

  const sections = [
    {
      id: 'profile',
      label: 'Profile',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      ),
    },
    {
      id: 'appearance',
      label: 'Appearance',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
        </svg>
      ),
    },
    {
      id: 'security',
      label: 'Account Security',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
      ),
    },
    {
      id: 'account',
      label: 'Account Information',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      ),
    },
    {
      id: 'reading',
      label: 'Reading',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
        </svg>
      ),
    },
    {
      id: 'notifications',
      label: 'Notifications',
      icon: (
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
      ),
    },
  ];

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold text-content-primary mb-8">Settings</h1>

      <div className="flex flex-col md:flex-row gap-6">
        {/* Sidebar Navigation */}
        <SettingsSidebar
          sections={sections}
          activeSection={activeSection}
        />

        {/* Content Area */}
        <div className="flex-1">
        {/* Profile Section */}
        {activeSection === 'profile' && <ProfileSection />}

        {/* Appearance Section */}
        {activeSection === 'appearance' && (

          <div className="bg-surface-base rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-content-primary mb-4">
              Appearance
            </h2>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-content-secondary mb-2">
                  Theme
                </label>
                <div className="space-y-2">
                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="light"
                      checked={theme === 'light'}
                      onChange={(e) => setTheme(e.target.value as 'light')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        Light
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Always use light theme
                      </div>
                    </div>
                  </label>

                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="dark"
                      checked={theme === 'dark'}
                      onChange={(e) => setTheme(e.target.value as 'dark')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        Dark
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Always use dark theme
                      </div>
                    </div>
                  </label>

                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="highContrast"
                      checked={theme === 'highContrast'}
                      onChange={(e) => setTheme(e.target.value as 'highContrast')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        High Contrast
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Maximum contrast light theme for accessibility
                      </div>
                    </div>
                  </label>

                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="highContrastDark"
                      checked={theme === 'highContrastDark'}
                      onChange={(e) => setTheme(e.target.value as 'highContrastDark')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        High Contrast Dark
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Maximum contrast dark theme for accessibility
                      </div>
                    </div>
                  </label>

                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="colorblind"
                      checked={theme === 'colorblind'}
                      onChange={(e) => setTheme(e.target.value as 'colorblind')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        Colorblind-Friendly
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Optimized for color vision deficiency
                      </div>
                    </div>
                  </label>

                  <label className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised">
                    <input
                      type="radio"
                      name="theme"
                      value="system"
                      checked={theme === 'system'}
                      onChange={(e) => setTheme(e.target.value as 'system')}
                      className="h-4 w-4 text-interactive-primary focus:ring-2 focus:ring-interactive-primary"
                    />
                    <div className="ml-3">
                      <div className="text-sm font-medium text-content-primary">
                        System
                      </div>
                      <div className="text-sm text-content-tertiary">
                        Use system preference (currently: {resolvedTheme})
                      </div>
                    </div>
                  </label>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-content-secondary mb-2">
                  Text Size
                </label>
                <div className="space-y-2">
                  <div
                    className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised"
                    data-testid="font-size-small"
                  >
                    <Radio
                      name="font_size"
                      value="small"
                      label="Small"
                      checked={(preferences?.font_size ?? 'medium') === 'small'}
                      onChange={() => handleFontSizeChange('small')}
                    />
                  </div>

                  <div
                    className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised"
                    data-testid="font-size-medium"
                  >
                    <Radio
                      name="font_size"
                      value="medium"
                      label="Medium"
                      checked={(preferences?.font_size ?? 'medium') === 'medium'}
                      onChange={() => handleFontSizeChange('medium')}
                    />
                  </div>

                  <div
                    className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised"
                    data-testid="font-size-large"
                  >
                    <Radio
                      name="font_size"
                      value="large"
                      label="Large"
                      checked={(preferences?.font_size ?? 'medium') === 'large'}
                      onChange={() => handleFontSizeChange('large')}
                    />
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t border-theme-default">
                <p className="text-sm text-content-secondary">
                  Your theme and text size preferences are saved and will be applied across all your devices when you
                  log in.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Account Security Section */}
        {activeSection === 'security' && (
          <div className="bg-surface-base rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-content-primary mb-4">
              Account Security
            </h2>
            <div className="space-y-6">
              <ChangePasswordForm />
              <ActiveSessions />
            </div>
          </div>
        )}

        {/* Account Information Section */}
        {activeSection === 'account' && (
          <div className="bg-surface-base rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-content-primary mb-4">
              Account Information
            </h2>
            <div className="space-y-6">
              <ChangeUsernameForm />
              <ChangeEmailForm />
            </div>
          </div>
        )}

        {/* Notifications Section */}
        {activeSection === 'notifications' && (
          <div className="space-y-6">
            <h2 className="text-xl font-semibold text-content-primary">Notifications</h2>
            <DiscordNotificationsSection />
          </div>
        )}

        {/* Reading Section */}
        {activeSection === 'reading' && (
          <div className="bg-surface-base rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-content-primary mb-4">Reading</h2>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-content-secondary mb-2">
                  Comment Read Tracking
                </label>
                <div className="space-y-2">
                  <div
                    className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised"
                    data-testid="read-mode-manual"
                  >
                    <Radio
                      name="comment_read_mode"
                      value="manual"
                      label="Manual"
                      helperText="Mark individual comments as read yourself; read comments fade out"
                      checked={(preferences?.comment_read_mode ?? 'manual') === 'manual'}
                      onChange={() => handleCommentReadModeChange('manual')}
                    />
                  </div>

                  <div
                    className="flex items-center p-3 border border-theme-default rounded-lg cursor-pointer hover:bg-surface-raised"
                    data-testid="read-mode-auto"
                  >
                    <Radio
                      name="comment_read_mode"
                      value="auto"
                      label="Automatic"
                      helperText="Highlight new comments since your last visit"
                      checked={(preferences?.comment_read_mode ?? 'manual') === 'auto'}
                      onChange={() => handleCommentReadModeChange('auto')}
                    />
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t border-theme-default">
                <p className="text-sm text-content-secondary">
                  Your reading preference is saved and synced across devices.
                </p>
              </div>
            </div>
          </div>
        )}
        </div>
      </div>
    </div>
  );
}
