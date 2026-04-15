import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { PageShell } from './PageShell';

afterEach(() => {
  cleanup();
});

describe('PageShell', () => {
  it('renders children', () => {
    // Arrange & Act
    render(<PageShell><p>Hello</p></PageShell>);

    // Assert
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });

  it('renders Reign heading', () => {
    // Arrange & Act
    render(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.getByRole('heading', { name: /reign/i })).toBeInTheDocument();
  });

  it('renders dark mode toggle', () => {
    // Arrange & Act
    render(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('dark-mode-toggle')).toBeInTheDocument();
  });

  it('does not render back button when onBack is not provided', () => {
    // Arrange & Act
    render(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.queryByTestId('back-button')).not.toBeInTheDocument();
  });

  it('renders back button when onBack is provided', () => {
    // Arrange
    const onBack = vi.fn();

    // Act
    render(<PageShell onBack={onBack}><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('back-button')).toBeInTheDocument();
  });

  it('calls onBack when back button is clicked', () => {
    // Arrange
    const onBack = vi.fn();
    render(<PageShell onBack={onBack}><p>content</p></PageShell>);

    // Act
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert
    expect(onBack).toHaveBeenCalledOnce();
  });

  it('uses custom backLabel for aria-label', () => {
    // Arrange & Act
    render(<PageShell onBack={vi.fn()} backLabel="Go back"><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('back-button')).toHaveAttribute('aria-label', 'Go back');
  });

  it('toggles dark mode on click', () => {
    // Arrange
    render(<PageShell><p>content</p></PageShell>);
    const toggle = screen.getByTestId('dark-mode-toggle');
    const initialLabel = toggle.getAttribute('aria-label');

    // Act
    fireEvent.click(toggle);

    // Assert
    expect(toggle.getAttribute('aria-label')).not.toEqual(initialLabel);
  });
});
