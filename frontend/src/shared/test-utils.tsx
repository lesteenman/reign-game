import { StrictMode, type ReactElement, type ReactNode } from 'react';
import {
  render as tlRender,
  renderHook as tlRenderHook,
  type RenderOptions,
  type RenderHookOptions,
} from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

/**
 * Re-exports @testing-library/react with `render` and `renderHook` wrapped
 * in React.StrictMode and a fresh QueryClientProvider per test. StrictMode
 * double-invokes state updaters and effects to surface impure-updater bugs
 * (e.g. nested setState inside a reducer updater) at unit-test time instead
 * of waiting for Playwright.
 *
 * Tests that need to opt out can import `render` / `renderHook` directly
 * from '@testing-library/react', but the default should be this module.
 */

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function StrictModeWrapper({ children }: { children: ReactNode }) {
  return (
    <StrictMode>
      <QueryClientProvider client={makeQueryClient()}>{children}</QueryClientProvider>
    </StrictMode>
  );
}

export function render(ui: ReactElement, options?: RenderOptions) {
  const UserWrapper = options?.wrapper;
  const Wrapper = UserWrapper
    ? ({ children }: { children: ReactNode }) => (
        <StrictMode>
          <QueryClientProvider client={makeQueryClient()}>
            <UserWrapper>{children}</UserWrapper>
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
            <UserWrapper>{children}</UserWrapper>
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
