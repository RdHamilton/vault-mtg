# Brand Design: Ray Hamilton Engineering

## Brand Identity

**Tagline**: Engineering products that work — and teams that ship.

**Voice**: Precise, confident, approachable

**Aesthetic**: Clean and typographically driven — the opposite of VaultMTG's dark, neon-accented gaming world. Ray Hamilton Engineering lives in daylight: generous white space, slate-and-indigo on near-white surfaces, no decorative chrome. Where VaultMTG earns attention through visual drama, this site earns trust through restraint. It reads like a well-crafted proposal document, not a product landing page.

---

## Color Palette

### Primary Brand Color

| Name | Hex | Tailwind Token | Usage |
|---|---|---|---|
| Indigo | `#4F46E5` | `brand-500` | Primary CTAs, links, highlights |
| Indigo Light | `#6366F1` | `brand-400` | Hover states |
| Indigo Dark | `#3730A3` | `brand-600` | Pressed states, active nav |

Rationale: Indigo sits at 6.0:1 contrast on `surface-base` — WCAG AA on body copy. It reads as technical and authoritative without the cold aggression of pure blue. Used by Linear, Stripe (accent), and Raycast — the toolset of high-craft engineering teams.

### Semantic Colors

| Token | Hex | Usage |
|---|---|---|
| `success` | `#047857` | Positive states, availability badges |
| `muted` | `#64748B` | Secondary text, borders, placeholder |

### Surface Colors (light theme)

| Token | Hex | Usage |
|---|---|---|
| `surface-base` | `#F8FAFC` | Page background (Slate 50 — warm near-white, not glaring) |
| `surface-raised` | `#FFFFFF` | Card backgrounds, form inputs |
| `surface-overlay` | `#F1F5F9` | Sidebar panels, code blocks, subtle sections |
| `surface-border` | `#E2E8F0` | Dividers, card borders, input outlines |

### Text Colors

| Token | Hex | Contrast on surface-base | Usage |
|---|---|---|---|
| `text-primary` | `#1E293B` | 14.0:1 | Headings, primary body copy |
| `text-secondary` | `#475569` | 7.2:1 | Supporting text, subheadings |
| `text-muted` | `#64748B` | 4.5:1 | Metadata, labels, placeholders |

All text ≥4.5:1 contrast — WCAG AA compliant.

---

## Typography

### Font Stack

