import type { LucideIcon, LucideProps } from 'lucide-react';

export interface IconProps extends Omit<LucideProps, 'ref'> {
  as: LucideIcon;
}

/**
 * Brand-default Lucide wrapper. Pins size 20 and strokeWidth 1.5 per
 * BRAND_GUIDELINES.md (Tab bar icons section). `aria-hidden` defaults
 * to true because every current adoption site is an icon inside a
 * `<button>` whose `aria-label` carries the accessible name.
 */
export function Icon({
  as: Component,
  size = 20,
  strokeWidth = 1.5,
  'aria-hidden': ariaHidden = true,
  ...rest
}: IconProps) {
  return (
    <Component
      size={size}
      strokeWidth={strokeWidth}
      aria-hidden={ariaHidden}
      {...rest}
    />
  );
}
