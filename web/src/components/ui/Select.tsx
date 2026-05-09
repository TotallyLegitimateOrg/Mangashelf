import { type SelectHTMLAttributes, useId } from "react";
import "./Select.css";

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label: string;
  options: { value: string; label: string }[];
  error?: string;
}

export function Select({
  label,
  options,
  error,
  className = "",
  id: externalId,
  ...props
}: SelectProps) {
  const autoId = useId();
  const id = externalId ?? autoId;

  return (
    <div className={`input-group ${error ? "input-group--error" : ""} ${className}`}>
      <label htmlFor={id} className="input-group__label">
        {label}
      </label>
      <div className="select__wrapper">
        <select id={id} className="select__field" {...props}>
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
        <span className="select__chevron">▾</span>
      </div>
      {error && <p className="input-group__error">{error}</p>}
    </div>
  );
}
