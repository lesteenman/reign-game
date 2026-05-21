import { render, screen, cleanup } from '@shared/test-utils';
import { describe, it, expect, afterEach } from 'vitest';
import { Card } from './Card';

afterEach(() => {
  cleanup();
});

describe('Card', () => {
  it('renders children', () => {
    // Arrange + Act
    render(<Card data-testid="card"><span>hi</span></Card>);

    // Assert
    expect(screen.getByTestId('card')).toHaveTextContent('hi');
  });

  it('passes data-testid through to the rendered element', () => {
    // Arrange + Act
    render(<Card data-testid="forbidden-card">x</Card>);

    // Assert
    expect(screen.getByTestId('forbidden-card')).toBeInTheDocument();
  });

  it.each([
    ['compact' as const],
    ['large' as const],
  ])('renders the %s size variant without crashing', (size) => {
    // Arrange + Act
    render(<Card size={size} data-testid="card">x</Card>);

    // Assert — smoke test that both variants resolve through Tamagui's
    // styled-component machinery (padding values come from the variant
    // block; no token-resolution failures).
    expect(screen.getByTestId('card')).toBeInTheDocument();
  });

  it('defaults to size="compact" when no size prop is provided', () => {
    // Arrange + Act
    const { rerender } = render(<Card data-testid="card">x</Card>);
    const initialPadding = window.getComputedStyle(
      screen.getByTestId('card'),
    ).paddingTop;

    rerender(<Card size="compact" data-testid="card">x</Card>);
    const explicitPadding = window.getComputedStyle(
      screen.getByTestId('card'),
    ).paddingTop;

    // Assert — explicit size="compact" matches the no-size default. If
    // Tamagui's `defaultVariants` regresses we'd see different padding.
    expect(explicitPadding).toBe(initialPadding);
  });
});