- **Display/Headings**: [Plus Jakarta Sans](https://fonts.google.com/specimen/Plus+Jakarta+Sans) — Geometric humanist with warm curves. At heavy weights (700–800) it's distinctive without being eccentric. Signals craft without shouting.
- **Body**: [Inter](https://fonts.google.com/specimen/Inter) — Purpose-built for screen readability, near-universal on high-quality software products. Pairs cleanly with Plus Jakarta Sans.

Note: Two families is a deliberate choice here. Plus Jakarta Sans at display weights gives the site a recognizable voice; Inter body text disappears into readability — the reader focuses on content, not letterforms.

### Type Scale

| Token | Size | Weight | Line Height | Usage |
|---|---|---|---|---|
| `text-sm` | 14px | 400 | 1.5 | Metadata, tags, legal copy |
| `text-base` | 16px | 400 | 1.6 | Body paragraphs |
| `text-lg` | 18px | 500 | 1.4 | Lead paragraph, card descriptions |
| `text-xl` | 24px | 600 | 1.3 | Section headers, card titles |
| `text-2xl` | 32px | 700 | 1.2 | Page headings |
| `text-3xl` | 48px | 800 | 1.1 | Hero heading |

---

## Spacing & Shape

- **Base unit**: 4px
- **Border radius**:
  - `sm`: 4px — tags, badges, small inputs
  - `default`: 8px — cards, buttons, standard inputs
  - `lg`: 16px — feature panels, hero containers
  - `full`: 9999px — pill tags, avatar frames
- **Spacing philosophy**: Err toward generous. 48–64px section gutters, 24–32px card padding. White space signals confidence — a cluttered layout suggests uncertainty. This is the primary stylistic contrast with VaultMTG's dense, information-packed dark UI.
- **Shadows**: Subtle only. `shadow-sm` (0 1px 2px rgba(0,0,0,0.05)) for raised cards. No dramatic drop shadows — those belong in gaming UIs.

---

## Tailwind Config Extension

```js
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        brand: {
          400: '#6366F1',
          500: '#4F46E5',
          600: '#3730A3',
        },
        surface: {
          base:    '#F8FAFC',
          raised:  '#FFFFFF',
          overlay: '#F1F5F9',
          border:  '#E2E8F0',
        },
        content: {
          primary:   '#1E293B',
          secondary: '#475569',
          muted:     '#64748B',
        },
        success: '#047857',
      },
      fontFamily: {
        display: ['"Plus Jakarta Sans"', 'sans-serif'],
        body:    ['"Inter"', 'sans-serif'],
      },
      fontSize: {
        'display-hero': ['48px', { lineHeight: '1.1', fontWeight: '800' }],
        'display-h1':   ['32px', { lineHeight: '1.2', fontWeight: '700' }],
        'display-h2':   ['24px', { lineHeight: '1.3', fontWeight: '600' }],
      },
      spacing: {
        '18': '72px',
        '22': '88px',
      },
    },
  },
}
```

---

## CSS Custom Properties

```css
:root {
  /* Brand */
  --color-brand:        #4F46E5;
  --color-brand-light:  #6366F1;
  --color-brand-dark:   #3730A3;

  /* Surfaces */
  --surface-base:       #F8FAFC;
  --surface-raised:     #FFFFFF;
  --surface-overlay:    #F1F5F9;
  --surface-border:     #E2E8F0;

  /* Text */
  --text-primary:       #1E293B;
  --text-secondary:     #475569;
  --text-muted:         #64748B;

  /* Semantic */
  --color-success:      #047857;

  /* Typography */
  --font-display: "Plus Jakarta Sans", sans-serif;
  --font-body:    "Inter", sans-serif;

  /* Spacing base */
  --space-unit: 4px;

  /* Border radius */
  --radius-sm:   4px;
  --radius-md:   8px;
  --radius-lg:   16px;
  --radius-full: 9999px;
}
```

---

## Component Specs

### 1. Hero Section

```
┌─────────────────────────────────────────────────────────────────┐
│  [nav: logo left · links right]                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌───────────────────────────┐   ┌──────────────────────┐     │
│   │                           │   │                      │     │
│   │  Ray Hamilton             │   │  [Photo or          │     │
│   │  Engineering              │   │   geometric art     │     │
│   │  ─────────────────        │   │   placeholder]      │     │
│   │  Software engineering     │   │                      │     │
│   │  for startups that        │   │  400×400 circle      │     │
│   │  need things done right.  │   │  bg: surface-overlay │     │
│   │                           │   └──────────────────────┘     │
│   │  [Work with me →]         │                                 │
│   │  [View projects]          │                                 │
│   │                           │                                 │
│   └───────────────────────────┘                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Desktop**: Two-column, left text / right image, max-width 1120px, centered. 120px vertical padding.

**Heading hierarchy**:
- Eyebrow line: `text-sm` uppercase tracking-widest, `text-muted`, font-body — e.g. "Software Engineer & Founder"
- H1: `font-display text-3xl` (48px / 800) `text-primary` — 2–3 word name line
- Subhead: `font-body text-lg` (18px / 500) `text-secondary` — one compelling sentence

**CTA buttons**:
- Primary: `bg-brand-500 text-white px-6 py-3 rounded-md` font-body 500 — "Work with me"
- Secondary: `border border-surface-border text-primary px-6 py-3 rounded-md` — "View projects"
- Hover: primary darkens to `brand-600`; secondary gets `surface-overlay` background

**Mobile** (collapsed, <768px): Single column. Image moves below heading or hides. Buttons stack full-width. Padding reduces to 64px vertical.

---

### 2. Project Card

```
┌──────────────────────────────────────────┐
│  [Project name]                   [↗]   │
│  font-display text-xl 600 text-primary   │
│                                          │
│  One-line description of what it does    │
│  font-body text-base text-secondary      │
│                                          │
│  ┌────────┐ ┌──────┐ ┌──────────┐       │
│  │  Go    │ │React │ │PostgreSQL│       │
│  └────────┘ └──────┘ └──────────┘       │
│                                          │
│  [GitHub]  [Live site →]                │
└──────────────────────────────────────────┘
```

**Card container**: `bg-surface-raised border border-surface-border rounded-lg p-6`

**Hover state**: `shadow-md` appears (0 4px 12px rgba(0,0,0,0.08)), border shifts to `brand-400` — subtle lift. CSS transition 150ms ease.

**Tech tag pills**:
- Style: `bg-surface-overlay text-content-secondary text-sm font-body px-3 py-1 rounded-full`
- No color coding — uniform gray keeps visual noise low, professional
- Font size: 13px / 400 weight

**Link treatment**:
- GitHub: icon + "Source" text, `text-muted` baseline, `text-brand-500` on hover
- Live link: icon + "Live" text, same treatment
- External link icon: Heroicon `ArrowTopRightOnSquare`, 16px

**Grid layout**: 2-col desktop (`grid-cols-2 gap-6`), 1-col mobile. Max 3 featured projects in main grid; overflow goes to list view.

---

### 3. Service / Skill Block

```
┌────────────────────────────────────────────────────────────────┐
│                                                                │
│  [Icon]  Full-Stack Product Engineering                        │
│  24px    font-display text-xl 600 text-primary                │
│          ──────────────────────────────────────               │
│          Go backends, React frontends, and the infra           │
│          to run them. From API design to production deploy.    │
│          font-body text-base text-secondary                    │
│                                                                │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  [Icon]  Technical Leadership                                  │
│          ...                                                   │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

**Layout**: 2-column desktop grid (`grid-cols-2 gap-8 max-w-4xl mx-auto`), 1-column mobile. 4 blocks max.

**Icon**: Heroicon, `text-brand-500`, 28px, sits top-left of each block. No icon background circle — flat, minimal.

**Block container**: `p-8 rounded-lg` — no border on default state. Background alternates:
- Odd blocks: `bg-surface-raised` (white)
- Even blocks: `bg-surface-overlay` (slate-100)

**Typography inside block**:
- Title: `font-display text-xl font-semibold text-primary mb-2`
- Body: `font-body text-base text-secondary leading-relaxed` — 2–3 sentences max

**Spacing**: 32px between icon and text baseline aligned. 8px gap between title and body copy.

---

## Brand Constraints

- **Light theme only** — professional daytime context; no dark mode toggle on v1
- **Generous white space** — minimum 48px between sections, 24px inside cards; space signals confidence
- **Typography does the heavy lifting** — no hero illustrations, no gradient overlays, no stock photos
- **Intentional VaultMTG contrast** — where VaultMTG uses dark surfaces, neon accents, and dense game data, this site uses daylight surfaces, a single restrained accent, and editorial spacing
- **Trustworthy over flashy** — a potential client or employer should feel they're reading a well-maintained technical document, not a marketing site
- **Indigo as the only accent** — resist adding secondary accent colors; restraint is the brand
