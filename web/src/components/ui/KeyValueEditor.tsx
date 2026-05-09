import type { InfoEntry } from "@/lib/types";
import "./KeyValueEditor.css";

interface KeyValueEditorProps {
  label: string;
  entries: InfoEntry[];
  onChange: (entries: InfoEntry[]) => void;
}

export function KeyValueEditor({
  label,
  entries,
  onChange,
}: KeyValueEditorProps) {
  const update = (index: number, field: "key" | "value", val: string) => {
    const next = entries.map((e, i) =>
      i === index ? { ...e, [field]: val } : e
    );
    onChange(next);
  };

  const add = () => {
    onChange([...entries, { key: "", value: "" }]);
  };

  const remove = (index: number) => {
    onChange(entries.filter((_, i) => i !== index));
  };

  return (
    <div className="kv-editor">
      <div className="kv-editor__header">
        <span className="input-group__label">{label}</span>
        <button type="button" className="kv-editor__add" onClick={add}>
          + Add
        </button>
      </div>
      {entries.length === 0 && (
        <p className="kv-editor__empty">No entries yet</p>
      )}
      {entries.map((entry, i) => (
        <div key={i} className="kv-editor__row">
          <input
            className="kv-editor__key"
            value={entry.key}
            onChange={(e) => update(i, "key", e.target.value)}
            placeholder="Key"
          />
          <input
            className="kv-editor__value"
            value={entry.value}
            onChange={(e) => update(i, "value", e.target.value)}
            placeholder="Value"
          />
          <button
            type="button"
            className="kv-editor__remove"
            onClick={() => remove(i)}
            aria-label="Remove entry"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}
