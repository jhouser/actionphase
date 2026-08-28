import { Link, useNavigate } from 'react-router-dom';

interface SettingsSection {
  id: string;
  label: string;
  icon?: React.ReactNode;
}

interface SettingsSidebarProps {
  sections: SettingsSection[];
  activeSection: string;
}

export const SettingsSidebar = ({ sections, activeSection }: SettingsSidebarProps) => {
  const navigate = useNavigate();

  return (
    <>
      {/* Mobile: Dropdown Select */}
      <div className="md:hidden mb-6">
        <label htmlFor="section-select" className="sr-only">
          Select a section
        </label>
        <div className="relative">
          <select
            id="section-select"
            value={activeSection}
            onChange={(e) => navigate(`/settings?tab=${e.target.value}`, { replace: true })}
            className="block w-full py-3 pl-4 pr-10 text-base font-semibold surface-raised text-content-primary border border-theme-default rounded-lg shadow-sm appearance-none cursor-pointer focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:border-interactive-primary transition-all"
            style={{ backgroundImage: 'none' }}
          >
            {sections.map((section) => (
              <option key={section.id} value={section.id}>
                {section.label}
              </option>
            ))}
          </select>
          {/* Dropdown chevron icon */}
          <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
            <svg className="h-5 w-5 text-content-secondary" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clipRule="evenodd" />
            </svg>
          </div>
        </div>
      </div>

      {/* Desktop: Vertical Sidebar */}
      <nav className="hidden md:block w-64 flex-shrink-0" role="navigation">
        <div className="surface-base rounded-lg shadow-md p-2">
          {sections.map((section) => {
            const isActive = activeSection === section.id;
            return (
              <Link
                key={section.id}
                to={`/settings?tab=${section.id}`}
                replace
                className={`
                  w-full text-left py-3 px-4 rounded-lg font-medium text-sm flex items-center gap-3
                  transition-all duration-200
                  ${isActive
                    ? 'bg-interactive-primary text-white shadow-sm'
                    : 'text-content-primary hover:surface-raised'
                  }
                `}
              >
                {section.icon && (
                  <span className="flex-shrink-0">
                    {section.icon}
                  </span>
                )}
                <span>{section.label}</span>
              </Link>
            );
          })}
        </div>
      </nav>
    </>
  );
};
