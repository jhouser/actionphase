import { Link, Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

export const HomePage = () => {
  const { isAuthenticated } = useAuth();

  // Redirect authenticated users to dashboard
  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div className="min-h-screen bg-surface-sunken">
      {/* Header */}
      <div className="bg-surface-base shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center py-6">
            <div>
              <h1 className="text-3xl font-bold text-content-primary">ActionPhase</h1>
              <p className="text-content-secondary mt-1">Play-by-Post RPG Platform</p>
            </div>

            <div className="flex items-center space-x-4">
              <Link
                to="/login"
                className="bg-interactive-primary text-white px-4 py-2 rounded-md hover:bg-interactive-primary focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:ring-offset-2"
              >
                Login
              </Link>
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
        {/* Hero CTA */}
        <div className="mb-8 bg-surface-base shadow rounded-lg overflow-hidden">
          <div className="px-8 py-12 text-center">
            <h2 className="text-3xl font-bold text-content-primary mb-4">
              Welcome to ActionPhase
            </h2>
            <p className="text-lg text-content-secondary mb-8 max-w-2xl mx-auto">
              A collaborative storytelling platform for play-by-post RPG games.
              Join ongoing campaigns or create your own adventure.
            </p>
            <Link
              to="/login"
              className="inline-block bg-interactive-primary text-white px-8 py-4 rounded-md text-lg font-semibold hover:bg-interactive-primary-hover focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:ring-offset-2 transition-colors"
            >
              Get Started - Sign Up or Login
            </Link>
          </div>
        </div>

        {/* Info Section */}
        <div className="bg-surface-base shadow rounded-lg">
          <div className="px-6 py-8">
            <h3 className="text-xl font-semibold text-content-primary mb-4">
              What is ActionPhase?
            </h3>
            <div className="prose dark:prose-invert text-content-secondary max-w-none">
              <p className="mb-4">
                ActionPhase is a specialized platform for play-by-post RPG games that alternate between
                two distinct phases:
              </p>

              <div className="grid md:grid-cols-2 gap-6 mt-6">
                <div className="bg-semantic-info-subtle p-6 rounded-lg">
                  <h4 className="font-semibold text-semantic-info mb-2">Common Room Phase</h4>
                  <p className="text-semantic-info text-sm">
                    Asynchronous discussion where players interact in-character, plan strategies,
                    and develop relationships. No time pressure - take your time to craft the
                    perfect response.
                  </p>
                </div>

                <div className="bg-semantic-success-subtle p-6 rounded-lg">
                  <h4 className="font-semibold text-semantic-success mb-2">Action Phase</h4>
                  <p className="text-semantic-success text-sm">
                    Players submit their moves privately to the Game Master, who processes
                    all actions and reveals the results. Strategic, decisive, and exciting!
                  </p>
                </div>
              </div>

              <div className="mt-8 p-6 bg-surface-raised rounded-lg text-center not-prose">
                <p className="text-content-secondary mb-4">
                  Ready to join the adventure?
                </p>
                <Link
                  to="/login"
                  className="inline-block bg-interactive-primary text-white px-6 py-3 rounded-md hover:bg-interactive-primary-hover focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:ring-offset-2"
                >
                  Sign Up or Login
                </Link>
                <p className="text-sm text-content-secondary mt-4">
                  <Link to="/site-guidelines" className="hover:underline">
                    Read our Site Guidelines
                  </Link>
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
