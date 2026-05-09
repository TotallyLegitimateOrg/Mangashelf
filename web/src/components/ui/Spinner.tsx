import "./Spinner.css";

interface SpinnerProps {
  size?: number;
  className?: string;
}

export function Spinner({ size = 32, className = "" }: SpinnerProps) {
  return (
    <div
      className={`spinner ${className}`}
      style={{ width: size, height: size }}
      role="status"
      aria-label="Loading"
    >
      <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          strokeOpacity="0.15"
          strokeWidth="3"
        />
        <path
          d="M12 2a10 10 0 0 1 10 10"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
    </div>
  );
}

export function PageSpinner() {
  return (
    <div className="page-spinner">
      <Spinner size={40} />
    </div>
  );
}
