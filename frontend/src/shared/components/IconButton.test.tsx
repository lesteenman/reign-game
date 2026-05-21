import { render, screen, cleanup, fireEvent } from '@shared/test-utils';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { IconButton } from './IconButton';

afterEach(() => {
  cleanup();
});

describe('IconButton', () => {
  it('renders children inside the button', () => {
    // Arrange + Act
    render(
      <IconButton aria-label="open menu" data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Assert
    expect(screen.getByTestId('btn')).toHaveTextContent('x');
  });

  it('forwards aria-label to the rendered button', () => {
    // Arrange + Act
    render(
      <IconButton aria-label="close dialog" data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Assert
    expect(screen.getByTestId('btn')).toHaveAttribute('aria-label', 'close dialog');
  });

  it('fires onClick when enabled', () => {
    // Arrange
    const onClick = vi.fn();
    render(
      <IconButton aria-label="action" onClick={onClick} data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Act
    fireEvent.click(screen.getByTestId('btn'));

    // Assert
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('sets the native disabled attribute and suppresses onClick when disabled', () => {
    // Arrange
    const onClick = vi.fn();
    render(
      <IconButton aria-label="action" onClick={onClick} disabled data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Act
    fireEvent.click(screen.getByTestId('btn'));

    // Assert — the browser short-circuits click dispatch on a disabled
    // button. The migration's `render={<button disabled={x} />}` override
    // is what wires the native attribute (Tamagui's own `disabled` prop
    // only sets `aria-disabled`); this test pins that contract.
    const btn = screen.getByTestId('btn');
    expect(btn).toBeDisabled();
    expect(onClick).not.toHaveBeenCalled();
  });

  it('defaults to type="button" via the underlying render element', () => {
    // Arrange + Act
    render(
      <IconButton aria-label="action" data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Assert — keeps the icon-button out of any surrounding form's
    // submit chain by default. Same baseline as Button.tsx.
    expect(screen.getByTestId('btn')).toHaveAttribute('type', 'button');
  });

  it.each([
    ['ink' as const],
    ['muted' as const],
  ])('renders the %s variant without crashing', (variant) => {
    // Arrange + Act
    render(
      <IconButton variant={variant} aria-label="action" data-testid="btn">
        <span>x</span>
      </IconButton>,
    );

    // Assert — smoke test that both variants resolve their token-based
    // color value through the Tamagui config without throwing.
    expect(screen.getByTestId('btn')).toBeInTheDocument();
  });
});
