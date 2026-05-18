import { describe, it, expect, afterEach } from 'vitest';
import { ArrowLeft } from 'lucide-react';
import { render, screen, cleanup } from '../../test-utils';
import { Icon } from './Icon';

afterEach(() => {
  cleanup();
});

describe('Icon', () => {
  it('renders the lucide icon passed via the `as` prop', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-under-test" />);

    // Assert
    const svg = screen.getByTestId('icon-under-test');
    expect(svg).toBeInTheDocument();
    expect(svg.tagName.toLowerCase()).toBe('svg');
  });

  it('applies brand defaults: size 20, strokeWidth 1.5', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-defaults" />);

    // Assert
    const svg = screen.getByTestId('icon-defaults');
    expect(svg).toHaveAttribute('width', '20');
    expect(svg).toHaveAttribute('height', '20');
    expect(svg).toHaveAttribute('stroke-width', '1.5');
  });

  it('marks the icon aria-hidden by default (icons sit inside labeled buttons)', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-aria" />);

    // Assert
    const svg = screen.getByTestId('icon-aria');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('allows per-site override of size and strokeWidth', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} size={32} strokeWidth={2} data-testid="icon-override" />);

    // Assert
    const svg = screen.getByTestId('icon-override');
    expect(svg).toHaveAttribute('width', '32');
    expect(svg).toHaveAttribute('height', '32');
    expect(svg).toHaveAttribute('stroke-width', '2');
  });

  it('allows caller to override aria-hidden to false', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} aria-hidden={false} data-testid="icon-aria-override" />);

    // Assert
    const svg = screen.getByTestId('icon-aria-override');
    expect(svg).toHaveAttribute('aria-hidden', 'false');
  });
});
