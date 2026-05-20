import { render, screen, cleanup, fireEvent } from '@shared/test-utils';
import { describe, it, expect, afterEach, vi } from 'vitest';
import {
  PrimaryButton,
  SecondaryButton,
  GhostButton,
  CompactSecondaryButton,
} from './Button';

afterEach(() => {
  cleanup();
});

describe('Button — disabled behaviour', () => {
  it.each([
    ['PrimaryButton', PrimaryButton],
    ['SecondaryButton', SecondaryButton],
    ['GhostButton', GhostButton],
    ['CompactSecondaryButton', CompactSecondaryButton],
  ] as const)(
    '%s sets the disabled DOM attribute and suppresses onClick',
    (_name, Component) => {
      // Arrange
      const onClick = vi.fn();
      render(
        <Component disabled onClick={onClick} data-testid="btn">
          Disabled
        </Component>,
      );

      // Act
      const button = screen.getByTestId('btn');
      fireEvent.click(button);

      // Assert — disabled attribute prevents the click handler from firing.
      // Tamagui's `disabled` prop forwards to the native `<button>` element,
      // and the browser short-circuits click dispatch for disabled buttons.
      expect(button).toBeDisabled();
      expect(onClick).not.toHaveBeenCalled();
    },
  );

  it.each([
    ['PrimaryButton', PrimaryButton],
    ['SecondaryButton', SecondaryButton],
    ['GhostButton', GhostButton],
    ['CompactSecondaryButton', CompactSecondaryButton],
  ] as const)(
    '%s fires onClick when enabled',
    (_name, Component) => {
      // Arrange
      const onClick = vi.fn();
      render(
        <Component onClick={onClick} data-testid="btn">
          Enabled
        </Component>,
      );

      // Act
      fireEvent.click(screen.getByTestId('btn'));

      // Assert
      expect(onClick).toHaveBeenCalledTimes(1);
    },
  );
});

describe('Button — prop propagation', () => {
  it('defaults to type="button" via the underlying render element', () => {
    // Arrange
    render(<PrimaryButton data-testid="btn">Click</PrimaryButton>);

    // Act + Assert — the styled component's `render: <button type="button" />`
    // bakes type="button" into every Button instance so it stays out of any
    // surrounding form's submit chain. Type="submit" is not exposed on the
    // wrapper today (no current consumer needs it; Tamagui's `type` style
    // prop would shadow the HTML attribute — see Button.tsx note).
    expect(screen.getByTestId('btn')).toHaveAttribute('type', 'button');
  });

  it('passes aria-label through to the rendered button', () => {
    // Arrange
    render(
      <PrimaryButton aria-label="dismiss" data-testid="btn">
        ×
      </PrimaryButton>,
    );

    // Act + Assert
    expect(screen.getByTestId('btn')).toHaveAttribute('aria-label', 'dismiss');
  });

  it('renders children inside the button', () => {
    // Arrange
    render(<SecondaryButton data-testid="btn">Hello world</SecondaryButton>);

    // Act + Assert
    expect(screen.getByTestId('btn')).toHaveTextContent('Hello world');
  });
});
