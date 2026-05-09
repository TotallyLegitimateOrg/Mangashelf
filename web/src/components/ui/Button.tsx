import type { ButtonHTMLAttributes, ReactNode } from "react";
import "./Button.css";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  loading?: boolean;
  icon?: ReactNode;
  children: ReactNode;
}

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  icon,
  children,
  className = "",
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`btn btn--${variant} btn--${size} ${loading ? "btn--loading" : ""} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && <span className="btn__spinner" />}
      {!loading && icon && <span className="btn__icon">{icon}</span>}
      <span className="btn__label">{children}</span>
    </button>
  );
}
