---
name: Lingnan Mahjong Social Scorekeeper
colors:
  surface: '#f8f9ff'
  surface-dim: '#cbdbf5'
  surface-bright: '#f8f9ff'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#eff4ff'
  surface-container: '#e5eeff'
  surface-container-high: '#dce9ff'
  surface-container-highest: '#d3e4fe'
  on-surface: '#0b1c30'
  on-surface-variant: '#3f4943'
  inverse-surface: '#213145'
  inverse-on-surface: '#eaf1ff'
  outline: '#6f7a72'
  outline-variant: '#bfc9c0'
  surface-tint: '#1b6b4a'
  primary: '#005235'
  on-primary: '#ffffff'
  primary-container: '#1b6b4a'
  on-primary-container: '#9ce9bf'
  inverse-primary: '#8ad6ae'
  secondary: '#904d00'
  on-secondary: '#ffffff'
  secondary-container: '#fe932c'
  on-secondary-container: '#663500'
  tertiary: '#8e1215'
  on-tertiary: '#ffffff'
  tertiary-container: '#b02d29'
  on-tertiary-container: '#ffcdc7'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#a6f3c9'
  primary-fixed-dim: '#8ad6ae'
  on-primary-fixed: '#002113'
  on-primary-fixed-variant: '#005235'
  secondary-fixed: '#ffdcc3'
  secondary-fixed-dim: '#ffb77d'
  on-secondary-fixed: '#2f1500'
  on-secondary-fixed-variant: '#6e3900'
  tertiary-fixed: '#ffdad6'
  tertiary-fixed-dim: '#ffb4ac'
  on-tertiary-fixed: '#410002'
  on-tertiary-fixed-variant: '#8e1214'
  background: '#f8f9ff'
  on-background: '#0b1c30'
  surface-variant: '#d3e4fe'
typography:
  display-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 36px
    fontWeight: '700'
    lineHeight: 44px
    letterSpacing: -0.02em
  display-lg-mobile:
    fontFamily: Plus Jakarta Sans
    fontSize: 28px
    fontWeight: '700'
    lineHeight: 36px
    letterSpacing: -0.02em
  headline-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: -0.015em
  headline-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
    letterSpacing: -0.01em
  headline-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 18px
    fontWeight: '600'
    lineHeight: 24px
  body-lg:
    fontFamily: Be Vietnam Pro
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-md:
    fontFamily: Be Vietnam Pro
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  body-sm:
    fontFamily: Be Vietnam Pro
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 18px
  label-lg:
    fontFamily: Plus Jakarta Sans
    fontSize: 14px
    fontWeight: '600'
    lineHeight: 20px
    letterSpacing: 0.01em
  label-md:
    fontFamily: Plus Jakarta Sans
    fontSize: 12px
    fontWeight: '500'
    lineHeight: 16px
    letterSpacing: 0.02em
  label-sm:
    fontFamily: Plus Jakarta Sans
    fontSize: 10px
    fontWeight: '500'
    lineHeight: 14px
    letterSpacing: 0.04em
  score-numeral:
    fontFamily: Plus Jakarta Sans
    fontSize: 22px
    fontWeight: '700'
    lineHeight: 26px
    letterSpacing: -0.01em
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  space-2xs: 0.25rem
  space-xs: 0.5rem
  space-sm: 0.75rem
  space-md: 1rem
  space-lg: 1.25rem
  space-xl: 1.5rem
  space-2xl: 2rem
  space-3xl: 2.5rem
  margin-screen: 1.25rem
  gutter-grid: 0.75rem
---

## Brand & Style

This design system establishes a refined, serene companion for casual parlor gaming and social gatherings. Translating the leisure of Lingnan tea-and-tile culture into a contemporary digital surface, the aesthetic trades loud casino tropes, skeuomorphic felt textures, and aggressive gamification badges for an airy, contemplative lifestyle utility.

Key characteristics:
- **Zen-Like Breathing Room:** Expansive negative space allows fast cognitive recovery during lively match play. Visual noise, aggressive modal prompts, and gratuitous celebratory confetti are omitted in favor of tranquil, purposeful layouts.
- **Modern Tea-Parlor Warmth:** Balanced jade accents evoke polished stone and green tea, set against porcelain and rice-paper neutrals for a grounded, tactile atmosphere.
- **Understated Precision:** Numeric adjustments, tallies, and wind directions are presented with typographic restraint. Positive and negative shifts guide players intuitively without jarring high-contrast alerts.

## Colors

The color palette centers on a disciplined jade green supported by gentle ceramics and muted status indicators:

- **Primary (`#1B6B4A`):** Deep polished jade. Used for focal actions, active table positions, wind indicators, and key match milestones.
- **Secondary (`#D97706`):** Warm burnished gold/amber. Reserved for subtle point wins, dealer badges, and minor achievements without flashing bright neons.
- **Tertiary (`#991B1B`):** Muted cinnabar red. Applied sparingly to point deficits and debt settles, keeping negative feedback factual rather than punitive.
- **Neutrals:** Crisp porcelain white (`#FFFFFF`) forms the elevated card base, nestled inside an airy, soft-mist canvas (`#F8F9FA`). Deep slate (`#1E293B`) anchors primary body text, while muted slate (`#64748B`) handles tabular metadata, dealer rotations, and secondary labels. Border lines rely on a faint hairline tone (`#F1F5F9`).

