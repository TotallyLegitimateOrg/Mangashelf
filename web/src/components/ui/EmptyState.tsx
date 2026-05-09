import type { ReactNode } from "react";
import "./EmptyState.css";

interface EmptyStateProps {
  icon?: string;
  title: string;
  description?: string;
  action?: ReactNode;
}

export function EmptyState({
  icon = "📭",
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <div className="empty-state">
      <span className="empty-state__icon">{icon}</span>
      <h3 className="empty-state__title font-display">{title}</h3>
      {description && (
        <p className="empty-state__desc">{description}</p>
      )}
      {action && <div className="empty-state__action">{action}</div>}
    </div>
  );
}
