import { type InputHTMLAttributes, useId } from "react";
import "./Input.css";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  hint?: string;
}

export function Input({
  label,
  error,
  hint,
  className = "",
  id: externalId,
  ...props
}: InputProps) {
  const autoId = useId();
  const id = externalId ?? autoId;

  return (
    <div className={`input-group ${error ? "input-group--error" : ""} ${className}`}>
      <label htmlFor={id} className="input-group__label">
        {label}
      </label>
      <input id={id} className="input-group__field" {...props} />
      {error && <p className="input-group__error">{error}</p>}
      {!error && hint && <p className="input-group__hint">{hint}</p>}
    </div>
  );
}
