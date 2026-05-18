import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { act } from '@testing-library/react';
import { render, screen, cleanup } from '../../test-utils';
import { OfflineBanner } from './OfflineBanner';

describe('OfflineBanner', () => {
  let originalDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalDescriptor = Object.getOwnPropertyDescriptor(window.navigator, 'onLine');
  });

  afterEach(() => {
    cleanup();
    if (originalDescriptor) {
      Object.defineProperty(window.navigator, 'onLine', originalDescriptor);
    } else {
      // onLine was on the prototype; remove the own-property override we set.
      delete (window.navigator as unknown as { onLine?: boolean }).onLine;
    }
  });

  it('renders nothing when online', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });

    // Act
    render(<OfflineBanner />);

    // Assert
    expect(screen.queryByTestId('offline-banner')).not.toBeInTheDocument();
  });

  it('renders a role=status banner when offline', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });

    // Act
    render(<OfflineBanner />);

    // Assert
    const banner = screen.getByTestId('offline-banner');
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveAttribute('role', 'status');
    expect(banner).not.toHaveAttribute('aria-live');
  });

  it('toggles visibility when window dispatches online/offline events', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });
    render(<OfflineBanner />);
    expect(screen.queryByTestId('offline-banner')).not.toBeInTheDocument();

    // Act — go offline
    act(() => {
      Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });
      window.dispatchEvent(new Event('offline'));
    });
    // Assert offline visible
    expect(screen.getByTestId('offline-banner')).toBeInTheDocument();

    // Act — back online
    act(() => {
      Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });
      window.dispatchEvent(new Event('online'));
    });
    // Assert hidden again
    expect(screen.queryByTestId('offline-banner')).not.toBeInTheDocument();
  });
});
