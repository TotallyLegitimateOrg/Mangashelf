import { type TextareaHTMLAttributes, useId } from "react";
import "./Textarea.css";

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label: string;
  error?: string;
}

export function Textarea({
  label,
  error,
  className = "",
  id: externalId,
  ...props
}: TextareaProps) {
  const autoId = useId();
  const id = externalId ?? autoId;

  return (
    <div className={`input-group ${error ? "input-group--error" : ""} ${className}`}>
      <label htmlFor={id} className="input-group__label">
        {label}
      </label>
      <textarea id={id} className="textarea__field" rows={4} {...props} />
      {error && <p className="input-group__error">{error}</p>}
    </div>
  );
}
