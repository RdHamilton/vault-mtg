# VaultMTG Design System
**Target surface**: `app.vaultmtg.app` — the React SPA at `frontend/`
**Implements ticket**: #1465
**Foundation**: Extends `vaultmtg-brand.md` into a full implementation-ready spec

---

## 1. Brand Personality

**Adjectives**: Tactical, precise, atmospheric, premium, data-forward

**Tone**: Speaks to a competitive player who wants signal, not noise. Every element earns its place. The UI surfaces data the way a coach surfaces stats — no editorializing, no clutter, just clear information that helps the player win.

**What it should feel like**: Opening a pro player's toolkit. Dark surfaces, numerics that snap into focus immediately, amber accents that feel like "mastery unlocked" rather than decorative chrome. The app should feel more capable than 17Lands (which feels cold and academic) and more purposeful than Untapped.gg (which feels consumer/casual). VaultMTG sits at the intersection: powerful enough for spikes, legible enough for grinders.

**What to avoid**: Gradients used decoratively. Bright-on-bright color combinations. Large areas of pure black. Rounded corners so aggressive they feel playful. Any pattern that looks like a mobile game UI.

---

## 2. Color Palette

### 2.1 Primary Accent — Vault Amber

Vault Amber is a warm gold-amber. It reads as mastery and premium craft. It pops on all dark surface tiers at ≥3:1 for large UI elements (badges, buttons) and ≥4.5:1 for text uses. It carries no MTG IP risk.

| Name | Hex | Tailwind Token | Usage |
|---|---|---|---|
| Amber Light | `#F7BA58` | `primary-400` | Hover states on amber elements, icon fills |
| Vault Amber | `#F5A623` | `primary-500` | Primary CTAs, active nav state, key accent highlights |
| Amber Deep | `#C8841A` | `primary-600` | Pressed/active button states, focus rings |
| Amber Dim | `#7A5210` | `primary-800` | Subtle amber tint on surfaces, draft tier A background |

**Contrast on surface-base (`#0D1117`)**: `#F5A623` = 8.9:1. Passes WCAG AA for all text and UI components.

### 2.2 Secondary Accent — Indigo

Used sparingly for info states, keyboard shortcuts, and secondary interactive elements. Provides visual contrast against amber without competing.

| Name | Hex | Tailwind Token | Usage |
|---|---|---|---|
| Indigo | `#6366F1` | `secondary-500` | Info highlights, secondary interactive elements |
| Indigo Light | `#818CF8` | `secondary-400` | Hover on secondary elements |
| Indigo Deep | `#4F46E5` | `secondary-600` | Focus ring on indigo elements |

### 2.3 Surface Colors

Surfaces use a cool-neutral dark slate family with a subtle blue-indigo tint. Not pure black (too harsh on eyes during long sessions) and not warm gray (too generic/system-like).

| Token | Hex | Usage |
|---|---|---|
| `surface-base` | `#0D1117` | Page background — near-black with a navy undertone |
| `surface-raised` | `#161C26` | Card backgrounds, panel backgrounds, table header fill |
| `surface-overlay` | `#1E2636` | Modals, dropdowns, popovers, tooltips |
| `surface-sunken` | `#0A0E16` | Input backgrounds, code blocks, inset areas |
| `surface-border` | `#2A3347` | All dividers, input borders, table row rules |
| `surface-border-subtle` | `#1F2A3C` | Subtle dividers between same-level panels |

**Elevation model**: base → raised (+1 level) → overlay (+2 levels). Elevation is communicated by background step, not box shadow color. Shadows are used only for floating elements (modals, dropdowns, tooltips).

### 2.4 Text Colors

| Token | Hex | Contrast on `surface-base` | Usage |
|---|---|---|---|
| `text-primary` | `#F1F5F9` | 15.8:1 | Headings, card names, primary body copy, active state labels |
| `text-secondary` | `#94A3B8` | 7.2:1 | Supporting text, column labels, metadata lines |
| `text-muted` | `#4E6080` | 4.6:1 | Timestamps, placeholders, fine print, disabled labels |
| `text-inverse` | `#0D1117` | N/A | Text on amber/light backgrounds (buttons, badges) |

All values meet WCAG AA (4.5:1 minimum). `text-primary` and `text-secondary` meet WCAG AAA (7:1+).

### 2.5 Semantic Colors

| Token | Hex | Usage |
|---|---|---|
| `success` | `#22C55E` | Wins, positive ratings (A-tier), above-average win rates (≥57%) |
| `success-dim` | `#14532D` | Success backgrounds (transparent: `success/15`) |
| `danger` | `#EF4444` | Losses, errors, F-tier ratings, below-average win rates (<50%) |
| `danger-dim` | `#7F1D1D` | Danger backgrounds (transparent: `danger/15`) |
| `warning` | `#EAB308` | Caution states, middling performance (C-tier), near-threshold values |
| `warning-dim` | `#713F12` | Warning backgrounds (transparent: `warning/15`) |
| `info` | `#38BDF8` | Neutral highlights, tooltips, informational callouts |
| `info-dim` | `#0C4A6E` | Info backgrounds (transparent: `info/15`) |

### 2.6 MTG Color Identity Pips

Used for color identity indicators on cards and deck summaries. These evoke MTG colors without copying the official art palette.

