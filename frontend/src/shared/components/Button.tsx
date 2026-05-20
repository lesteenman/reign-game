import { Children, type ReactNode, type MouseEventHandler } from 'react';
import { Link } from 'react-router-dom';
import { styled, Text, View } from 'tamagui';

// Tamagui v2-RC ships a known typing regression in downstream consumer
// projects: when `defaultConfig` from `@tamagui/config/v4` sets
// `settings.onlyAllowShorthands: true`, `Partial<StackStyle>` resolves
// the value type for every style key to a never-collapsing intersection
// of `WithThemeValues<...> & WithShorthands<...> & ...`, and TypeScript
// reports each primitive literal as "no properties in common with
// type ...". The runtime works correctly — Tamagui's own
// `@tamagui/button` uses identical longhand props (`borderWidth: 1`,
// `backgroundColor: '$background'`) and ships without issue because
// its pre-built `.d.ts` was generated against a different config.
// Until upstream fixes the `Partial`-over-intersection inference, we
// cast `styled` to `any` so type errors don't block development. The
// styled component still gets correct prop types downstream — the
// escape is local to the config object.

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

/**
 * Reign's chrome button (BRAND_GUIDELINES §5.4 "tactile ink shadow").
 *
 * Three variants — `primary` / `secondary` / `ghost` — at two sizes
 * (`large` default, `compact` for header / inline use). The press
 * effect (translate 1px + shadow shrink from 3→2) plays on hover AND
 * on press; both transitions hold the `quickerLessBouncy` animation
 * key (100ms ease-out from `@tamagui/config/v4`'s defaults — see
 * `tamagui.config.ts` for the canonical mapping).
 *
 * Disabled handling is built-in: passing `disabled` forwards to the
 * native `<button>` AND tells Tamagui to suppress `hoverStyle` /
 * `pressStyle` for that render, so the visual press effect doesn't
 * fire on a disabled CTA. The `disabledStyle` (opacity + cursor) is
 * applied automatically.
 *
 * The `Ghost` variant deliberately has no shadow + no translate —
 * BRAND_GUIDELINES specifies a color-shift hover effect for Ghost
 * (text-ink + bg-muted), preserved here via per-variant `hoverStyle`.
 *
 * **Implementation note.** Built on `styled(View, { render: <button/> })`
 * rather than the `styled.button({...})` factory because the v2-RC
 * factory typing collides with React's `type="button" | "submit"` HTML
 * attribute (Tamagui's own `type` style prop wins, breaking longhand
 * style usage too). `styled(View, ...)` is also what `@tamagui/button`
 * uses internally, so we stay aligned with upstream patterns.
 */
const ReignButton = styledAny(View, {
  name: 'ReignButton',
  // `render` controls the rendered DOM element + its initial props.
  // Default is `<button type="button">`; the `CompactSecondaryLink`
  // wrapper overrides this with a react-router `<Link />`. Tamagui's
  // role + tabIndex inference picks up `<button>` / `<a>` semantics
  // from the rendered element automatically (no explicit `role: 'button'`
  // here — that would force role="button" onto the Link's <a> too,
  // breaking screen-reader semantics for navigation links).
  render: <button type="button" />,

  // Base shape
  borderWidth: 2,
  borderColor: '$ink',
  borderRadius: 10,
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: '700',
  cursor: 'pointer',
  animation: 'quickerLessBouncy',
  // Hard ink-shadow style: radius 0 = sharp edge (no gaussian blur).
  // Per-variant `shadowColor` + `shadowOffset` carry the visual depth.
  shadowRadius: 0,
  shadowOpacity: 1,

  disabledStyle: {
    opacity: 0.4,
    cursor: 'not-allowed',
  },

  variants: {
    variant: {
      primary: {
        backgroundColor: '$accent',
        color: '$onAccent',
        shadowColor: '$accentShadow',
        shadowOffset: { width: 0, height: 3 },
        // hoverStyle/pressStyle must re-declare shadowColor alongside the
        // new shadowOffset; Tamagui emits the composite box-shadow only
        // when both shadow props are present in the pseudo-state block,
        // otherwise the hover-state shadow falls back to the base 3px.
        hoverStyle: {
          y: 1,
          shadowColor: '$accentShadow',
          shadowOffset: { width: 0, height: 2 },
        },
        pressStyle: {
          y: 1,
          shadowColor: '$accentShadow',
          shadowOffset: { width: 0, height: 2 },
        },
      },
      secondary: {
        backgroundColor: '$surface',
        color: '$ink',
        shadowColor: '$ink',
        shadowOffset: { width: 0, height: 3 },
        hoverStyle: {
          y: 1,
          shadowColor: '$ink',
          shadowOffset: { width: 0, height: 2 },
        },
        pressStyle: {
          y: 1,
          shadowColor: '$ink',
          shadowOffset: { width: 0, height: 2 },
        },
      },
      ghost: {
        backgroundColor: 'transparent',
        color: '$muted',
        borderColor: 'transparent',
        shadowColor: 'transparent',
        shadowOffset: { width: 0, height: 0 },
        // BRAND_GUIDELINES Ghost-hover: `text-ink, bg-muted-bg` — color
        // shift + a soft muted-background tint, no translate. The
        // `$mutedBg` token was added alongside #208 (mirrored in
        // `src/index.css` and `tamagui.config.ts`); it sits between
        // `$background` and `$border` so the affordance reads as a
        // subtle surface darkening rather than a hard outlined block.
        hoverStyle: {
          color: '$ink',
          backgroundColor: '$mutedBg',
        },
      },
    },
    size: {
      large: {
        paddingHorizontal: 32,
        paddingVertical: 12,
        fontSize: 16,
      },
      compact: {
        paddingHorizontal: 16,
        paddingVertical: 8,
        fontSize: 14,
        // 44×44 minimum is the Apple HIG / WCAG 2.5.5 tap-target floor.
        // The original `compactSecondaryButtonStyle` in `buttonStyles.ts`
        // enforced this; preserve it here so header SignInButton and
        // similar compact CTAs stay tappable on mobile.
        minHeight: 44,
        minWidth: 44,
      },
    },
  },

  defaultVariants: {
    variant: 'primary',
    size: 'large',
  },
});

