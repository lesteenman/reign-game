import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act } from '@testing-library/react';
import { renderHook } from '@shared/test-utils';
import { useInstallPrompt } from './useInstallPrompt';

type Outcome = 'accepted' | 'dismissed';

function makeBeforeInstallPromptEvent(outcome: Outcome = 'accepted'): Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: Outcome }>;
  preventDefault: () => void;
} {
  const prompt = vi.fn().mockResolvedValue(undefined);
  const event = new Event('beforeinstallprompt') as unknown as Event & {
    prompt: () => Promise<void>;
    userChoice: Promise<{ outcome: Outcome }>;
    preventDefault: () => void;
  };
  Object.assign(event, {
    prompt,
    userChoice: Promise.resolve({ outcome }),
  });
  return event;
}

describe('useInstallPrompt', () => {
  let originalMatchMedia: typeof window.matchMedia;

  beforeEach(() => {
    originalMatchMedia = window.matchMedia;
  });

  afterEach(() => {
    window.matchMedia = originalMatchMedia;
  });

  it('starts with canInstall=false and isStandalone=false in jsdom', () => {
    // Arrange — matchMedia mock from test-setup.ts returns matches:false
    // Act
    const { result } = renderHook(() => useInstallPrompt());

    // Assert
    expect(result.current.canInstall).toBe(false);
    expect(result.current.isStandalone).toBe(false);
  });

  it('detects standalone display mode via matchMedia', () => {
    // Arrange
    window.matchMedia = ((query: string) => ({
      matches: query === '(display-mode: standalone)',
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;

    // Act
    const { result } = renderHook(() => useInstallPrompt());

    // Assert
    expect(result.current.isStandalone).toBe(true);
  });

  it('flips canInstall to true when beforeinstallprompt fires', () => {
    // Arrange
    const { result } = renderHook(() => useInstallPrompt());
    expect(result.current.canInstall).toBe(false);

    // Act
    act(() => {
      window.dispatchEvent(makeBeforeInstallPromptEvent());
    });

    // Assert
    expect(result.current.canInstall).toBe(true);
  });

  it('promptInstall returns "unavailable" when no event was captured', async () => {
    // Arrange
    const { result } = renderHook(() => useInstallPrompt());

    // Act
    let outcome: string | undefined;
    await act(async () => {
      outcome = await result.current.promptInstall();
    });

    // Assert
    expect(outcome).toBe('unavailable');
  });

  it('promptInstall calls event.prompt() and returns user choice outcome', async () => {
    // Arrange
    const { result } = renderHook(() => useInstallPrompt());
    const event = makeBeforeInstallPromptEvent('accepted');
    act(() => { window.dispatchEvent(event); });
    expect(result.current.canInstall).toBe(true);

    // Act
    let outcome: string | undefined;
    await act(async () => {
      outcome = await result.current.promptInstall();
    });

    // Assert
    expect(event.prompt).toHaveBeenCalledOnce();
    expect(outcome).toBe('accepted');
    expect(result.current.canInstall).toBe(false); // consumed
  });

  it('appinstalled event marks the app standalone and clears canInstall', () => {
    // Arrange
    const { result } = renderHook(() => useInstallPrompt());
    act(() => { window.dispatchEvent(makeBeforeInstallPromptEvent()); });
    expect(result.current.canInstall).toBe(true);

    // Act
    act(() => { window.dispatchEvent(new Event('appinstalled')); });

    // Assert
    expect(result.current.isStandalone).toBe(true);
    expect(result.current.canInstall).toBe(false);
  });
});
