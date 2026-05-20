import { describe, it, expect, afterEach, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';
import { render, screen, cleanup } from '@shared/test-utils';
import { InstallButton } from './InstallButton';
import * as useInstallPromptModule from '@shared/hooks/useInstallPrompt';
import type { InstallPromptState } from '@shared/hooks/useInstallPrompt';

function mockUseInstallPrompt(overrides: Partial<InstallPromptState> = {}): InstallPromptState {
  const state: InstallPromptState = {
    canInstall: false,
    isStandalone: false,
    promptInstall: vi.fn().mockResolvedValue('accepted'),
    ...overrides,
  };
  vi.spyOn(useInstallPromptModule, 'useInstallPrompt').mockReturnValue(state);
  return state;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('InstallButton', () => {
  it('renders nothing when canInstall=false', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: false, isStandalone: false });

    // Act
    render(<InstallButton />);

    // Assert
    expect(screen.queryByTestId('install-button')).not.toBeInTheDocument();
  });

  it('renders nothing when isStandalone=true (already installed)', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: true, isStandalone: true });

    // Act
    render(<InstallButton />);

    // Assert
    expect(screen.queryByTestId('install-button')).not.toBeInTheDocument();
  });

  it('renders the button when canInstall=true and not standalone', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: true, isStandalone: false });

    // Act
    render(<InstallButton />);

    // Assert
    const button = screen.getByTestId('install-button');
    expect(button).toBeInTheDocument();
    expect(button).toHaveTextContent(/install/i);
    expect(button).toHaveAttribute('aria-label', 'Install Reign as an app');
  });

  it('calls promptInstall when clicked', () => {
    // Arrange
    const state = mockUseInstallPrompt({ canInstall: true, isStandalone: false });

    // Act
    render(<InstallButton />);
    fireEvent.click(screen.getByTestId('install-button'));

    // Assert
    expect(state.promptInstall).toHaveBeenCalledOnce();
  });
});
