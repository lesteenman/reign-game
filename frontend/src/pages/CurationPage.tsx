import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { PageShell } from '@shared/components/PageShell';
import { GhostButton } from '@shared/components/Button';
import { PuzzleSelector } from '@components/landing/PuzzleSelector';
import type { PuzzleSelection } from '@components/landing/PuzzleSelector';
import { fetchEnabledModes } from '@services/landingService';
import type { ModeEntry } from '@shared/types/modes';

/**
 * Curation route — admin-only, mounted under <ProtectedAdminRoute>.
 *
 * Mirrors the pre-Phase-7 LandingPage flow: shows one button per
 * enabled (size, mode) pool, plus a Settings link to /admin for pool
 * configuration (threshold, enabled flag, replenish controls). When a
 * pool is selected, navigates to /play with the size + mode the
 * GamePage uses to fetch the next puzzle.
 *
 * Verdict surface + explicit Skip button live on GamePage and gate
 * themselves on the admin role — no curation-specific query param is
 * needed; admins are the only callers of /curation by construction.
 */
export function CurationPage() {
  const navigate = useNavigate();
  const [modes, setModes] = useState<ModeEntry[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    void fetchEnabledModes()
      .then((list) => {
        if (!cancelled) setModes(list);
      })
      .catch((err) => {
        // Same defensive stance as the old LandingPage: a network
        // failure falls through to the empty-state UI rather than
        // hiding the failure behind a generic loader.
        console.warn('CurationPage: fetchEnabledModes failed', err);
        if (!cancelled) setModes([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleSelect = useCallback(
    (selection: PuzzleSelection) => {
      const params = new URLSearchParams();
      params.set('flow', 'curation');
      params.set('size', String(selection.size));
      params.set('mode', selection.mode);
      navigate(`/play?${params.toString()}`);
    },
    [navigate],
  );

  const handleSettings = useCallback(() => {
    navigate('/admin');
  }, [navigate]);

  return (
    <PageShell>
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '24px',
          width: '100%',
          maxWidth: '600px',
          alignItems: 'center',
        }}
      >
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            width: '100%',
          }}
        >
          <h2
            style={{
              fontFamily: '"Nunito Sans", system-ui, sans-serif',
              fontWeight: 800,
              fontSize: '1.25rem',
              margin: 0,
            }}
          >
            Curation
          </h2>
          <GhostButton
            onClick={handleSettings}
            data-testid="curation-settings-button"
            aria-label="Pool settings (admin)"
          >
            Settings
          </GhostButton>
        </div>
        <PuzzleSelector modes={modes ?? []} onSelect={handleSelect} />
      </div>
    </PageShell>
  );
}
