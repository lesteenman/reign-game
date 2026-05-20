import { StrictMode, type ReactElement, type ReactNode } from 'react';
import {
  render as tlRender,
  renderHook as tlRenderHook,
  type RenderOptions,
  type RenderHookOptions,
} from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DarkModeProvider } from '@theme/useDarkMode';

/**
 * Re-exports @testing-library/react with `render` and `renderHook` wrapped
 * in React.StrictMode, a fresh QueryClientProvider per test, and the app's
 * DarkModeProvider (so any component / hook consuming `useDarkMode()`
 * inside a test runs against the same context the app uses).
 *
 * StrictMode double-invokes state updaters and effects to surface impure-
 * updater bugs (e.g. nested setState inside a reducer updater) at unit-
 * test time instead of waiting for Playwright.
 *
 * Tests that need to opt out can import `render` / `renderHook` directly
 * from '@testing-library/react', but the default should be this module.
 *
 * Note for the upcoming Button-migration slice (#208): once Tamagui-styled
 * components land, this helper should also wrap in `<TamaguiProvider>`
 * — flagged in #208's acceptance criteria.
 */

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function StrictModeWrapper({ children }: { children: ReactNode }) {
  return (
    <StrictMode>
      <QueryClientProvider client={makeQueryClient()}>
        <DarkModeProvider>{children}</DarkModeProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}

export function render(ui: ReactElement, options?: RenderOptions) {
  const UserWrapper = options?.wrapper;
  const Wrapper = UserWrapper
    ? ({ children }: { children: ReactNode }) => (
        <StrictMode>
          <QueryClientProvider client={makeQueryClient()}>
            <DarkModeProvider>
            <UserWrapper>{children}</UserWrapper>
          </DarkModeProvider>
          </QueryClientProvider>
        </StrictMode>
      )
    : StrictModeWrapper;
  return tlRender(ui, { ...options, wrapper: Wrapper });
}

export function renderHook<Result, Props>(
  callback: (initialProps: Props) => Result,
  options?: RenderHookOptions<Props>,
) {
  const UserWrapper = options?.wrapper;
  const Wrapper = UserWrapper
    ? ({ children }: { children: ReactNode }) => (
        <StrictMode>
          <QueryClientProvider client={makeQueryClient()}>
            <DarkModeProvider>
            <UserWrapper>{children}</UserWrapper>
          </DarkModeProvider>
          </QueryClientProvider>
        </StrictMode>
      )
    : StrictModeWrapper;
  return tlRenderHook(callback, { ...options, wrapper: Wrapper });
}

export {
  act,
  cleanup,
  fireEvent,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
