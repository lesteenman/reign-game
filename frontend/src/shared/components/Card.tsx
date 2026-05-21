import type { ReactNode } from 'react';
import { styled, View } from 'tamagui';

/**
 * BRAND_GUIDELINES §5.5 card — surface background, 2px ink border,
 * 10px radius, 3px ink offset shadow, centered flex column with 16px
 * gap. Two sizes: `compact` (24px padding, used by DailyFlow's
 * error / submit-error cards) and `large` (32px vertical / 24px
 * horizontal, used by AdminLandingPage's landing cards). Both cap at
 * 480px wide — beyond that, focused-CTA cards stop reading as cards
 * and start reading as sparse dialogs.
 *
 * **Implementation note.** Same `styled(View, ...)` + `styledAny`
 * cast pattern as Button.tsx (#208) / IconButton.tsx (#176 PageShell
 * slice) — see `frontend/CLAUDE.md` for the Tamagui-v2-RC
 * `Partial<StackStyle>` inference workaround.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const CardBase = styledAny(View, {
  name: 'Card',
  backgroundColor: '$surface',
  borderWidth: 2,
  borderColor: '$ink',
  borderRadius: 10,
  // Hard ink-shadow (radius 0 = sharp edge, no gaussian blur). Same
  // primitive Button.tsx uses for its tactile press shadow; here it
  // sits at 3px to give the card visible depth on light backgrounds.
  shadowColor: '$ink',
  shadowOffset: { width: 0, height: 3 },
  shadowRadius: 0,
  shadowOpacity: 1,
  alignItems: 'center',
  gap: 16,
  width: '100%',
  maxWidth: 480,
  // Text alignment is the responsibility of text children (apply via
  // `<Text textAlign="center">` or a styled wrapper). Setting it here
  // would leak as a DOM `textalign` attribute — Tamagui doesn't
  // recognize it as a style key on View-derived components in v2-RC,
  // so the prop passes through to the rendered `<div>` and triggers
  // React's "does not recognize prop on DOM element" warning. Same
  // class of leak as `lineHeight` on IconButton in #216.

  variants: {
    size: {
      compact: {
        padding: 24,
      },
      large: {
        paddingHorizontal: 24,
        paddingVertical: 32,
      },
    },
  },

  defaultVariants: {
    size: 'compact',
  },
});

interface CardProps {
  children: ReactNode;
  size?: 'compact' | 'large';
  'data-testid'?: string;
}

/**
 * Surface-style card primitive. Renders a `<div>` (via Tamagui's
 * default View tag) — semantic-content children (`<h2>`, `<p>`,
 * buttons) are the consumer's responsibility.
 */
export function Card({ children, size = 'compact', ...rest }: CardProps) {
  return (
    <CardBase size={size} {...rest}>
      {children}
    </CardBase>
  );
}
