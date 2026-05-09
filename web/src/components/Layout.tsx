import { NavLink, Outlet, useNavigate } from "react-router";
import { useAuth } from "@/lib/auth";
import { useState } from "react";
import "./Layout.css";

const NAV_ITEMS = [
  { to: "/", label: "Library" },
  { to: "/paperback", label: "Paperback" },
  { to: "/settings", label: "Settings" },
];

export function Layout() {
  const { logout } = useAuth();
  const navigate = useNavigate();

  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const handleLogout = () => {
    setIsLoggingOut(true);
    setTimeout(() => {
      logout();
      navigate("/login");
    }, 300);
  };

  return (
    <div className={`layout ${isLoggingOut ? "layout--exiting" : ""}`}>
      {/* Sidebar */}
      <aside className="sidebar">
        {/* Brand — typographic wordmark only */}
        <NavLink to="/" className="sidebar__brand">
          <span className="sidebar__wordmark font-display">M</span>
          <span className="sidebar__wordmark-full font-display">angashelf</span>
        </NavLink>

        {/* Navigation — stacked text links */}
        <nav className="sidebar__nav">
          {NAV_ITEMS.map((item, index) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                `sidebar__link ${isActive ? "sidebar__link--active" : ""}`
              }
              style={{ "--nav-index": index } as React.CSSProperties}
            >
              <span className="sidebar__link-label">{item.label}</span>
              <span className="sidebar__link-indicator" />
            </NavLink>
          ))}
        </nav>

        {/* Bottom — just a sign-out action */}
        <div className="sidebar__bottom">
          <button
            className="sidebar__logout"
            onClick={handleLogout}
            title="Sign out"
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Mobile bottom bar — text only */}
      <nav className="mobile-nav">
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              `mobile-nav__link ${isActive ? "mobile-nav__link--active" : ""}`
            }
          >
            <span className="mobile-nav__label">{item.label}</span>
          </NavLink>
        ))}
      </nav>

      {/* Main content */}
      <main className="layout__main">
        <Outlet />
      </main>
    </div>
  );
}
