import { useCallback, useEffect, useState } from 'react';

/**
 * Wraps the browser's PWA install handshake. The `beforeinstallprompt`
 * event fires when the browser determines the app is installable;
 * we capture it (calling preventDefault to prevent the browser's
 * default mini-bar so we can render our own CTA), expose a
 * `promptInstall` action that calls `prompt()` on the deferred event,
 * and listen for `appinstalled` so the UI can self-hide.
 *
 * Browsers that don't support PWA install (iOS Safari, most desktops
 * without manifest detection) never fire beforeinstallprompt — the
 * hook stays canInstall=false and the consumer should render nothing.
 */

interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

export interface InstallPromptState {
  canInstall: boolean;
  isStandalone: boolean;
  promptInstall: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
}

function getStandalone(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false;
  }
  return window.matchMedia('(display-mode: standalone)').matches;
}

export function useInstallPrompt(): InstallPromptState {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null);
  const [isStandalone, setIsStandalone] = useState<boolean>(getStandalone);

  useEffect(() => {
    function handleBeforeInstallPrompt(event: Event) {
      event.preventDefault();
      setDeferredPrompt(event as BeforeInstallPromptEvent);
    }
    function handleAppInstalled() {
      setDeferredPrompt(null);
      setIsStandalone(true);
    }
    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
    window.addEventListener('appinstalled', handleAppInstalled);
    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
      window.removeEventListener('appinstalled', handleAppInstalled);
    };
  }, []);

  const promptInstall = useCallback(async (): Promise<'accepted' | 'dismissed' | 'unavailable'> => {
    if (!deferredPrompt) return 'unavailable';
    await deferredPrompt.prompt();
    const choice = await deferredPrompt.userChoice;
    setDeferredPrompt(null);
    return choice.outcome;
  }, [deferredPrompt]);

  return {
    canInstall: deferredPrompt !== null,
    isStandalone,
    promptInstall,
  };
}