## Typography

The pairing of **Plus Jakarta Sans** for structure and **Be Vietnam Pro** for textual rhythm creates an uncluttered, modern atmosphere that remains easy to scan across quick rounds:

- **Headlines & Scores:** Plus Jakarta Sans provides geometric stability with gentle humanist apertures. Numeric values employ tabular figures (`tnum`) to ensure tally sheets remain balanced without column jitter.
- **Body & Captions:** Be Vietnam Pro delivers high legibility at compact mobile scales, offering comfortable reading during prolonged score-entry sessions.
- **Hierarchy Rules:** Never stack more than two typographic weights in a single row or card segment. Score tallies rely on distinct color shifts and weight contrast instead of oversized point sizes.

## Layout & Spacing

Designed for the standard portrait viewport of mobile mini-programs, layout rhythm prioritizes single-thumb efficiency and generous vertical intervals:

- **Grid Model:** A single-column flow with a maximum content boundary of `480px`, centered on larger viewports. Internal composite components (e.g., 4-player quadrant layouts) utilize an equal-width 2x2 or 4-column balanced grid with a fixed `0.75rem` (12px) gutter.
- **Screen Margins:** Fixed `1.25rem` (20px) horizontal safe boundaries provide calm separation from physical hardware bezels and WeChat overlay menus (capsule controls).
- **Vertical Breathing Cadence:** Section headers maintain a `2rem` top offset and a `0.75rem` bottom separation from interactive cards. Grouped list records sit flush with `0.5rem` row spacing, preserving scannable rhythm without dense packing.

## Elevation & Depth

Visual hierarchy uses flat, tactile surface stacking paired with diffused ambient light rather than harsh drop shadows:

- **Base Canvas:** `#F8F9FA` serves as the grounding plane, evoking matte porcelain paper.
- **Card Tier (Level 1):** `#FFFFFF` surfaces with an ultra-soft ambient shadow (`0 2px 8px -2px rgba(30, 41, 59, 0.04), 0 1px 2px -1px rgba(30, 41, 59, 0.02)`) and a delicate hairline boundary (`1px solid #F1F5F9`).
- **Interactive Floating Tier (Level 2):** Bottom entry sheets and active dealer selectors receive a light ambient lift (`0 8px 24px -4px rgba(27, 107, 74, 0.08), 0 2px 6px -1px rgba(30, 41, 59, 0.03)`).
- **Glass Accents:** Modal overlays and sticky top action banners feature high-translucency frosted glass (`background: rgba(248, 249, 250, 0.85); backdrop-filter: blur(12px)`), keeping underlying round tables contextualized while preventing stark dark cutoffs.

## Shapes

With a roundedness index of `2`, elements take on balanced curvature inspired by smoothed ceramic tiles:

- **Standard Elements:** Interactive controls, input rows, and tabular cells utilize `rounded-md` (`0.5rem` / 8px).
- **Containers & Match Cards:** Standalone cards, player seating blocks, and modular score panels use `rounded-lg` (`1rem` / 16px).
- **Dialogues & Bottom Sheets:** Floating modal surfaces feature `rounded-xl` (`1.5rem` / 24px) for upper corners.
- **Status Pills & Seat Toggles:** Dealer indicators, wind direction chips, and quick-filter buttons maintain full pill contours (`9999px`) to distinguish meta-states from square input cells.

## Components

### Buttons
- **Primary:** Solid `#1B6B4A` background with crisp white typography. Height: 48px; border-radius: 8px. Hover/tap state shifts to `#16563B` with no scale bouncing.
- **Secondary / Ghost:** Porcelain white background, border: `1px solid #E2E8F0`, slate-700 text (`#334155`). Tap state: `#F8F9FA`.
- **Destructive / Reset:** Off-white background, subtle cinnabar outline (`1px solid #FECACA`), text `#991B1B`.

### Cards & Player Seats
- **Scorecard Cell:** White container, `1px solid #F1F5F9` perimeter, 16px padding. Displays player avatar, seat wind token, current tally in tabular figures, and net delta (`+` in `#1B6B4A`, `-` in `#991B1B`).
- **Active Dealer Highlight:** Border upgrades subtly to `1.5px solid #1B6B4A` with a soft jade background tint (`#F0FDF4`).

### Chips & Badges
- **Wind Direction Tokens:** Pill format, 24px height, muted gray backdrop (`#F1F5F9`), slate text (`#475569`). Active dealer wind switches to `#FEF3C7` background with `#D97706` text.
- **Quick-Score Filters:** Height: 32px; padding: 0 12px; rounded to 9999px. Outline styling without drop shadows.

### Input Fields & Keypads
- **Point Adjustment Steppers:** Height: 44px; clean divided box design (`-` button, centered value, `+` button). Border: `1px solid #E2E8F0`, zero interior shadows.
- **Custom Numeric Entry:** Big, centered display numeral with lightweight helper units below. Clean tap response without keyboard vibration distraction.

### Lists & Match History
- **Round Ledger:** Borderless rows separated by soft hairline divider lines (`#F1F5F9`). Left: Round number and winning condition label. Right: Net transaction summary per seat.
- **Checkboxes & Radios:** Minimalist circles/squares with 2px solid jade checkmarks; inactive states use clean `#CBD5E1` outlines on white. No gradient fills.