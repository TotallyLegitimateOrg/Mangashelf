import "./UrlListEditor.css";

interface UrlListEditorProps {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  hint?: string;
}

export function UrlListEditor({
  label,
  values,
  onChange,
  placeholder = "https://…",
  hint,
}: UrlListEditorProps) {
  const update = (index: number, val: string) => {
    const next = [...values];
    next[index] = val;
    onChange(next);
  };

  const add = () => onChange([...values, ""]);

  const remove = (index: number) => onChange(values.filter((_, i) => i !== index));

  return (
    <div className="url-list">
      <div className="url-list__header">
        <span className="input-group__label">{label} ({values.length})</span>
        <button type="button" className="kv-editor__add" onClick={add}>
          + Add
        </button>
      </div>
      {hint && <p className="input-group__hint">{hint}</p>}
      {values.length === 0 && (
        <p className="kv-editor__empty">No URLs added</p>
      )}
      {values.map((url, i) => (
        <div key={i} className="url-list__row">
          <span className="url-list__num">{i + 1}</span>
          <input
            className="url-list__input"
            value={url}
            onChange={(e) => update(i, e.target.value)}
            placeholder={placeholder}
          />
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="url-list__link"
              title="Open in new tab"
            >
              ↗
            </a>
          )}
          <button
            type="button"
            className="kv-editor__remove"
            onClick={() => remove(i)}
            aria-label="Remove URL"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}
