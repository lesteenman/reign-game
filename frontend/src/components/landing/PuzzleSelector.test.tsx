import { render, screen, fireEvent, cleanup } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { PuzzleSelector } from './PuzzleSelector';
import type { ModeEntry } from '@shared/types/modes';

const DEFAULT_MODES: ModeEntry[] = [
  { size: 5, mode: 'standard' },
  { size: 7, mode: 'standard' },
  { size: 9, mode: 'standard' },
  { size: 9, mode: 'double' },
];

afterEach(() => {
  cleanup();
});

function renderSelector(
  modes: ModeEntry[] = DEFAULT_MODES,
  onSelect = vi.fn(),
) {
  return {
    ...render(<PuzzleSelector modes={modes} onSelect={onSelect} />),
    onSelect,
  };
}

describe('PuzzleSelector', () => {
  describe('preset buttons', () => {
    it('renders one button per supplied mode', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      for (let i = 0; i < DEFAULT_MODES.length; i++) {
        expect(screen.getByTestId(`preset-${i}`)).toBeInTheDocument();
      }
    });

    it('displays correct labels on preset buttons', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      expect(screen.getByTestId('preset-0')).toHaveTextContent('5\u00D75 Standard');
      expect(screen.getByTestId('preset-1')).toHaveTextContent('7\u00D77 Standard');
      expect(screen.getByTestId('preset-2')).toHaveTextContent('9\u00D79 Standard');
      expect(screen.getByTestId('preset-3')).toHaveTextContent('9\u00D79 Double Queens');
    });

    it('first preset is selected by default', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      expect(screen.getByTestId('preset-0')).toHaveAttribute('aria-pressed', 'true');
      expect(screen.getByTestId('preset-1')).toHaveAttribute('aria-pressed', 'false');
    });

    it('clicking a preset updates the selected state', () => {
      // Arrange
      renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('preset-2'));

      // Assert
      expect(screen.getByTestId('preset-0')).toHaveAttribute('aria-pressed', 'false');
      expect(screen.getByTestId('preset-2')).toHaveAttribute('aria-pressed', 'true');
    });

    it('Play with default preset emits the first mode', () => {
      // Arrange
      const { onSelect } = renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({ size: 5, mode: 'standard' });
    });

    it('Play with 9x9 Double Queens preset sends size=9 mode=double', () => {
      // Arrange
      const { onSelect } = renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('preset-3'));
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({ size: 9, mode: 'double' });
    });
  });

  describe('empty state', () => {
    it('renders a friendly message when no modes are available', () => {
      // Arrange & Act
      renderSelector([]);

      // Assert
      expect(screen.getByTestId('puzzle-selector-empty')).toBeInTheDocument();
      expect(screen.queryByTestId('puzzle-selector')).not.toBeInTheDocument();
      expect(screen.queryByTestId('play-button')).not.toBeInTheDocument();
    });
  });

  describe('no advanced section', () => {
    it('does not render an advanced toggle', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      expect(screen.queryByTestId('advanced-toggle')).not.toBeInTheDocument();
    });

    it('does not render an advanced section', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      expect(screen.queryByTestId('advanced-section')).not.toBeInTheDocument();
    });

    it('onSelect only receives size and mode, no pipeline/solver/regions', () => {
      // Arrange
      const { onSelect } = renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('preset-1'));
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      const callArg = onSelect.mock.calls[0]![0] as Record<string, unknown>;
      expect(callArg).toEqual({ size: 7, mode: 'standard' });
      expect(callArg).not.toHaveProperty('pipeline');
      expect(callArg).not.toHaveProperty('solver');
      expect(callArg).not.toHaveProperty('regions');
      expect(callArg).not.toHaveProperty('regionVariance');
    });
  });
});
