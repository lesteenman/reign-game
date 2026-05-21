import { render, screen, cleanup } from '@shared/test-utils';
import { describe, it, expect, afterEach } from 'vitest';
import { Spinner } from './Spinner';

afterEach(() => {
  cleanup();
});

describe('Spinner', () => {
  it('renders an element with role="status" so screen readers announce loading', () => {
    // Arrange + Act
    render(<Spinner />);

    // Assert
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('carries aria-label="Loading" for accessible naming', () => {
    // Arrange + Act
    render(<Spinner />);

    // Assert — required because the indicator is visual-only; screen
    // readers need the text equivalent (WCAG 2.1 SC 4.1.2).
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Loading');
  });

  it('renders without crashing (smoke)', () => {
    // Arrange + Act
    const { container } = render(<Spinner />);

    // Assert — basic mount check; the @keyframes / @media rules inside
    // the inline <style> don't apply visibly in jsdom but should at
    // least parse + render the DOM node without error.
    expect(container.firstChild).toBeTruthy();
  });
});
