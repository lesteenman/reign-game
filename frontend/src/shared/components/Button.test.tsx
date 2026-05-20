import { render, screen, cleanup } from '@shared/test-utils';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { PrimaryButton, SecondaryButton, GhostButton } from './Button';

afterEach(() => {
  cleanup();
});

describe('Button — disabled visual state', () => {
  it.each([
    ['PrimaryButton', PrimaryButton],
    ['SecondaryButton', SecondaryButton],
    ['GhostButton', GhostButton],
  ] as const)('%s applies reduced opacity and not-allowed cursor when disabled', (_name, Component) => {
    // Arrange
    render(
      <Component disabled data-testid="btn">
        Disabled
      </Component>,
    );

    // Act
    const button = screen.getByTestId('btn');

    // Assert — disabled state must be visually distinguishable per
    // BRAND_GUIDELINES (reduced opacity + not-allowed cursor).
    expect(button).toBeDisabled();
    expect(button.style.opacity).toBe('0.4');
    expect(button.style.cursor).toBe('not-allowed');
  });

  it.each([
    ['PrimaryButton', PrimaryButton],
    ['SecondaryButton', SecondaryButton],
    ['GhostButton', GhostButton],
  ] as const)('%s renders default opacity and pointer cursor when enabled', (_name, Component) => {
    // Arrange
    render(
      <Component data-testid="btn">
        Enabled
      </Component>,
    );

    // Act
    const button = screen.getByTestId('btn');

    // Assert
    expect(button).not.toBeDisabled();
    expect(button.style.opacity).not.toBe('0.4');
    expect(button.style.cursor).toBe('pointer');
  });

  it('PrimaryButton hover handler is a no-op when disabled (no press-in transform)', () => {
    // Arrange
    render(
      <PrimaryButton disabled onClick={vi.fn()} data-testid="btn">
        Disabled
      </PrimaryButton>,
    );
    const button = screen.getByTestId('btn');

    // Act — fire a mouse-enter to trigger pressIn; if disabled-guard is
    // missing, this would apply translateY(1px). We rely on the inline
    // style attribute to detect the leak.
    button.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));

    // Assert — disabled buttons must not animate on hover.
    expect(button.style.transform).not.toContain('translateY');
  });

  it('SecondaryButton hover handler is a no-op when disabled', () => {
    // Arrange
    render(
      <SecondaryButton disabled onClick={vi.fn()} data-testid="btn">
        Disabled
      </SecondaryButton>,
    );
    const button = screen.getByTestId('btn');

    // Act
    button.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));

    // Assert
    expect(button.style.transform).not.toContain('translateY');
  });
});
