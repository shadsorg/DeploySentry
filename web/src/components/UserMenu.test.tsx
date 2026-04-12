import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import UserMenu from './UserMenu';

const mockLogout = vi.fn();
const mockNavigate = vi.fn();

vi.mock('@/authHooks', () => ({
  useAuth: () => ({
    user: { id: '1', email: 'jane.doe@example.com', name: 'Jane Doe' },
    loading: false,
    logout: mockLogout,
    login: vi.fn(),
    register: vi.fn(),
  }),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({ orgSlug: 'acme' }),
  };
});

function renderMenu() {
  return render(
    <MemoryRouter>
      <UserMenu />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockLogout.mockReset();
  mockNavigate.mockReset();
  localStorage.clear();
});

describe('UserMenu', () => {
  it('renders initials from the user name', () => {
    renderMenu();
    expect(screen.getByRole('button', { name: /user menu/i })).toHaveTextContent('JD');
  });

  it('opens the dropdown on click and shows email', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('jane.doe@example.com')).toBeInTheDocument();
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('Logout')).toBeInTheDocument();
  });

  it('closes on Escape', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('Logout')).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByText('Logout')).not.toBeInTheDocument();
  });

  it('closes on outside click', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <div>
          <UserMenu />
          <div data-testid="outside">outside</div>
        </div>
      </MemoryRouter>,
    );
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    expect(screen.getByText('Logout')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByTestId('outside'));
    expect(screen.queryByText('Logout')).not.toBeInTheDocument();
  });

  it('Settings link points to current org settings', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    const settings = screen.getByText('Settings').closest('a');
    expect(settings).toHaveAttribute('href', '/orgs/acme/settings');
  });

  it('Logout calls logout and navigates to /', async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByRole('button', { name: /user menu/i }));
    await user.click(screen.getByText('Logout'));
    expect(mockLogout).toHaveBeenCalledOnce();
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });
});
