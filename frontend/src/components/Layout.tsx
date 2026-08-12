import { useState, useEffect } from 'react';
import type { ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { Wrench } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { useOptionalUtilityDrawer } from '../contexts/UtilityDrawerContext';
import NotificationBell from './NotificationBell';
import { AdminBanner } from './AdminBanner';
import { EmailVerificationBanner } from './EmailVerificationBanner';

interface LayoutProps {
  children: ReactNode;
}

export const Layout = ({ children }: LayoutProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const { isAuthenticated, logout, currentUser } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  // Optional: the nav renders in contexts without the drawer mounted (and in
  // tests that render Layout standalone). No drawer, no button.
  const utilityDrawer = useOptionalUtilityDrawer();

  // Reset menu states on route changes to prevent stale menu visibility
  useEffect(() => {
    setIsUserMenuOpen(false);
    setIsMobileMenuOpen(false);
  }, [location.pathname]);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const isActive = (path: string) => {
    return location.pathname === path;
  };

  const navLinkClass = (path: string) => {
    const baseClasses = "px-3 py-2 rounded-md text-sm font-medium transition-colors";
    return isActive(path)
      ? `${baseClasses} bg-interactive-primary-hover text-white`
      : `${baseClasses} text-white/90 hover:bg-interactive-primary-hover hover:text-white`;
  };

  return (
    <div className="min-h-screen surface-sunken" data-root-layout="true">
      {/* Navigation Bar */}
      {isAuthenticated && (
        <nav className="bg-interactive-primary shadow-lg sticky top-0 z-50">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                {/* Logo/Brand */}
                <Link to="/dashboard" className="flex items-center">
                  <span className="text-xl sm:text-2xl font-bold text-white">ActionPhase</span>
                </Link>

                {/* Desktop Navigation Links - Hidden on Mobile */}
                <div className="hidden md:flex ml-10 space-x-4">
                  <Link to="/dashboard" className={navLinkClass('/dashboard')}>
                    Dashboard
                  </Link>
                  <Link to="/games" className={navLinkClass('/games')}>
                    Games
                  </Link>
                  <a
                    href="/docs/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="px-3 py-2 rounded-md text-sm font-medium text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                  >
                    Help
                  </a>
                </div>
              </div>

              {/* Right side: Notification + User Menu (Desktop) / Hamburger (Mobile) */}
              <div className="flex items-center space-x-2 sm:space-x-4">
                {/* Utilities — character sheet, dice roller, and friends.
                    Available on every page, not just the common room. */}
                {utilityDrawer && (
                  <button
                    type="button"
                    onClick={utilityDrawer.openDrawer}
                    className="p-2 rounded-md text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                    title="Utilities"
                    aria-label="Utilities"
                    data-testid="global-utility-drawer-toggle"
                    data-faro-user-action-name="open-utility-drawer"
                  >
                    <Wrench className="w-5 h-5" />
                  </button>
                )}

                {/* Notification Bell */}
                <NotificationBell />

                {/* Desktop: User Dropdown Menu - Hidden on Mobile */}
                <div
                  className="hidden md:block relative"
                  data-testid="user-menu-trigger"
                  onMouseEnter={() => setIsUserMenuOpen(true)}
                  onMouseLeave={() => setIsUserMenuOpen(false)}
                >
                  <button
                    className="flex items-center space-x-2 px-3 py-2 rounded-md text-sm font-medium text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                    data-faro-user-action-name="open-user-menu"
                    // Hover alone left this menu unreachable by keyboard and by
                    // touch on desktop-width screens; the button had no handler
                    // at all. Clicking now toggles it as well.
                    onClick={() => setIsUserMenuOpen((open) => !open)}
                    aria-expanded={isUserMenuOpen}
                    aria-haspopup="menu"
                  >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                    <span>{currentUser?.username}</span>
                    <svg className={`w-4 h-4 transition-transform ${isUserMenuOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>

                  {/* Desktop Dropdown Menu */}
                  {isUserMenuOpen && (
                    <div className="absolute right-0 top-full pt-1 w-48 z-50">
                      <div className="surface-raised rounded-md shadow-lg border border-border-primary">
                        <div className="py-1">
                          <Link
                            to="/settings"
                            className="block px-4 py-2 text-sm text-content-primary hover:bg-bg-secondary transition-colors"
                            onClick={() => setIsUserMenuOpen(false)}
                          >
                            <div className="flex items-center space-x-2">
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                              </svg>
                              <span>Settings</span>
                            </div>
                          </Link>

                          {/* Admin Link (only visible to admins) */}
                          {currentUser?.is_admin && (
                            <Link
                              to="/admin"
                              className="block px-4 py-2 text-sm text-content-primary hover:bg-bg-secondary transition-colors"
                              onClick={() => setIsUserMenuOpen(false)}
                            >
                              <div className="flex items-center space-x-2">
                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                                </svg>
                                <span>Admin</span>
                              </div>
                            </Link>
                          )}

                          <div className="border-t border-border-primary my-1"></div>

                          <button
                            onClick={handleLogout}
                            className="w-full text-left px-4 py-2 text-sm text-content-primary hover:bg-bg-secondary transition-colors"
                          >
                            <div className="flex items-center space-x-2">
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                              </svg>
                              <span>Logout</span>
                            </div>
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* Mobile: Hamburger Menu Button - Hidden on Desktop */}
                <button
                  onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
                  className="md:hidden p-2 rounded-md text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                  aria-label="Menu"
                  data-faro-user-action-name="open-mobile-menu"
                >
                  {isMobileMenuOpen ? (
                    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  ) : (
                    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                    </svg>
                  )}
                </button>
              </div>
            </div>

            {/* Mobile Menu Dropdown */}
            {isMobileMenuOpen && (
              <div className="pb-4 pt-2 border-t border-interactive-primary-hover">
                <div className="space-y-1">
                  {/* User Info */}
                  <div className="px-3 py-2 text-sm text-white/70">
                    Signed in as <span className="font-semibold text-white">{currentUser?.username}</span>
                  </div>

                  <div className="border-t border-interactive-primary-hover my-2"></div>

                  {/* Navigation Links */}
                  <Link
                    to="/dashboard"
                    className={`block ${navLinkClass('/dashboard')}`}
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    <div className="flex items-center space-x-2">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                      </svg>
                      <span>Dashboard</span>
                    </div>
                  </Link>

                  <Link
                    to="/games"
                    className={`block ${navLinkClass('/games')}`}
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    <div className="flex items-center space-x-2">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                      </svg>
                      <span>Games</span>
                    </div>
                  </Link>

                  <a
                    href="/docs/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="block px-3 py-2 rounded-md text-sm font-medium text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    <div className="flex items-center space-x-2">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <span>Help</span>
                    </div>
                  </a>

                  <div className="border-t border-interactive-primary-hover my-2"></div>

                  {/* Settings */}
                  <Link
                    to="/settings"
                    className={`block ${navLinkClass('/settings')}`}
                    onClick={() => setIsMobileMenuOpen(false)}
                  >
                    <div className="flex items-center space-x-2">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                      </svg>
                      <span>Settings</span>
                    </div>
                  </Link>

                  {/* Admin Link (only visible to admins) */}
                  {currentUser?.is_admin && (
                    <Link
                      to="/admin"
                      className={`block ${navLinkClass('/admin')}`}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <div className="flex items-center space-x-2">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                        </svg>
                        <span>Admin</span>
                      </div>
                    </Link>
                  )}

                  <div className="border-t border-interactive-primary-hover my-2"></div>

                  {/* Logout */}
                  <button
                    onClick={() => {
                      handleLogout();
                      setIsMobileMenuOpen(false);
                    }}
                    className="w-full text-left px-3 py-2 rounded-md text-sm font-medium text-white/90 hover:bg-interactive-primary-hover hover:text-white transition-colors"
                  >
                    <div className="flex items-center space-x-2">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                      </svg>
                      <span>Logout</span>
                    </div>
                  </button>
                </div>
              </div>
            )}
          </div>
        </nav>
      )}

      {/* Admin Mode Banner */}
      <AdminBanner />

      {/* Email Verification Banner */}
      {isAuthenticated && currentUser && !currentUser.email_verified && (
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2">
          <EmailVerificationBanner />
        </div>
      )}

      {/* Main Content */}
      <main className="py-2">
        {children}
      </main>

      {/* Footer */}
      <footer className="surface-base border-t border-theme-default mt-auto">
        <div className="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
          <p className="text-center text-sm text-content-secondary">
            &copy; 2025 ActionPhase. A collaborative role-playing platform.
          </p>
          <p className="text-center text-sm text-content-secondary mt-2">
            <Link to="/community-guidelines" className="hover:underline">
              Community Guidelines
            </Link>
          </p>
          <p className="text-center text-xs text-content-secondary mt-1">
            <a
              href="https://www.flaticon.com/free-icons/roleplay"
              title="roleplay icons"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:underline"
            >
              Favicon created by Three musketeers - Flaticon
            </a>
          </p>
        </div>
      </footer>
    </div>
  );
};
