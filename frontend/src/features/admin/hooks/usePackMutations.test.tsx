import { renderHook, waitFor } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useCreatePack } from './useCreatePack';
import { useUpdatePack } from './useUpdatePack';
import { useDeletePack } from './useDeletePack';
import type { PackView } from '@shared/types/pack';

const mockCreatePack = vi.fn();
const mockUpdatePack = vi.fn();
const mockDeletePack = vi.fn();
vi.mock('@features/admin/services/adminPackService', () => ({
  createPack: (...args: unknown[]) => mockCreatePack(...args),
  updatePack: (...args: unknown[]) => mockUpdatePack(...args),
  deletePack: (...args: unknown[]) => mockDeletePack(...args),
}));

const MOCK_PACK: PackView = {
  slug: 'starter',
  name: 'Starter',
  size: 7,
  mode: 'standard',
  published: false,
  puzzleIds: ['p1'],
  createdAt: '2026-06-14T00:00:00Z',
  updatedAt: '2026-06-14T00:00:00Z',
};

beforeEach(() => {
  mockCreatePack.mockReset();
  mockUpdatePack.mockReset();
  mockDeletePack.mockReset();
});

describe('useCreatePack', () => {
  it('calls createPack and resolves on success', async () => {
    // Arrange
    mockCreatePack.mockResolvedValue(MOCK_PACK);

    // Act
    const { result } = renderHook(() => useCreatePack());
    result.current.mutate({
      slug: 'starter',
      name: 'Starter',
      size: 7,
      mode: 'standard',
      puzzleIds: ['p1'],
      published: false,
    });

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockCreatePack).toHaveBeenCalledOnce();
  });

  it('surfaces a service rejection as the mutation error', async () => {
    // Arrange
    mockCreatePack.mockRejectedValue(new Error('pack already exists: starter'));

    // Act
    const { result } = renderHook(() => useCreatePack());
    result.current.mutate({
      slug: 'starter',
      name: 'Starter',
      size: 7,
      mode: 'standard',
      puzzleIds: [],
      published: false,
    });

    // Assert
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('pack already exists: starter');
  });
});

describe('useUpdatePack', () => {
  it('calls updatePack with slug + data', async () => {
    // Arrange
    mockUpdatePack.mockResolvedValue(MOCK_PACK);

    // Act
    const { result } = renderHook(() => useUpdatePack());
    result.current.mutate({
      slug: 'starter',
      data: { name: 'x', puzzleIds: ['p1'], published: true },
    });

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockUpdatePack).toHaveBeenCalledWith('starter', {
      name: 'x',
      puzzleIds: ['p1'],
      published: true,
    });
  });
});

describe('useDeletePack', () => {
  it('calls deletePack with the slug', async () => {
    // Arrange
    mockDeletePack.mockResolvedValue(undefined);

    // Act
    const { result } = renderHook(() => useDeletePack());
    result.current.mutate('starter');

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockDeletePack).toHaveBeenCalledWith('starter');
  });
});
