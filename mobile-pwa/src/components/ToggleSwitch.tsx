type Size = 'sm' | 'md';

export function ToggleSwitch({
  checked,
  onChange,
  ariaLabel,
  size = 'md',
  disabled = false,
  loading = false,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  ariaLabel: string;
  size?: Size;
  disabled?: boolean;
  loading?: boolean;
}) {
  return (
    <label
      className="m-toggle"
      data-size={size}
      data-loading={loading ? 'true' : undefined}
    >
      <input
        type="checkbox"
        role="switch"
        aria-label={ariaLabel}
        checked={checked}
        disabled={disabled || loading}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="m-toggle-track" aria-hidden="true">
        <span className="m-toggle-thumb" aria-hidden="true" />
      </span>
    </label>
  );
}
