import { StrictMode, type ReactElement, type ReactNode } from 'react';
import {
  render as tlRender,
  renderHook as tlRenderHook,
  type RenderOptions,
  type RenderHookOptions,
} from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TamaguiProvider } from 'tamagui';
import { DarkModeProvider } from '@theme/useDarkMode';
// eslint-disable-next-line no-restricted-imports
import tamaguiConfig from '../../tamagui.config';

/**
 * Re-exports @testing-library/react with `render` and `renderHook` wrapped
 * in React.StrictMode, a fresh QueryClientProvider per test, the app's
 * DarkModeProvider, and Tamagui's provider so any Tamagui-styled
 * component (or hook calling `useDarkMode()`) runs against the same
 * context the app uses.
 *
 * StrictMode double-invokes state updaters and effects to surface impure-
 * updater bugs (e.g. nested setState inside a reducer updater) at unit-
 * test time instead of waiting for Playwright.
 *
 * `<TamaguiProvider>` is required even for non-Tamagui components,
 * because once Tamagui-styled siblings exist anywhere in the rendered
 * tree (e.g. `Button.tsx` after #208), Tamagui's `createComponent`
 * looks up the active config via `getConfig()` and throws "Missing
 * tamagui config" if it isn't mounted. Wrapping it here means every
 * test gets the config for free.
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
      <QueryClientProvider client={makeQueryClient()}>
        <DarkModeProvider>
          <TamaguiProvider config={tamaguiConfig} defaultTheme="light">{children}</TamaguiProvider>
        </DarkModeProvider>
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
              <TamaguiProvider config={tamaguiConfig} defaultTheme="light">
                <UserWrapper>{children}</UserWrapper>
              </TamaguiProvider>
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
              <TamaguiProvider config={tamaguiConfig} defaultTheme="light">
                <UserWrapper>{children}</UserWrapper>
              </TamaguiProvider>
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
