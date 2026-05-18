import { describe, it, expect, afterEach, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';
import { render, screen, cleanup } from '../../test-utils';
import { InstallAppTile } from './InstallAppTile';
import * as useInstallPromptModule from '../../shared/hooks/useInstallPrompt';
import type { InstallPromptState } from '../../shared/hooks/useInstallPrompt';

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

describe('InstallAppTile', () => {
  it('renders nothing when canInstall=false', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: false, isStandalone: false });

    // Act
    render(<InstallAppTile />);

    // Assert
    expect(screen.queryByTestId('tile-install')).not.toBeInTheDocument();
  });

  it('renders nothing when isStandalone=true (already installed)', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: true, isStandalone: true });

    // Act
    render(<InstallAppTile />);

    // Assert
    expect(screen.queryByTestId('tile-install')).not.toBeInTheDocument();
  });

  it('renders the tile when canInstall=true and not standalone', () => {
    // Arrange
    mockUseInstallPrompt({ canInstall: true, isStandalone: false });

    // Act
    render(<InstallAppTile />);

    // Assert
    const tile = screen.getByTestId('tile-install');
    expect(tile).toBeInTheDocument();
    expect(tile).toHaveTextContent(/install/i);
  });

  it('calls promptInstall when clicked', () => {
    // Arrange
    const state = mockUseInstallPrompt({ canInstall: true, isStandalone: false });

    // Act
    render(<InstallAppTile />);
    fireEvent.click(screen.getByTestId('tile-install'));

    // Assert
    expect(state.promptInstall).toHaveBeenCalledOnce();
  });
});