interface ButtonProps {
  children: ReactNode;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  disabled?: boolean;
  'aria-label'?: string;
  'data-testid'?: string;
}

// Tamagui's `disabled` prop sets `aria-disabled` + applies `disabledStyle`
// but does NOT forward to the underlying `<button>`'s native HTML
// `disabled` attribute (cross-platform design: native `<button disabled>`
// doesn't exist in React Native). On web we DO want the native attribute
// — it prevents click dispatch entirely, while `aria-disabled` only
// announces state to assistive tech. Override `render` per-instance so
// the rendered `<button>` receives both treatments.
function renderEl(disabled: boolean | undefined) {
  return <button type="button" disabled={disabled} />;
}

/**
 * Tamagui's `<Stack>`-derived styled components refuse raw text children
 * (logs "A text node cannot be a child of a <ReignButton>"). Auto-wrap
 * strings/numbers in `<Text>` so consumers can keep writing
 * `<PrimaryButton>Sign in</PrimaryButton>` without ceremony. ReactNode
 * children that are already elements pass through unchanged.
 */
function wrapTextChildren(children: ReactNode): ReactNode {
  return Children.map(children, (child) => {
    if (typeof child === 'string' || typeof child === 'number') {
      return <Text>{child}</Text>;
    }
    return child;
  });
}

/** Accent-colored primary action button. */
export function PrimaryButton({ children, disabled, ...rest }: ButtonProps) {
  return (
    <ReignButton
      variant="primary"
      size="large"
      disabled={disabled}
      render={renderEl(disabled)}
      {...rest}
    >
      {wrapTextChildren(children)}
    </ReignButton>
  );
}

/** Surface-colored secondary action button. */
export function SecondaryButton({ children, disabled, ...rest }: ButtonProps) {
  return (
    <ReignButton
      variant="secondary"
      size="large"
      disabled={disabled}
      render={renderEl(disabled)}
      {...rest}
    >
      {wrapTextChildren(children)}
    </ReignButton>
  );
}

/**
 * Ghost / tertiary button. Transparent background, muted text, no
 * border or shadow. For deliberate-but-rare actions (Skip, Cancel,
 * dismiss-style controls) where the visual weight of Primary or
 * Secondary would compete with the main CTA.
 *
 * Per BRAND_GUIDELINES §5.4.
 */
export function GhostButton({ children, disabled, ...rest }: ButtonProps) {
  return (
    <ReignButton
      variant="ghost"
      size="large"
      disabled={disabled}
      render={renderEl(disabled)}
      {...rest}
    >
      {wrapTextChildren(children)}
    </ReignButton>
  );
}

/**
 * Compact secondary button used by header controls (sign-in) and
 * forbidden-state landing cards. Smaller padding + font, but the same
 * tactile-ink-shadow press feel as `SecondaryButton`.
 *
 * Replaces the pre-#208 `compactSecondaryButtonStyle` const +
 * `pressIn`/`pressOut` handlers from `buttonStyles.ts` / `press.ts`.
 */
export function CompactSecondaryButton({ children, disabled, ...rest }: ButtonProps) {
  return (
    <ReignButton
      variant="secondary"
      size="compact"
      disabled={disabled}
      render={renderEl(disabled)}
      {...rest}
    >
      {wrapTextChildren(children)}
    </ReignButton>
  );
}

interface CompactSecondaryLinkProps {
  children: ReactNode;
  to: string;
  'aria-label'?: string;
  'data-testid'?: string;
}

/**
 * Router-aware twin of `CompactSecondaryButton` — same visual style
 * (size: compact, variant: secondary, tactile-ink-shadow press effect),
 * but renders as a react-router `<Link>` so client-side navigation
 * still works. Used in places like AdminLandingPage's "Home" CTA where
 * the affordance reads as a button but the action is a route change.
 *
 * Implementation uses Tamagui's `render` override to swap the underlying
 * element from `<button>` to `<Link>` while keeping every style the
 * styled component declared. The `style={textDecoration: 'none'}` keeps
 * the `<a>`-default underline off — anchor-styling defaults would
 * otherwise leak through.
 */
export function CompactSecondaryLink({ children, to, ...rest }: CompactSecondaryLinkProps) {
  return (
    <ReignButton
      variant="secondary"
      size="compact"
      render={<Link to={to} style={{ textDecoration: 'none' }} />}
      {...rest}
    >
      {wrapTextChildren(children)}
    </ReignButton>
  );
}
