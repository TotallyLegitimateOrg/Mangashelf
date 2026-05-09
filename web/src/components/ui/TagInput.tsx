import { useState, type KeyboardEvent } from "react";
import "./TagInput.css";

interface TagInputProps {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  hint?: string;
}

export function TagInput({
  label,
  values,
  onChange,
  placeholder = "Type and press Enter",
  hint,
}: TagInputProps) {
  const [input, setInput] = useState("");

  const addValue = (raw: string) => {
    const val = raw.trim();
    if (val && !values.includes(val)) {
      onChange([...values, val]);
    }
    setInput("");
  };

  const handleKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      addValue(input);
    } else if (e.key === "Backspace" && input === "" && values.length > 0) {
      onChange(values.slice(0, -1));
    }
  };

  const remove = (index: number) => {
    onChange(values.filter((_, i) => i !== index));
  };

  return (
    <div className="tag-input-group">
      <span className="input-group__label">{label}</span>
      <div className="tag-input__container">
        {values.map((val, i) => (
          <span key={i} className="tag-input__chip">
            <span className="tag-input__chip-text truncate">{val}</span>
            <button
              type="button"
              className="tag-input__chip-remove"
              onClick={() => remove(i)}
              aria-label={`Remove ${val}`}
            >
              ✕
            </button>
          </span>
        ))}
        <input
          className="tag-input__field"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKey}
          onBlur={() => { if (input.trim()) addValue(input); }}
          placeholder={values.length === 0 ? placeholder : ""}
        />
      </div>
      {hint && <p className="input-group__hint">{hint}</p>}
    </div>
  );
}
