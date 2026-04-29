import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RuleEditSheet } from './RuleEditSheet';
import { setFetch } from '../api';
import type { TargetingRule } from '../types';

function makeRule(overrides: Partial<TargetingRule>): TargetingRule {
  return {
    id: 'r1',
    flag_id: 'f1',
    value: '',
    priority: 1,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('RuleEditSheet', () => {
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;

  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>();
    setFetch(fetchMock);
    localStorage.setItem('ds_token', 'header.payload.sig');
  });

  it('renders nothing when open is false', () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 25 });
    const { container } = render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open={false}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('percentage: typing in numeric input updates slider and preview, Save calls updateRule', async () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 10 });
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ ...rule, percentage: 25 }),
        { status: 200 },
      ),
    );
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={onClose}
        onSaved={onSaved}
      />,
    );
    const numeric = screen.getByLabelText('Percentage') as HTMLInputElement;
    const slider = screen.getByLabelText('Percentage slider') as HTMLInputElement;
    await userEvent.clear(numeric);
    await userEvent.type(numeric, '25');
    expect(numeric.value).toBe('25');
    expect(slider.value).toBe('25');
    expect(screen.getByTestId('rule-preview').textContent).toBe('25% rollout');

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/flags/f1/rules/r1');
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe('PUT');
    expect(init.body).toBe(JSON.stringify({ percentage: 25 }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });

  it('user_target: Enter and comma add chips, tapping a chip removes it, Save sends user_ids', async () => {
    const rule = makeRule({ rule_type: 'user_target', user_ids: [] });
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ ...rule, user_ids: ['bob'] }),
        { status: 200 },
      ),
    );
    const onSaved = vi.fn();
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={onSaved}
      />,
    );
    const input = screen.getByLabelText('Add user ID') as HTMLInputElement;
    await userEvent.type(input, 'alice{enter}');
    expect(screen.getByRole('button', { name: 'Remove alice' })).toBeInTheDocument();
    expect(input.value).toBe('');

    await userEvent.type(input, 'bob,');
    expect(screen.getByRole('button', { name: 'Remove bob' })).toBeInTheDocument();
    expect(input.value).toBe('');

    await userEvent.click(screen.getByRole('button', { name: 'Remove alice' }));
    expect(screen.queryByRole('button', { name: 'Remove alice' })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe('PUT');
    expect(init.body).toBe(JSON.stringify({ user_ids: ['bob'] }));
  });

  it('attribute: changing operator and value sends both fields on Save', async () => {
    const rule = makeRule({
      rule_type: 'attribute',
      attribute: 'plan',
      operator: 'eq',
      value: 'pro',
    });
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ ...rule, operator: 'neq', value: 'free' }),
        { status: 200 },
      ),
    );
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    const operator = screen.getByLabelText('Operator') as HTMLSelectElement;
    await userEvent.selectOptions(operator, 'neq');
    const value = screen.getByLabelText('Value') as HTMLInputElement;
    await userEvent.clear(value);
    await userEvent.type(value, 'free');

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe('PUT');
    expect(init.body).toBe(JSON.stringify({ operator: 'neq', value: 'free' }));
  });

  it('segment: setting segment_id calls updateRule with segment_id', async () => {
    const rule = makeRule({ rule_type: 'segment', segment_id: '' });
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ ...rule, segment_id: 'beta-testers' }),
        { status: 200 },
      ),
    );
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    const input = screen.getByLabelText('Segment ID') as HTMLInputElement;
    await userEvent.type(input, 'beta-testers');

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.body).toBe(JSON.stringify({ segment_id: 'beta-testers' }));
  });

  it('schedule: setting start and end times sends both fields', async () => {
    const rule = makeRule({
      rule_type: 'schedule',
      start_time: '',
      end_time: '',
    });
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          ...rule,
          start_time: '2026-05-01T10:00',
          end_time: '2026-05-02T10:00',
        }),
        { status: 200 },
      ),
    );
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    const start = screen.getByLabelText('Start time') as HTMLInputElement;
    const end = screen.getByLabelText('End time') as HTMLInputElement;
    await userEvent.type(start, '2026-05-01T10:00');
    await userEvent.type(end, '2026-05-02T10:00');

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.body).toBe(
      JSON.stringify({
        start_time: '2026-05-01T10:00',
        end_time: '2026-05-02T10:00',
      }),
    );
  });

  it('Cancel discards edits and calls onClose without onSaved or PUT', async () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 10 });
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={onClose}
        onSaved={onSaved}
      />,
    );
    const numeric = screen.getByLabelText('Percentage') as HTMLInputElement;
    await userEvent.clear(numeric);
    await userEvent.type(numeric, '90');

    await userEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('on a 500 response, sheet stays open and renders error inline', async () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 10 });
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: 'Server boom' }), { status: 500 }),
    );
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={onClose}
        onSaved={onSaved}
      />,
    );
    const numeric = screen.getByLabelText('Percentage') as HTMLInputElement;
    await userEvent.clear(numeric);
    await userEvent.type(numeric, '50');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(screen.getByRole('alert').textContent).toMatch(/Server boom/),
    );
    expect(onSaved).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('Save button is disabled when no fields have changed', () => {
    const rule = makeRule({ rule_type: 'percentage', percentage: 10 });
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    const save = screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
  });

  it('compound: shows "edit on desktop" message and disables Save', () => {
    const rule = makeRule({ rule_type: 'compound' });
    render(
      <RuleEditSheet
        rule={rule}
        flagId="f1"
        open
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    expect(screen.getByText('Compound rules must be edited on desktop.')).toBeInTheDocument();
    const save = screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
  });
});
