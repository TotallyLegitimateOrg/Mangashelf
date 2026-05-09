import "./Badge.css";

interface BadgeProps {
  children: string;
  variant?: "default" | "accent" | "success" | "warning" | "error" | "info";
}

const variantMap: Record<string, BadgeProps["variant"]> = {
  SAFE: "success",
  MATURE: "warning",
  ADULT: "error",
  Ongoing: "info",
  Completed: "success",
  Hiatus: "warning",
  Cancelled: "error",
};

export function Badge({ children, variant }: BadgeProps) {
  const resolved = variant ?? variantMap[children] ?? "default";
  return <span className={`badge badge--${resolved}`}>{children}</span>;
}
