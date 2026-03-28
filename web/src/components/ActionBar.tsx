import { useState, useRef, useEffect } from 'react';

interface ActionBarAction {
  label: string;
  onClick: () => void;
  variant?: 'default' | 'primary' | 'danger';
}

interface ActionBarProps {
  primaryAction?: ActionBarAction;
  secondaryActions?: ActionBarAction[];
}

export default function ActionBar({ primaryAction, secondaryActions }: ActionBarProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const hasSecondary = secondaryActions && secondaryActions.length > 0;

  if (!primaryAction && !hasSecondary) return null;

  return (
    <div className="action-bar" ref={ref}>
      {primaryAction && (
        <button
          className={`btn btn-${primaryAction.variant || 'primary'}`}
          onClick={primaryAction.onClick}
        >
          {primaryAction.label}
        </button>
      )}
      {hasSecondary && (
        <div className="action-bar-more">
          <button
            className="btn btn-secondary action-bar-more-btn"
            onClick={() => setOpen(!open)}
          >
            More ▾
          </button>
          {open && (
            <div className="action-bar-dropdown">
              {secondaryActions!.map((action) => (
                <button
                  key={action.label}
                  className={`action-bar-option${action.variant === 'danger' ? ' action-bar-option-danger' : ''}`}
                  onClick={() => { action.onClick(); setOpen(false); }}
                >
                  {action.label}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export type { ActionBarProps, ActionBarAction };
