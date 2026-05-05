# Brand Design: VaultMTG

## Brand Identity

**Tagline**: Your edge. Every draft. Every match.

**Voice**: Precise, confident, atmospheric

**Aesthetic**: VaultMTG occupies the space between a war room and an arcane library — dark surfaces lit by data. The visual language is premium and restrained: deep slate backgrounds, gold-touched amber accents that evoke treasure and mastery without copying MTG IP directly, and a data-first layout hierarchy where every number is immediately legible. Think analyst's dashboard meets fantasy compendium — functional at 2 AM during a competitive session, beautiful enough to screenshot.

---

## Color Palette

### Primary Accent

The primary accent is **Vault Amber** — a warm, saturated gold-amber that reads as "mastery" and "premium" without lifting any MTG gold directly. It pops cleanly against dark surfaces at all sizes.

| Name | Hex | Tailwind Token | Usage |
|---|---|---|---|
| Vault Amber | `#F5A623` | `primary-500` | CTAs, key highlights, active nav items |
| Amber Light | `#F7BA58` | `primary-400` | Hover states, icon fills |
| Amber Deep | `#C8841A` | `primary-600` | Pressed states, focus rings |

### Semantic Colors

| Token | Hex | Usage |
|---|---|---|
| `success` | `#22C55E` | Wins, positive ratings (A-tier), above-average win rates |
| `danger` | `#EF4444` | Losses, errors, F-tier ratings |
| `warning` | `#EAB308` | Caution, middling ratings (C-tier), near-threshold values |
| `muted` | `#64748B` | Secondary text, disabled states |
| `info` | `#38BDF8` | Neutral highlights, tooltips, informational callouts |

### Surface Colors (dark theme)

Surfaces use a cool-neutral dark slate family — not pure black (too harsh) and not warm gray (too generic). A slight blue-indigo tint gives the UI a polished, premium quality consistent with high-end gaming tools.

| Token | Hex | Usage |
|---|---|---|
| `surface-base` | `#0D1117` | Page background — near-black with a hint of navy |
| `surface-raised` | `#161C26` | Card and panel backgrounds |
| `surface-overlay` | `#1E2636` | Modals, dropdowns, popovers |
| `surface-border` | `#2A3347` | Dividers, input borders, table rules |

### Text Colors

| Token | Hex | Contrast on surface-base | Usage |
|---|---|---|---|
| `text-primary` | `#F1F5F9` | 15.8:1 | Main body text, card names, headings |
| `text-secondary` | `#94A3B8` | 7.2:1 | Supporting text, labels, metadata |
| `text-muted` | `#4E6080` | 4.6:1 | Placeholders, timestamps, fine print |

All text-on-background values meet or exceed WCAG AA (4.5:1 minimum). `text-primary` and `text-secondary` achieve WCAG AAA (7:1+).

---

## Typography

### Font Stack

