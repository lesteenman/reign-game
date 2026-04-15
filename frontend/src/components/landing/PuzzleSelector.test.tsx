import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { PuzzleSelector, PRESETS, VARIANCE_STOPS } from './PuzzleSelector';

afterEach(() => {
  cleanup();
});

function renderSelector(onSelect = vi.fn()) {
  return { ...render(<PuzzleSelector onSelect={onSelect} />), onSelect };
}

describe('PuzzleSelector', () => {
  describe('preset buttons', () => {
    it('renders all 4 preset buttons', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      for (let i = 0; i < PRESETS.length; i++) {
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

    it('Play with default preset sends size=5 mode=standard', () => {
      // Arrange
      const { onSelect } = renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({ size: 5, mode: 'standard' });
    });

    it('Play with 9x9 Double Queens preset sends correct options', () => {
      // Arrange
      const { onSelect } = renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('preset-3'));
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({ size: 9, mode: 'double' });
    });
  });

  describe('advanced toggle', () => {
    it('advanced section is hidden by default', () => {
      // Arrange & Act
      renderSelector();

      // Assert
      expect(screen.queryByTestId('advanced-section')).not.toBeInTheDocument();
    });

    it('clicking Advanced toggles the advanced section open', () => {
      // Arrange
      renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert
      expect(screen.getByTestId('advanced-section')).toBeInTheDocument();
    });

    it('clicking Advanced again hides the advanced section', () => {
      // Arrange
      renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('advanced-toggle'));
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert
      expect(screen.queryByTestId('advanced-section')).not.toBeInTheDocument();
    });

    it('toggle has correct aria-expanded state', () => {
      // Arrange
      renderSelector();
      const toggle = screen.getByTestId('advanced-toggle');

      // Assert (closed)
      expect(toggle).toHaveAttribute('aria-expanded', 'false');

      // Act
      fireEvent.click(toggle);

      // Assert (open)
      expect(toggle).toHaveAttribute('aria-expanded', 'true');
    });
  });

  describe('advanced options', () => {
    it('renders all advanced controls when expanded', () => {
      // Arrange
      renderSelector();

      // Act
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert
      expect(screen.getByTestId('adv-size')).toBeInTheDocument();
      expect(screen.getByTestId('adv-mode')).toBeInTheDocument();
      expect(screen.getByTestId('adv-pipeline')).toBeInTheDocument();
      expect(screen.getByTestId('adv-solver')).toBeInTheDocument();
      expect(screen.getByTestId('adv-regions')).toBeInTheDocument();
      expect(screen.getByTestId('adv-variance')).toBeInTheDocument();
    });

    it('changing advanced options overrides preset selection', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '12' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith(
        expect.objectContaining({ size: 12 }),
      );
    });

    it('sends all advanced params when Play is clicked', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '7' } });
      fireEvent.change(screen.getByTestId('adv-mode'), { target: { value: 'double' } });
      fireEvent.change(screen.getByTestId('adv-pipeline'), { target: { value: 'iterative' } });
      fireEvent.change(screen.getByTestId('adv-solver'), { target: { value: 'propagation' } });
      fireEvent.change(screen.getByTestId('adv-regions'), { target: { value: 'wfc' } });
      fireEvent.change(screen.getByTestId('adv-variance'), { target: { value: '4' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({
        size: 7,
        mode: 'double',
        pipeline: 'iterative',
        solver: 'propagation',
        regions: 'wfc',
        regionVariance: 1.0,
      });
    });

    it('clicking a preset after using advanced reverts to preset mode', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '12' } });

      // Act
      fireEvent.click(screen.getByTestId('preset-1'));
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith({ size: 7, mode: 'standard' });
    });
  });

  describe('variance slider', () => {
    it('snaps to 5 discrete values', () => {
      // Arrange
      renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));
      const slider = screen.getByTestId('adv-variance') as HTMLInputElement;

      // Assert
      expect(slider.min).toBe('0');
      expect(slider.max).toBe('4');
      expect(slider.step).toBe('1');
    });

    it('shows all 5 variance labels', () => {
      // Arrange
      renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert
      for (const stop of VARIANCE_STOPS) {
        expect(screen.getByText(stop.label)).toBeInTheDocument();
      }
    });

    it('indicates current value in label', () => {
      // Arrange
      renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert — default is Moderate (index 2)
      expect(screen.getByText('Region Variance: Moderate')).toBeInTheDocument();
    });

    it('updates label when slider value changes', () => {
      // Arrange
      renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-variance'), { target: { value: '4' } });

      // Assert
      expect(screen.getByText('Region Variance: Wild')).toBeInTheDocument();
    });

    it('maps slider positions to correct numeric values', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act — set to Slight (index 1, value 0.25)
      fireEvent.change(screen.getByTestId('adv-variance'), { target: { value: '1' } });
      // Need to trigger advanced mode by also changing something
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '5' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith(
        expect.objectContaining({ regionVariance: 0.25 }),
      );
    });
  });

  describe('grid size validation', () => {
    it('shows error when grid size is below 3', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '2' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(screen.getByTestId('adv-size-error')).toHaveTextContent(
        'Grid size must be between 3 and 15',
      );
      expect(onSelect).not.toHaveBeenCalled();
    });

    it('shows error when grid size is above 15', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '20' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(screen.getByTestId('adv-size-error')).toHaveTextContent(
        'Grid size must be between 3 and 15',
      );
      expect(onSelect).not.toHaveBeenCalled();
    });

    it('shows error when grid size is empty', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(screen.getByTestId('adv-size-error')).toHaveTextContent(
        'Grid size must be between 3 and 15',
      );
      expect(onSelect).not.toHaveBeenCalled();
    });

    it('clears error when value changes to valid range', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '2' } });
      fireEvent.click(screen.getByTestId('play-button'));
      expect(screen.getByTestId('adv-size-error')).toBeInTheDocument();

      // Act
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '10' } });

      // Assert
      expect(screen.queryByTestId('adv-size-error')).not.toBeInTheDocument();
    });

    it('allows typing 10 by clearing field first', () => {
      // Arrange
      const { onSelect } = renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Act — simulate mobile: clear field, then type "10"
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '' } });
      fireEvent.change(screen.getByTestId('adv-size'), { target: { value: '10' } });
      fireEvent.click(screen.getByTestId('play-button'));

      // Assert
      expect(onSelect).toHaveBeenCalledWith(
        expect.objectContaining({ size: 10 }),
      );
    });

    it('does not have min or max HTML attributes on grid size input', () => {
      // Arrange
      renderSelector();
      fireEvent.click(screen.getByTestId('advanced-toggle'));

      // Assert
      const input = screen.getByTestId('adv-size');
      expect(input).not.toHaveAttribute('min');
      expect(input).not.toHaveAttribute('max');
    });
  });
});