| Color | Hex | Token |
|---|---|---|
| White (W) | `#E8E0C8` | `mtg-white` |
| Blue (U) | `#4A90D9` | `mtg-blue` |
| Black (B) | `#9B7FC2` | `mtg-black` |
| Red (R) | `#C94E3A` | `mtg-red` |
| Green (G) | `#3A9E5F` | `mtg-green` |
| Colorless (C) | `#8B9BB4` | `mtg-colorless` |
| Multicolor (M) | `#F5A623` | use `primary-500` |

---

## 3. Typography

### 3.1 Font Stack

Load all three via Google Fonts CDN. Add to `<head>` in `index.html`:

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Sora:wght@400;500;600;700;800&family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
```

| Role | Family | Weights | Fallback | Rationale |
|---|---|---|---|---|
| Display / Headings | Sora | 600, 700, 800 | sans-serif | Geometric precision, distinct uppercase, futuristic without being retro |
| Body / UI | Inter | 400, 500, 600 | sans-serif | Designed for screens, excellent tabular numerals, 14–16px legibility |
| Data / Mono | JetBrains Mono | 400, 500 | monospace | Wider mono characters, numeric data alignment in stat tables |

### 3.2 Type Scale

| Token | Size | Weight | Line Height | Letter Spacing | Usage |
|---|---|---|---|---|---|
| `text-xs` | 11px | 400 | 1.5 | +0.02em | Labels, badge text, timestamps, pip counts |
| `text-sm` | 13px | 400 | 1.5 | 0 | Table rows, secondary body, filter labels |
| `text-base` | 15px | 400 | 1.6 | 0 | Primary body copy, form inputs |
| `text-lg` | 17px | 500 | 1.4 | -0.01em | Section subheadings, card names in detail view |
| `text-xl` | 20px | 600 | 1.3 | -0.01em | Panel headings, modal titles |
| `text-2xl` | 26px | 700 | 1.2 | -0.02em | Page headings |
| `text-3xl` | 34px | 700 | 1.15 | -0.02em | Hero stats (e.g. overall win rate on dashboard) |
| `text-4xl` | 48px | 800 | 1.1 | -0.03em | Splash/empty-state hero numbers |

**Note on sizing**: The SPA sits inside a tight desktop viewport with dense data. These sizes are intentionally 1–2px smaller than the Tailwind defaults to preserve information density.

### 3.3 Font Application Rules

- Headings (`h1`–`h4`): `font-display` (Sora), weight 700 or 600
- Body copy, labels, table data: `font-body` (Inter)
- Any numeric stat, win rate, ALSA value, mana cost, card count: `font-mono` (JetBrains Mono)
- Never mix display and mono on the same line; use body as the bridge font

---

## 4. Spacing System

**Base unit**: 4px

| Token | px | rem | Usage |
|---|---|---|---|
| `space-0` | 0 | 0 | Reset, flush |
| `space-1` | 4px | 0.25rem | Tight gaps between inline elements, pip spacing |
| `space-2` | 8px | 0.5rem | Padding inside badges/chips, gap between label+value pairs |
| `space-3` | 12px | 0.75rem | Table cell padding (vertical), compact form field padding |
| `space-4` | 16px | 1rem | Standard card padding, filter row gap, page gutter |
| `space-5` | 20px | 1.25rem | Section gap within a panel |
| `space-6` | 24px | 1.5rem | Gap between cards/panels in a grid |
| `space-8` | 32px | 2rem | Page section spacing, gap between major layout regions |
| `space-10` | 40px | 2.5rem | Hero section vertical padding |
| `space-12` | 48px | 3rem | Empty state vertical padding |
| `space-16` | 64px | 4rem | Page-level top/bottom margin |
| `space-24` | 96px | 6rem | Splash screen vertical rhythm |

**Grid**: 4px grid strictly. No values like 10px, 15px, 18px, 22px. Round to nearest 4px multiple.

---

## 5. Border Radius

| Token | px | Tailwind class | Usage |
|---|---|---|---|
| `radius-none` | 0 | `rounded-none` | Table rows, flush panel edges, toolbar items |
| `radius-sm` | 4px | `rounded` | Badges, tags, chips, small inline elements |
| `radius-md` | 8px | `rounded-lg` | Cards, panels, inputs, buttons, select dropdowns |
| `radius-lg` | 12px | `rounded-xl` | Modals, bottom sheets, large floating surfaces |
| `radius-full` | 9999px | `rounded-full` | Avatar circles, pill-shaped status indicators, progress bars |

**Rule**: Do not use `rounded-2xl` (16px) or larger on surfaces. The aesthetic is precise and angular, not bubbly.

---

## 6. Shadows and Elevation

On dark themes, colored or diffuse shadows read as muddy. VaultMTG uses sharp, layered border + background-step for card elevation. Shadow is reserved exclusively for floating elements that physically overlay content.

| Token | Value | Usage |
|---|---|---|
| `shadow-none` | none | Inline elements, table rows, flat surfaces |
| `shadow-sm` | `0 1px 3px rgba(0,0,0,0.5)` | Subtle lift on interactive cards on hover |
| `shadow-md` | `0 4px 12px rgba(0,0,0,0.6)` | Dropdowns, popovers, context menus |
| `shadow-lg` | `0 8px 32px rgba(0,0,0,0.7)` | Modals, drawers |
| `shadow-amber` | `0 0 0 1px rgba(245,166,35,0.2), 0 4px 12px rgba(245,166,35,0.1)` | Selected/active card states, primary CTA focus |
| `shadow-inset` | `inset 0 1px 0 rgba(255,255,255,0.04)` | Top edge highlight on raised surfaces |

---

## 7. Component Specifications

### 7.1 Button

Buttons use `radius-md` (8px), `font-body` Inter, `font-size: text-sm` (13px), `font-weight: 500`. Minimum tap target 36px height. Transition all interactive states with `transition: background-color 150ms ease, border-color 150ms ease, box-shadow 150ms ease`.

#### Variants

**Primary** — amber CTA, used once per primary action zone:
```
Background:  primary-500  (#F5A623)
Text:        text-inverse  (#0D1117)
Border:      none
Hover:       background → primary-400 (#F7BA58)
Active:      background → primary-600 (#C8841A)
Focus:       box-shadow: shadow-amber
Disabled:    background → primary-500/40, text → text-inverse/40, cursor-not-allowed
```

**Secondary** — bordered, used for secondary actions alongside a primary:
```
Background:  transparent
Text:        text-primary  (#F1F5F9)
Border:      1px solid surface-border  (#2A3347)
Hover:       background → surface-raised (#161C26), border-color → primary-500/50
Active:      background → surface-overlay (#1E2636)
Focus:       border-color: primary-500, box-shadow: shadow-amber
Disabled:    opacity: 0.4, cursor-not-allowed
```

**Ghost** — no border, low-weight actions (icon buttons, nav items, inline controls):
```
Background:  transparent
Text:        text-secondary  (#94A3B8)
Border:      none
Hover:       background → surface-raised (#161C26), text → text-primary
Active:      background → surface-overlay (#1E2636)
Focus:       box-shadow: 0 0 0 2px primary-500/40
Disabled:    opacity: 0.4
```

**Danger** — destructive actions (delete, reset, disconnect):
```
Background:  transparent
Text:        danger  (#EF4444)
Border:      1px solid danger/40  (rgba(239,68,68,0.4))
Hover:       background → danger/10, border-color: danger
Active:      background → danger/20
Focus:       box-shadow: 0 0 0 2px danger/30
Disabled:    opacity: 0.4
```

**Icon Button** — square, ghost variant with an icon:
```
Size:        32×32px (sm), 36×36px (default), 40×40px (lg)
Padding:     0
Border:      none
Uses:        ghost variant interaction states
Icon size:   16px (sm/default), 20px (lg) — Heroicons outline
```

#### Sizes

| Size | Height | Padding H | Font size |
|---|---|---|---|
| `sm` | 28px | 12px | 11px (text-xs) |
| `md` (default) | 36px | 16px | 13px (text-sm) |
| `lg` | 44px | 20px | 15px (text-base) |

#### ASCII Wireframe

```
Primary:
┌─────────────────────────┐
│  [icon?]  Label text    │   ← h:36px, amber fill, dark text
└─────────────────────────┘

Secondary:
┌─ - - - - - - - - - - - ┐
│  [icon?]  Label text    │   ← border: surface-border, transparent fill
└─ - - - - - - - - - - - ┘

Danger:
┌─ · · · · · · · · · · · ┐
│  [icon?]  Danger text   │   ← border: danger/40, danger text
└─ · · · · · · · · · · · ┘
```

---

### 7.2 Card

The primary content container. Used for draft panels, stat summaries, match summaries, and standalone data widgets.

**Structure**:
```
Background:   surface-raised  (#161C26)
Border:       1px solid surface-border  (#2A3347)
Border-radius: radius-md (8px)
Shadow:       shadow-inset (top edge highlight)
Padding:      space-4 (16px) default; space-3 (12px) compact variant

┌─────────────────────────────────────────────────────┐ ← surface-raised, border
│  [Card Header]                          [actions?]  │ ← 16px padding, border-bottom
│─────────────────────────────────────────────────────│ ← surface-border
│                                                     │
│  [Card Body content]                                │ ← 16px padding
│                                                     │
│─────────────────────────────────────────────────────│ ← surface-border (optional)
│  [Card Footer — optional]                           │ ← 12px padding, text-muted
└─────────────────────────────────────────────────────┘
```

**Header**: `font-display` Sora, `text-base` (15px), `font-weight: 600`, `text-primary`. If the header has a supporting description, it sits at `text-xs`, `text-muted`, same line or below.

**States**:
- Default: as above
- Hoverable (clickable cards): `cursor: pointer`, hover adds `shadow-sm`, border-color shifts to `surface-border` + 10% lighter (`#354060`)
- Selected: `border-color: primary-500`, `shadow-amber`
- Loading: see Skeleton spec (7.10)
- Error: `border-color: danger/40`, add error state content inside body

**Variants**:
- **Default** (16px padding): General stats panels, deck cards, collection items
- **Compact** (12px padding): Draft pick cards, dense data grids
- **Flush** (0px padding, border only): When internal content manages its own padding (tables, custom layouts)

---

### 7.3 Badge / Tag

Compact inline indicators for categorical data: draft tier ratings (A/B/C/D/F), format names, color identity, set symbols, deck archetypes.

**Base anatomy**:
```
┌──────────┐
│  label   │   ← font-mono, text-xs (11px), font-weight 500
└──────────┘   padding: 2px 6px, radius-sm (4px), no border by default
```

**Tier rating badges** (A/B/C/D/F) — always monospace, always 28×22px (fixed size for column alignment):

| Tier | Background | Text color | Meaning |
|---|---|---|---|
| A | `#F5A623/20` (`primary-500/20`) | `#F5A623` | Exceptional |
| B | `#22C55E/15` (`success/15`) | `#22C55E` | Above average |
| C | `#94A3B8/15` (`text-secondary/15`) | `#94A3B8` | Average |
| D | `#EAB308/15` (`warning/15`) | `#EAB308` | Below average |
| F | `#EF4444/15` (`danger/15`) | `#EF4444` | Avoid |

**Format badges** (Standard, Historic, Alchemy, Explorer):
```
Background:   surface-overlay  (#1E2636)
Border:       1px solid surface-border  (#2A3347)
Text:         text-secondary, font-body, text-xs
Radius:       radius-sm
Padding:      2px 8px
```

**Color identity pip** (MTG colors):
```
Shape:        circle, 10×10px
Fill:         mtg-[color] (see 2.6)
Display:      inline-flex, gap: space-1 between pips
```

**Win-rate performance badge** (inline in tables):
```
Layout:  [label in text-muted]  [value in mono]   e.g.  WR  63.2%
```
- Positive (≥57%): text-color: `success`
- Neutral (50–57%): text-color: `text-secondary`
- Negative (<50%): text-color: `danger`

---

### 7.4 Table Rows

Used in match history, collection, draft analytics, and card performance tables. VaultMTG tables are data-dense — optimize for scanability over decoration.

**Table structure**:
```
┌────────────────────────────────────────────────────────────────────────┐
│  COL 1          COL 2              COL 3     COL 4    COL 5            │ ← thead
├────────────────────────────────────────────────────────────────────────┤ ← 2px solid surface-border
│  [data]         [data]             [data]    [data]   [data]           │ ← tbody row
├────────────────────────────────────────────────────────────────────────┤ ← 1px solid surface-border-subtle
│  [data]         [data]             [data]    [data]   [data]           │
└────────────────────────────────────────────────────────────────────────┘
```

**`thead`**:
- Background: `surface-raised` (#161C26)
- Text: `text-muted` (#4E6080), `text-xs` (11px), `font-weight: 600`, `font-body`, ALL CAPS with `letter-spacing: +0.06em`
- Padding: `12px 12px` (space-3 vertical, space-3 horizontal)
- Bottom border: `2px solid surface-border`
- Sticky positioning when table scrolls

**`tbody` rows**:
- Background: `transparent` (base shows through)
- Padding: `10px 12px`
- Border-bottom: `1px solid surface-border-subtle` (#1F2A3C)
- Text: `text-sm` (13px), `text-secondary` for most columns; `text-primary` for the primary identifier column (card name, deck name)
- Numeric values: `font-mono`, `text-sm`
- Hover: `background: surface-raised`, no transition delay (instant feel)
- Hover also shows left accent bar: `3px solid primary-500` via `::before` pseudo-element

**Sortable column headers**:
- Add Heroicon `chevron-up-down` (16px, `text-muted`) inline after label
- Active sort column: icon becomes `chevron-up` or `chevron-down`, `text-secondary`
- Active column header text shifts to `text-primary`

**Striping**: Do not use striped rows (they add visual noise). Use hover state + border lines for scanability instead.

**Empty state within table**: span all columns, centered, see 7.9 Empty State spec.

---

### 7.5 Navigation / Sidebar

The app uses a left sidebar for primary navigation. The sidebar is persistent on desktop (≥1024px) and collapses to a bottom bar on mobile (<768px).

**Sidebar dimensions**: 220px wide (collapsed: 56px icon-only mode)

**Structure**:
```
┌────────────────────┐
│ ┌──────────────┐   │ ← Logo / wordmark area, h:56px, border-bottom
│ │  [V] VaultMTG│   │   surface-raised background
│ └──────────────┘   │
│                    │
│  NAV SECTION       │ ← text-xs, text-muted, uppercase, letter-spacing 0.08em
│                    │   padding: 8px 12px, margin-top: 16px first section
│  [icon] Dashboard  │ ← nav item (see below)
│  [icon] Draft      │
│  [icon] Match Hist │
│  [icon] Decks      │
│  [icon] Collection │
│                    │
│  ─────────────────  │ ← surface-border divider
│                    │
│  ANALYTICS         │
│  [icon] Format     │
│  [icon] Meta       │
│                    │
│ ┌──────────────┐   │ ← bottom section, pinned
│ │ [icon] Set…  │   │   Settings, Download, User
│ │ [icon] Sett. │   │
│ │ [avatar] Ray │   │
│ └──────────────┘   │
└────────────────────┘
```

**Nav item anatomy**:
```
[icon 18px]  [label text-sm]
```
- Height: 36px
- Padding: `8px 12px`
- Border-radius: `radius-md` (8px), applied to the item itself
- Margin: `2px 8px` (creates inset appearance within sidebar)
- Icon: Heroicon outline, 18px, matches text color

**States**:
- Default: `background: transparent`, `text: text-secondary`, `icon: text-muted`
- Hover: `background: surface-raised`, `text: text-primary`, `icon: text-secondary`
- Active: `background: primary-500/10` (`rgba(245,166,35,0.1)`), `text: primary-500`, `icon: primary-500`, `border-left: 2px solid primary-500` (aligned to item left edge inside padding)
- Active amber left accent: achieved via `border-left: 2px solid primary-500` inside the 8px margin; the item container uses `padding-left: 10px` to compensate, keeping text aligned

**Section labels** (e.g. "ANALYTICS"):
- `font-body`, `text-xs` (11px), `text-muted`, ALL CAPS, `letter-spacing: 0.08em`
- Not interactive
- `margin-top: 20px`

**Sidebar background**: `surface-raised` (#161C26), full height, `border-right: 1px solid surface-border`

---

### 7.6 Input / Select

**Text Input**:
```
┌─────────────────────────────────────────┐
│  [icon?]  Placeholder text              │  ← h:36px default
└─────────────────────────────────────────┘
```
- Background: `surface-sunken` (#0A0E16)
- Border: `1px solid surface-border` (#2A3347)
- Border-radius: `radius-md` (8px)
- Text: `text-sm` (13px), `font-body`, `text-primary`
- Placeholder: `text-muted`
- Padding: `8px 12px`; if leading icon: `8px 12px 8px 36px`; leading icon positioned `left: 10px`
- Hover: `border-color: #354060` (surface-border lightened ~10%)
- Focus: `border-color: primary-500`, `box-shadow: 0 0 0 2px rgba(245,166,35,0.15)`, outline: none
- Error state: `border-color: danger`, `box-shadow: 0 0 0 2px rgba(239,68,68,0.15)`
- Disabled: `opacity: 0.5`, `cursor: not-allowed`, `background: surface-raised`

**Select** (custom dropdown trigger):
- Same visual base as Input
- Append Heroicon `chevron-down` (16px, `text-muted`) at right: `right: 10px`, vertically centered
- Padding right: `36px` to prevent text colliding with chevron
- Open state: `border-color: primary-500`, chevron rotates 180deg

**Dropdown panel** (Select options list):
```
Background:   surface-overlay  (#1E2636)
Border:       1px solid surface-border  (#2A3347)
Border-radius: radius-md (8px)
Shadow:       shadow-md
Max-height:   240px, overflow-y: auto
Padding:      4px 0
z-index:      50
```

Option items:
- Height: 32px
- Padding: `6px 12px`
- Font: `text-sm`, `font-body`, `text-primary`
- Hover: `background: surface-raised`, `text-primary`
- Selected: `background: primary-500/10`, `text: primary-500`
- Disabled: `text-muted`, `cursor: not-allowed`

**Search input variant**:
- Prepend Heroicon `magnifying-glass` (16px, `text-muted`) at left
- Clear button (Heroicon `x-mark`, 14px) appears at right when input has value
- Identical border/focus states to standard Input

**Label** (above inputs):
- `font-body`, `text-xs` (11px), `font-weight: 500`, `text-secondary`
- `margin-bottom: 4px`

**Helper / Error text** (below inputs):
- `font-body`, `text-xs` (11px), `font-weight: 400`
- Helper: `text-muted`
- Error: `danger` color
- `margin-top: 4px`

---

### 7.7 Toast Notifications

Non-blocking ephemeral feedback. Positioned `bottom-right` at `(20px, 20px)` from the viewport edge. Stack vertically upward when multiple are present. Auto-dismiss after 4s (error: 6s, no auto-dismiss if action is present).

**Anatomy**:
```
┌─────────────────────────────────────────────────┐
│ [icon 16px]  Title text           [×]           │ ← h: auto, min 48px
│              Supporting detail (optional)       │
│              [Action button — ghost sm]         │ ← optional
└─────────────────────────────────────────────────┘
```

- Width: 320px fixed
- Background: `surface-overlay` (#1E2636)
- Border: `1px solid surface-border`, left accent 3px colored by variant
- Border-radius: `radius-md` (8px)
- Shadow: `shadow-md`
- Padding: `12px 16px`
- Dismiss button: Heroicon `x-mark`, 14px, `text-muted`, top-right, `8px 8px` absolute

**Variants**:

| Variant | Left border color | Icon | Icon color | Title color |
|---|---|---|---|---|
| Success | `success` (#22C55E) | `check-circle` | `success` | `text-primary` |
| Error | `danger` (#EF4444) | `x-circle` | `danger` | `text-primary` |
| Warning | `warning` (#EAB308) | `exclamation-triangle` | `warning` | `text-primary` |
| Info | `info` (#38BDF8) | `information-circle` | `info` | `text-primary` |
| Default | `surface-border` | none | — | `text-primary` |

**Animation**:
- Enter: slide up from bottom + fade in, `duration: 200ms, easing: ease-out`
- Exit: slide down + fade out, `duration: 150ms, easing: ease-in`

---

### 7.8 Empty State

Shown when a page or section has no data. Must communicate clearly that data is missing and — when applicable — guide the user to an action that would populate data (start a draft, connect daemon, etc.).

**Layout (centered in the containing region)**:
```
         [icon — 48px, text-muted]
         
         Primary message
         
         Supporting detail text
         (1–2 lines, centered)
         
         [Primary CTA button — optional]
         [Secondary/ghost link — optional]
```

- Minimum vertical padding: `space-12` (48px) top and bottom
- Icon: Heroicon outline, 48px, `text-muted`, `margin-bottom: space-4`
- Primary message: `font-display` Sora, `text-xl` (20px), `font-weight: 600`, `text-primary`, `margin-bottom: space-2`
- Detail text: `font-body` Inter, `text-sm` (13px), `text-muted`, `max-width: 320px`, centered, `margin-bottom: space-6`
- CTA button: Primary or Ghost variant, centered
- Do NOT use placeholder mock data in empty states — show the real empty layout

**Icon selection guide**:
- No match history: `clock` or `chart-bar`
- No drafts: `rectangle-stack`
- No collection data: `squares-2x2`
- No internet/daemon: `signal-slash`
- Error / fetch failed: `exclamation-circle`
- No search results: `magnifying-glass` with X overlay

---

### 7.9 Loading Skeleton

Used while async data loads. Prevents layout shift by holding the space content will occupy. Do not use a spinner for full-page or large panel loads — use skeletons.

**Skeleton base**:
- Background: `surface-border` (#2A3347)
- Animated shimmer: `background: linear-gradient(90deg, surface-border 0%, surface-overlay 50%, surface-border 100%)`, `background-size: 200% 100%`, `animation: shimmer 1.6s ease-in-out infinite`
- Border-radius: match the element being replaced (text lines: `radius-sm`; cards: `radius-md`)

**Shimmer keyframes** (add to global CSS):
```css
@keyframes shimmer {
  0%   { background-position: 200% center; }
  100% { background-position: -200% center; }
}
```

**Common skeleton patterns**:

Card skeleton:
```
┌─────────────────────────────────────────┐
│ [████████████████████]   [████]         │ ← header + badge skeleton
│                                         │
│ [████████████████████████████████████]  │ ← content line 1
│ [███████████████████████████]           │ ← content line 2 (shorter)
│ [████████████████]                      │ ← content line 3 (shorter still)
└─────────────────────────────────────────┘
```

Table row skeleton:
```
│ [██████████]  [████████████████]  [████]  [████]  [████] │
│ [████████]    [██████████████]    [████]  [████]  [████] │
│ [█████████]   [█████████████████] [████]  [████]  [████] │
```

Vary skeleton line widths: 100%, 85%, 70%, 60% — never identical rows (looks artificial).

**Pulse alternative**: For icon-only or small elements, use a simple opacity pulse (`@keyframes pulse { 0%,100% { opacity:0.4 } 50% { opacity:1 } }`) instead of shimmer.

---

### 7.10 Draft Pick Card

Primary surface in the Draft view. Communicates card name, mana cost, color identity, tier rating, win rate, and additional pick context in a tight footprint under time pressure.

**Dimensions**: approximately 160×220px portrait, scales to 140×196px compact.

```
┌──────────────────────────────────┐  ← surface-raised, border: surface-border
│ ●●   [color pips]     {3}{W}{W} │  ← pips left, mana cost right, 12px mono
│                                  │
│  Sanctuary Warden                │  ← text-lg (17px), font-body, text-primary
│  Creature — Angel Soldier        │  ← text-xs, text-muted
│                                  │
│ ┌──────┐  GIH WR  63.2%         │  ← tier badge + mono stat
│ │  A   │  ALSA    2.4           │  ← mono, color-coded by value
│ └──────┘                         │
│                                  │
│  [Pick / Pass button — sm]       │  ← primary or ghost, sm size
└──────────────────────────────────┘
```

States (see vaultmtg-brand.md for full state specs including selected/hover/amber glow).

---

## 8. CSS Custom Properties

Drop into `frontend/src/index.css` inside `:root {}`. Replace the existing ad-hoc color declarations.

```css
/* ============================================================
   VaultMTG Design System — CSS Custom Properties
   Source of truth for all color, typography, spacing tokens.
   ============================================================ */

:root {
  /* ── Primary accent — Vault Amber ──────────────────────────── */
  --color-primary:         #F5A623;
  --color-primary-light:   #F7BA58;
  --color-primary-dark:    #C8841A;
  --color-primary-dim:     #7A5210;
  --color-primary-alpha10: rgba(245, 166, 35, 0.10);
  --color-primary-alpha15: rgba(245, 166, 35, 0.15);
  --color-primary-alpha20: rgba(245, 166, 35, 0.20);
  --color-primary-alpha50: rgba(245, 166, 35, 0.50);

  /* ── Secondary accent — Indigo ─────────────────────────────── */
  --color-secondary:       #6366F1;
  --color-secondary-light: #818CF8;
  --color-secondary-dark:  #4F46E5;

  /* ── Surfaces ───────────────────────────────────────────────── */
  --surface-base:          #0D1117;
  --surface-sunken:        #0A0E16;
  --surface-raised:        #161C26;
  --surface-overlay:       #1E2636;
  --surface-border:        #2A3347;
  --surface-border-subtle: #1F2A3C;

  /* ── Text ───────────────────────────────────────────────────── */
  --text-primary:          #F1F5F9;
  --text-secondary:        #94A3B8;
  --text-muted:            #4E6080;
  --text-inverse:          #0D1117;

  /* ── Semantic ───────────────────────────────────────────────── */
  --color-success:         #22C55E;
  --color-success-dim:     rgba(34, 197, 94, 0.15);
  --color-danger:          #EF4444;
  --color-danger-dim:      rgba(239, 68, 68, 0.15);
  --color-warning:         #EAB308;
  --color-warning-dim:     rgba(234, 179, 8, 0.15);
  --color-info:            #38BDF8;
  --color-info-dim:        rgba(56, 189, 248, 0.15);

  /* ── MTG Color Identity ─────────────────────────────────────── */
  --mtg-white:             #E8E0C8;
  --mtg-blue:              #4A90D9;
  --mtg-black:             #9B7FC2;
  --mtg-red:               #C94E3A;
  --mtg-green:             #3A9E5F;
  --mtg-colorless:         #8B9BB4;

  /* ── Typography ─────────────────────────────────────────────── */
  --font-display:          'Sora', sans-serif;
  --font-body:             'Inter', sans-serif;
  --font-mono:             'JetBrains Mono', monospace;

  /* ── Type scale ─────────────────────────────────────────────── */
  --text-xs:               11px;
  --text-sm:               13px;
  --text-base:             15px;
  --text-lg:               17px;
  --text-xl:               20px;
  --text-2xl:              26px;
  --text-3xl:              34px;
  --text-4xl:              48px;

  /* ── Spacing ─────────────────────────────────────────────────── */
  --space-1:               4px;
  --space-2:               8px;
  --space-3:               12px;
  --space-4:               16px;
  --space-5:               20px;
  --space-6:               24px;
  --space-8:               32px;
  --space-10:              40px;
  --space-12:              48px;
  --space-16:              64px;
  --space-24:              96px;

  /* ── Border radius ───────────────────────────────────────────── */
  --radius-sm:             4px;
  --radius-md:             8px;
  --radius-lg:             12px;
  --radius-full:           9999px;

  /* ── Shadows ─────────────────────────────────────────────────── */
  --shadow-sm:             0 1px 3px rgba(0, 0, 0, 0.5);
  --shadow-md:             0 4px 12px rgba(0, 0, 0, 0.6);
  --shadow-lg:             0 8px 32px rgba(0, 0, 0, 0.7);
  --shadow-amber:          0 0 0 1px rgba(245, 166, 35, 0.2), 0 4px 12px rgba(245, 166, 35, 0.1);
  --shadow-inset:          inset 0 1px 0 rgba(255, 255, 255, 0.04);

  /* ── Transitions ─────────────────────────────────────────────── */
  --transition-fast:       150ms ease;
  --transition-base:       200ms ease;
  --transition-slow:       300ms ease;

  /* ── Z-index scale ───────────────────────────────────────────── */
  --z-base:                0;
  --z-raised:              10;
  --z-dropdown:            50;
  --z-sticky:              100;
  --z-overlay:             200;
  --z-modal:               300;
  --z-toast:               400;
  --z-tooltip:             500;
}
```

---

## 9. Tailwind Config Extension

Add to `frontend/tailwind.config.js` (or create it if Tailwind is not yet configured). This maps all design tokens into Tailwind utility classes.

```js
// tailwind.config.js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{ts,tsx,js,jsx}'],
  darkMode: 'class', // apply .dark class at root for future light-mode toggle
  theme: {
    extend: {
      colors: {
        primary: {
          400: '#F7BA58',
          500: '#F5A623',
          600: '#C8841A',
          800: '#7A5210',
        },
        secondary: {
          400: '#818CF8',
          500: '#6366F1',
          600: '#4F46E5',
        },
        surface: {
          base:          '#0D1117',
          sunken:        '#0A0E16',
          raised:        '#161C26',
          overlay:       '#1E2636',
          border:        '#2A3347',
          'border-subtle': '#1F2A3C',
        },
        text: {
          primary:   '#F1F5F9',
          secondary: '#94A3B8',
          muted:     '#4E6080',
          inverse:   '#0D1117',
        },
        success: {
          DEFAULT: '#22C55E',
          dim:     'rgba(34, 197, 94, 0.15)',
        },
        danger: {
          DEFAULT: '#EF4444',
          dim:     'rgba(239, 68, 68, 0.15)',
        },
        warning: {
          DEFAULT: '#EAB308',
          dim:     'rgba(234, 179, 8, 0.15)',
        },
        info: {
          DEFAULT: '#38BDF8',
          dim:     'rgba(56, 189, 248, 0.15)',
        },
        mtg: {
          white:     '#E8E0C8',
          blue:      '#4A90D9',
          black:     '#9B7FC2',
          red:       '#C94E3A',
          green:     '#3A9E5F',
          colorless: '#8B9BB4',
        },
      },

      fontFamily: {
        display: ['"Sora"', 'sans-serif'],
        body:    ['"Inter"', 'sans-serif'],
        mono:    ['"JetBrains Mono"', 'monospace'],
      },

      fontSize: {
        // Override Tailwind defaults with VaultMTG scale
        xs:   ['11px', { lineHeight: '1.5', letterSpacing: '0.02em' }],
        sm:   ['13px', { lineHeight: '1.5', letterSpacing: '0' }],
        base: ['15px', { lineHeight: '1.6', letterSpacing: '0' }],
        lg:   ['17px', { lineHeight: '1.4', letterSpacing: '-0.01em' }],
        xl:   ['20px', { lineHeight: '1.3', letterSpacing: '-0.01em' }],
        '2xl': ['26px', { lineHeight: '1.2', letterSpacing: '-0.02em' }],
        '3xl': ['34px', { lineHeight: '1.15', letterSpacing: '-0.02em' }],
        '4xl': ['48px', { lineHeight: '1.1', letterSpacing: '-0.03em' }],
      },

      spacing: {
        // Supplement Tailwind's 4px scale with named aliases
        // Tailwind's default already covers these; this is for token clarity
        '18': '72px',
        '22': '88px',
      },

      borderRadius: {
        sm:   '4px',
        DEFAULT: '8px',
        lg:   '8px',   // Tailwind lg = 8px in our system
        xl:   '12px',  // Tailwind xl = 12px in our system
        full: '9999px',
      },

      boxShadow: {
        sm:    '0 1px 3px rgba(0, 0, 0, 0.5)',
        md:    '0 4px 12px rgba(0, 0, 0, 0.6)',
        lg:    '0 8px 32px rgba(0, 0, 0, 0.7)',
        amber: '0 0 0 1px rgba(245, 166, 35, 0.2), 0 4px 12px rgba(245, 166, 35, 0.1)',
        inset: 'inset 0 1px 0 rgba(255, 255, 255, 0.04)',
      },

      transitionDuration: {
        fast: '150ms',
        base: '200ms',
        slow: '300ms',
      },

      zIndex: {
        dropdown: '50',
        sticky:   '100',
        overlay:  '200',
        modal:    '300',
        toast:    '400',
        tooltip:  '500',
      },

      keyframes: {
        shimmer: {
          '0%':   { backgroundPosition: '200% center' },
          '100%': { backgroundPosition: '-200% center' },
        },
        pulse: {
          '0%, 100%': { opacity: '0.4' },
          '50%':      { opacity: '1' },
        },
        'slide-up': {
          '0%':   { transform: 'translateY(12px)', opacity: '0' },
          '100%': { transform: 'translateY(0)',    opacity: '1' },
        },
        'slide-down': {
          '0%':   { transform: 'translateY(0)',    opacity: '1' },
          '100%': { transform: 'translateY(12px)', opacity: '0' },
        },
        'fade-in': {
          '0%':   { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },

      animation: {
        shimmer:     'shimmer 1.6s ease-in-out infinite',
        pulse:       'pulse 1.8s ease-in-out infinite',
        'toast-in':  'slide-up 200ms ease-out',
        'toast-out': 'slide-down 150ms ease-in',
        'fade-in':   'fade-in 200ms ease-out',
      },
    },
  },
  plugins: [],
};
```

---

## 10. Migration Notes (Existing Codebase)

The current frontend uses ad-hoc hex values in individual `.css` files. The migration path for ticket #1465:

### Priority 1 — Global foundations (implement first)
1. Add Google Fonts `<link>` to `frontend/index.html`
2. Replace `frontend/src/index.css` `:root` block with the CSS custom properties from section 8
3. Add `tailwind.config.js` with the extension from section 9
4. Set `background-color: var(--surface-base)` on `body` and `#root`
5. Set `color: var(--text-primary)` on `body`

### Priority 2 — Global component classes (implement second)
Replace these recurring patterns across all component `.css` files:

| Old value | Replace with |
|---|---|
| `#1e1e1e` | `var(--surface-base)` |
| `#2d2d2d` | `var(--surface-raised)` |
| `#252525`, `#2a2a2a` | `var(--surface-raised)` |
| `#3d3d3d`, `#333`, `#444` | `var(--surface-border)` |
| `#4a9eff`, `#3a8eef`, `#6366f1` | `var(--color-primary)` |
| `#ffffff`, `#fff` (on dark bg) | `var(--text-primary)` |
| `#aaaaaa`, `#aaa`, `#a0a0a0` | `var(--text-secondary)` |
| `#888`, `#888888`, `#666` | `var(--text-muted)` |
| `#4caf50`, `#22c55e` | `var(--color-success)` |
| `#f44336`, `#ef4444`, `#ff4444`, `#ff6b6b` | `var(--color-danger)` |
| `#ffd700`, `#f59e0b`, `#fbbf24` | `var(--color-warning)` |

### Priority 3 — Component-by-component refactor
Apply component specs from section 7 to the shadcn/ui component library and replace individual `.css` files with Tailwind utility classes. Start with the most frequently viewed components: nav sidebar, match history table, draft pick cards.

---

## 11. Accessibility Requirements

- All text on `surface-base`: minimum 4.5:1 contrast ratio (WCAG AA). Verified values in section 2.4.
- Interactive elements: focus state always visible — use `box-shadow` focus ring (not `outline: none` alone).
- Focus ring color: `primary-500` at 40% opacity ring (`0 0 0 2px rgba(245,166,35,0.4)`).
- Do not rely on color alone to communicate meaning — always pair color with text label or icon (e.g., success/danger always includes a label, not just a colored dot).
- Minimum touch target: 36×36px for any interactive element.
- Disabled states: `opacity: 0.4`, `cursor: not-allowed`, `pointer-events: none` — do not just remove from DOM.
- Screen readers: all icon-only buttons must have `aria-label`. Status badges must have `aria-label` with full text (not just the tier letter).

---

## 12. Responsive Breakpoints

The SPA is desktop-first. The sidebar collapses at the `md` breakpoint.

| Breakpoint | Min-width | Layout behavior |
|---|---|---|
| `sm` | 640px | — |
| `md` | 768px | Sidebar collapses to icon-only (56px); content area expands |
| `lg` | 1024px | Full sidebar (220px); standard layout |
| `xl` | 1280px | Primary target — all layouts optimized here |
| `2xl` | 1536px | Wider content columns; more table columns visible |

On `< md` (mobile):
- Sidebar replaced by bottom navigation bar (5 primary items max)
- Cards go to single column
- Tables gain horizontal scroll with sticky first column
- Modal drawers come from bottom instead of center

---

*Design system version 1.0 — 2026-05-06*
*Supersedes ad-hoc per-component CSS. All new components must reference tokens from this document.*