- **Display/Headings**: [Sora](https://fonts.google.com/specimen/Sora) — A geometric sans-serif with clean angles and subtle futuristic character. Sora reads as "precision technology" without the retro-sci-fi clichés of condensed display fonts. Its distinct uppercase letterforms hold up well at large sizes for hero text and page headings.
- **Body**: [Inter](https://fonts.google.com/specimen/Inter) — The gold standard for UI readability. Designed explicitly for screens, Inter's tabular numerals make stat-heavy interfaces scannable at a glance. Near-universal legibility at 14–16px in dark contexts.
- **Monospace/Data**: [JetBrains Mono](https://fonts.google.com/specimen/JetBrains+Mono) — For win rate percentages, card ratings, ALSA values, and any data tables. Monospace alignment keeps columns tidy; JetBrains Mono has slightly wider characters than most mono fonts, making numeric data easier to read in compact stat rows.

### Type Scale

| Token | Size | Weight | Line Height | Usage |
|---|---|---|---|---|
| `text-xs` | 12px | 400 | 1.5 | Labels, badge text, metadata |
| `text-sm` | 14px | 400 | 1.5 | Secondary body, table rows |
| `text-base` | 16px | 400 | 1.6 | Primary body copy |
| `text-lg` | 18px | 500 | 1.4 | Section headers, card names |
| `text-xl` | 24px | 600 | 1.3 | Page subheadings |
| `text-2xl` | 32px | 700 | 1.2 | Page headings |
| `text-3xl` | 48px | 800 | 1.1 | Hero text, splash screens |

---

## Spacing & Shape

- **Base unit**: 4px
- **Border radius** — sm: 4px, default: 8px, lg: 12px, full: 9999px
- **Key spacings**: 4 / 8 / 12 / 16 / 24 / 32 / 48 / 64 / 96px
- **Elevation model**: surface-base → surface-raised (+1dp) → surface-overlay (+3dp). No box shadows with color; use border + background step to communicate elevation on dark themes.

---

## Tailwind Config Extension

```js
// Paste into tailwind.config.js → theme.extend
colors: {
  primary: {
    400: '#F7BA58',
    500: '#F5A623',
    600: '#C8841A',
  },
  surface: {
    base:    '#0D1117',
    raised:  '#161C26',
    overlay: '#1E2636',
    border:  '#2A3347',
  },
  text: {
    primary:   '#F1F5F9',
    secondary: '#94A3B8',
    muted:     '#4E6080',
  },
  success: '#22C55E',
  danger:  '#EF4444',
  warning: '#EAB308',
  info:    '#38BDF8',
},
fontFamily: {
  display: ['"Sora"', 'sans-serif'],
  body:    ['"Inter"', 'sans-serif'],
  mono:    ['"JetBrains Mono"', 'monospace'],
},
```

---

## CSS Custom Properties

```css
:root {
  /* Primary accent — Vault Amber */
  --color-primary:       #F5A623;
  --color-primary-light: #F7BA58;
  --color-primary-dark:  #C8841A;

  /* Surfaces */
  --surface-base:    #0D1117;
  --surface-raised:  #161C26;
  --surface-overlay: #1E2636;
  --surface-border:  #2A3347;

  /* Text */
  --text-primary:   #F1F5F9;
  --text-secondary: #94A3B8;
  --text-muted:     #4E6080;

  /* Semantic */
  --color-success: #22C55E;
  --color-danger:  #EF4444;
  --color-warning: #EAB308;
  --color-info:    #38BDF8;

  /* Typography */
  --font-display: 'Sora', sans-serif;
  --font-body:    'Inter', sans-serif;
  --font-mono:    'JetBrains Mono', monospace;

  /* Spacing base */
  --space-unit: 4px;

  /* Radii */
  --radius-sm:   4px;
  --radius-md:   8px;
  --radius-lg:   12px;
  --radius-full: 9999px;
}
```

---

## Component Specs

### 1. Draft Pick Card

Used in the draft helper overlay and the main draft view. The card must communicate card name, mana cost, color identity, tier rating, and win rate in a tight footprint — users are making time-pressured decisions.

**Layout (portrait card, ~160×220px):**

```
┌─────────────────────────────┐
│ [Color pip(s)]   [Mana cost]│  ← 12px, text-muted, font-mono
│                             │
│  Card Name                  │  ← text-lg, text-primary, font-body
│  Subtype / Card Type        │  ← text-xs, text-secondary
│                             │
│  ┌────────┐  Win Rate       │
│  │  A     │  63.2%          │  ← tier badge (see Stat Badge)
│  └────────┘  font-mono      │
│                             │
└─────────────────────────────┘
```

**States:**

- **Normal**: `background: surface-raised`, `border: 1px solid surface-border`
- **Hovered**: `background: surface-overlay`, `border-color: primary-500/50`, subtle amber glow `box-shadow: 0 0 0 1px #F5A62333`
- **Selected/Picked**: `border: 2px solid primary-500`, amber left accent bar (4px wide, full height), `background: surface-overlay`

**Color pip rendering**: small filled circle (8px) per MTG color — use neutral semantic colors that evoke but don't copy: W=`#E8E0C8`, U=`#4A90D9`, B=`#6B4F8E`, R=`#C0392B`, G=`#27AE60`. Multicolor uses the amber primary gradient.

**Key data shown**: card name, mana cost, color pip(s), tier rating (A/B/C/D/F), win rate %

---

### 2. Stat Badge

Compact, inline badge for numeric game stats. Used in draft card details, player profiles, and match history rows. Must be scannable in groups of 3–4.

**Layout (~80×28px):**

```
┌──────────────────┐
│ LABEL   63.2%    │
└──────────────────┘
```

- `font-family: font-mono`
- `font-size: text-xs (12px)` for label, `text-sm (14px)` for value
- `padding: 4px 8px`
- `border-radius: radius-sm (4px)`
- Label in `text-muted`, value in variant color below

**Variants:**

| Variant | Background | Text color | Trigger |
|---|---|---|---|
| Positive | `success/15` | `success` | Win rate ≥ 57%, A/B tier |
| Neutral | `surface-border/60` | `text-secondary` | Mid-range values, C tier |
| Negative | `danger/15` | `danger` | Win rate < 50%, D/F tier |

**Usage examples**: `GIH WR 63.2%`, `ALSA 2.4`, `ATA 3.1`, `GP WR 55.8%`

---

### 3. Match Result Row

Full-width list row in match history tables. Must communicate outcome at a glance while showing supporting context on the same line.

**Layout (full width, ~56px height):**

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ● WON   Azorius Tempo        vs. Mono-Red Aggro    May 4   22:14  18m  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│ ● LOST  Golgari Midrange     vs. Izzet Control     May 3   19:47  31m  │
└─────────────────────────────────────────────────────────────────────────┘
```

**Layout breakdown (left → right):**

1. **Outcome dot** (8px circle): `success` for win, `danger` for loss
2. **Outcome label** (`text-sm`, weight 600): "WON" in `success`, "LOST" in `danger`
3. **Player deck name** (`text-sm`, `text-primary`, truncated at 24ch)
4. **"vs." separator** (`text-xs`, `text-muted`)
5. **Opponent deck name** (`text-sm`, `text-secondary`, truncated at 22ch)
6. **Date** (`text-xs`, `text-muted`, font-mono)
7. **Time** (`text-xs`, `text-muted`, font-mono)
8. **Duration** (`text-xs`, `text-muted`, font-mono) — right-aligned

**States:**

- **Default**: `background: transparent`, `border-bottom: 1px solid surface-border`
- **Hovered**: `background: surface-raised`, left accent bar (3px, `success` or `danger` per outcome)
- **Active/expanded** (if clickable): `background: surface-overlay`, border-left becomes 4px

**Win row**: left border and dot use `success (#22C55E)`
**Loss row**: left border and dot use `danger (#EF4444)`
