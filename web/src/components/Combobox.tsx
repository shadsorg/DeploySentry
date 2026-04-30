import { useEffect, useMemo, useRef, useState } from 'react';

export interface ComboboxOption {
  value: string;
  label?: string;
  hint?: string;
}

export interface ComboboxProps {
  value: string;
  onChange: (next: string) => void;
  options: ComboboxOption[];
  placeholder?: string;
  disabled?: boolean;
  // When true, typing values not in `options` still resolves via onChange.
  // We don't have a "strict" mode because the deploy form explicitly
  // wants free-text entry for brand-new artifacts/versions.
  id?: string;
  className?: string;
}

/**
 * A small combobox: a text input paired with a filterable dropdown of
 * suggestions. Always accepts free-text entry; when the typed value has
 * no match in `options`, a "will create new" hint appears below the
 * input so the user knows they're ahead of the system.
 *
 * Keyboard:
 *   ArrowDown / ArrowUp — move highlight
 *   Enter               — commit highlighted option (or keep typed text)
 *   Escape              — close dropdown without changing value
 */
export default function Combobox({
  value,
  onChange,
  options,
  placeholder,
  disabled,
  id,
  className,
}: ComboboxProps) {
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const filtered = useMemo(() => {
    const q = value.trim().toLowerCase();
    if (!q) return options;
    return options.filter(
      (o) => o.value.toLowerCase().includes(q) || (o.label && o.label.toLowerCase().includes(q)),
    );
  }, [options, value]);

  // Reset highlight when the filtered list shrinks.
  useEffect(() => {
    if (highlight >= filtered.length) setHighlight(0);
  }, [filtered.length, highlight]);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    function onDocDown(e: MouseEvent) {
      if (
        inputRef.current &&
        !inputRef.current.contains(e.target as Node) &&
        listRef.current &&
        !listRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onDocDown);
    return () => document.removeEventListener('mousedown', onDocDown);
  }, [open]);

  const hasExactMatch = options.some((o) => o.value === value);
  const showCreateHint = value.trim().length > 0 && !hasExactMatch;

  function commit(nextValue: string) {
    onChange(nextValue);
    setOpen(false);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setOpen(true);
      setHighlight((h) => Math.min(h + 1, filtered.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === 'Enter') {
      if (open && filtered[highlight]) {
        e.preventDefault();
        commit(filtered[highlight].value);
      }
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  }

  return (
    <div className={`combobox ${className ?? ''}`} style={{ position: 'relative' }}>
      <input
        ref={inputRef}
        id={id}
        className="form-input"
        type="text"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        placeholder={placeholder}
        value={value}
        disabled={disabled}
        onChange={(e) => {
          onChange(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
      />
      {open && filtered.length > 0 && (
        <ul
          ref={listRef}
          role="listbox"
          style={{
            position: 'absolute',
            zIndex: 50,
            top: 'calc(100% + 2px)',
            left: 0,
            right: 0,
            maxHeight: 240,
            overflowY: 'auto',
            margin: 0,
            padding: 0,
            listStyle: 'none',
            background: 'var(--color-bg, #fff)',
            border: '1px solid var(--color-border, #ddd)',
            borderRadius: 4,
            boxShadow: '0 2px 6px rgba(0,0,0,0.08)',
          }}
        >
          {filtered.map((opt, i) => (
            <li
              key={opt.value}
              role="option"
              aria-selected={i === highlight}
              onMouseEnter={() => setHighlight(i)}
              onMouseDown={(e) => {
                // mousedown (not click) to beat the input's blur.
                e.preventDefault();
                commit(opt.value);
              }}
              style={{
                padding: '6px 10px',
                cursor: 'pointer',
                background: i === highlight ? 'var(--color-surface-hover, #f2f2f2)' : 'transparent',
              }}
            >
              <div style={{ fontWeight: 500 }}>{opt.label ?? opt.value}</div>
              {opt.hint && (
                <div style={{ fontSize: 12, color: 'var(--color-text-muted, #888)' }}>
                  {opt.hint}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {showCreateHint && (
        <div className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted, #888)' }}>
          Will create new: <code>{value}</code>
        </div>
      )}
    </div>
  );
}
