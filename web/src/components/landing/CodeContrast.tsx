const BEFORE = `// Before — direct call, flag check scattered
if (flags.isEnabled('new-checkout')) {
  newCheckout(ctx);
} else {
  oldCheckout(ctx);
}`;

const AFTER = `// After — centralized dispatch
ds.register('new-checkout', { on: newCheckout, off: oldCheckout });

ds.dispatch('new-checkout', ctx);`;

export default function CodeContrast() {
  return (
    <section className="code-contrast">
      <div className="code-contrast-inner">
        <h2 className="section-heading">One call site per flag</h2>
        <p className="section-sub">
          The SDK becomes the single source of truth for which function runs and when.
        </p>
        <div className="code-pair">
          <pre className="code-block code-before"><code>{BEFORE}</code></pre>
          <pre className="code-block code-after"><code>{AFTER}</code></pre>
        </div>
      </div>
    </section>
  );
}
