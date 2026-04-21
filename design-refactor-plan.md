# Design Refactor Plan

## Intent

Translate the Codex-style calm workspace into `better-search` without turning it into a chat clone. The product should feel empty and centered before intent exists, then become a fast search workspace once the user starts typing.

## Visual Thesis

A restrained search canvas with one dominant object: the search bar. Idle state should feel almost poster-like, with the search control centered in the viewport, minimal supporting chrome, and secondary actions reduced to compact triggers.

## Interaction Thesis

1. Idle state stays centered and quiet.
2. As soon as the user types, the search area transitions upward into a working position.
3. Filters stay collapsed by default and expand only on demand, so the UI keeps a low visual footprint.

## Current UI Baseline

The current structure lives almost entirely in:

- `frontend/src/main.js`
- `frontend/src/style.css`

Today the interface is always in "work mode":

- the query bar, shortcuts, and full filter bar are always visible
- results only render as a vertical list
- the layout does not distinguish between idle and active search states

## Target Experience

### 1. Two clear layout states

#### Idle state

- Search block is vertically centered inside the main surface.
- Only the primary search field is visually dominant.
- Small supporting controls can sit around it as compact pills or icon buttons.
- Results area is visually suppressed or replaced by a soft empty-state message.

#### Active search state

- Triggered when the query has content.
- Search block animates upward into a top workspace position.
- Results container fades/slides in below.
- The workspace becomes denser, but keeps the same visual language.

This should be a layout state change, not a full screen swap.

### 2. Compact filters that expand on click

Replace the always-open filter row with collapsed filter triggers:

- `Scope`
- `Type`
- `Path`
- `Extension`

Behavior:

- Each trigger looks like a small chip/button in the idle and active states.
- Clicking a trigger opens an inline popover, dropdown, or expanding panel.
- Only one filter panel should be open at a time.
- Active filters should show state in the trigger itself.
  Example: `Type: Files`, `Ext: .pdf`, `Path: src`.
- `Clear filters` should only appear when filters are active, and should stay secondary.

Recommendation:

- Keep drive/scope as a dropdown-style chooser.
- Keep type as a segmented mini-panel.
- Keep path and extension as compact inline text inputs that open from chips.

### 3. Results view modes

Keep the current list view and add a mosaic view.

#### List view

- Best for dense scanning and keyboard navigation.
- Preserves full path visibility and action affordances.
- Remains the default for power usage.

#### Mosaic view

- Best for broader browsing and visual chunking.
- Use responsive cards/tiles in a grid.
- Each tile should show:
  - item icon
  - name
  - shortened path
  - lightweight metadata
- Tile actions should stay minimal until hover/focus.

Mosaic is not a gallery. It should still feel like a search tool, not a file browser.

### 4. View toggle placement

Add a small view switch near the result header in active state only:

- `List`
- `Mosaic`

This control should not appear in the idle state, because it has no meaning before results exist.

## Motion Plan

Use motion sparingly and only where it sharpens the experience:

- Search block transitions from centered to top-aligned with a smooth translate + scale adjustment.
- Results fade and lift in after the layout shift begins.
- Filter chips animate open/closed with short height/opacity transitions.
- View switching between list and mosaic should use a quick crossfade or shared container transition, not a flashy animation.

Target feel:

- calm
- fast
- deliberate
- not decorative

## Layout Proposal

### Idle composition

- Main surface vertically centers a `search-stage`.
- Search input is the hero element.
- Supporting controls sit directly below or around the input in a compact secondary row:
  - rescan
  - filter triggers
- Keyboard shortcut help should be reduced or tucked away so the first impression stays clean.

### Active composition

- `search-stage` shifts toward the top with tighter vertical spacing.
- Results header appears under the search area.
- Results viewport fills the remaining height.
- Controls relevant only during results browsing appear here:
  - result count
  - active filter summary
  - list/mosaic toggle

## Suggested Structural Refactor

The current render path is simple enough that this can stay in plain JS for now. No framework change is required for this refactor.

### Main DOM sections to introduce

- `search-stage`
- `search-stage__topline`
- `search-stage__filters`
- `results-toolbar`
- `results--list`
- `results--mosaic`

### State to add

Extend the existing `state` object with:

- `viewMode: 'list' | 'mosaic'`
- `openFilter: null | 'scope' | 'type' | 'path' | 'extension'`
- derived layout state based on `query.length > 0`

### Render responsibilities

- `renderSearchStage()`
  Renders the centered vs active search container and compact filter triggers.
- `renderFilterPanels()`
  Controls which filter panel is expanded.
- `renderResultsToolbar()`
  Shows result count, active filter summary, and view toggle.
- `renderListResults()`
  Keeps the current row-based layout.
- `renderMosaicResults()`
  Adds the card/grid renderer.

## CSS Refactor Direction

Move away from the current always-on stacked layout and introduce stateful classes on the root surface.

Suggested state classes:

- `.surface.is-idle`
- `.surface.is-active`
- `.results.is-list`
- `.results.is-mosaic`
- `.filter-chip.is-open`
- `.filter-chip.is-active`

### Styling principles

- Reduce persistent borders and boxes.
- Keep one strong surface around the search input, not around every sub-element.
- Use spacing and typography before extra card treatments.
- Let mosaic tiles feel lighter than dashboard cards.

## Result Card Direction For Mosaic

Each tile should prioritize scanning order:

1. icon/type cue
2. file or folder name
3. path
4. meta

Interaction:

- Whole tile selectable for keyboard state
- primary click opens
- secondary actions reveal on hover/focus

Responsive behavior:

- 3 to 4 columns on wide desktop
- 2 columns on medium widths
- 1 column on small screens

## Keyboard And Behavior Notes

Preserve existing keyboard strengths:

- `/` focuses search
- arrows move selection
- `Enter` opens selected item

Additional requirement for mosaic:

- keyboard selection must remain visible and deterministic
- arrow navigation should still feel linear unless a true grid-navigation model is implemented deliberately

Recommendation:

- Keep selection order linear even in mosaic view for the first pass
- visually highlight the selected tile/card

## Phased Implementation Plan

### Phase 1. Layout state split

- Add idle vs active root classes
- Center search in idle state
- Move search area upward when query exists
- Keep existing list results renderer working

### Phase 2. Filter compression

- Replace full filter row with compact triggers
- Implement expandable filter panels
- Add active filter summaries inside the chips

### Phase 3. Results toolbar

- Add result count + filter summary bar
- Add list/mosaic view toggle

### Phase 4. Mosaic renderer

- Implement second renderer for results
- Add tile states for hover, active, and keyboard selection

### Phase 5. Polish

- tune transitions
- reduce visual noise in empty state
- review mobile behavior for collapsed filters and mosaic density

## Risks To Watch

- If the upward transition is too large, it may feel like a jarring mode switch.
- If filter expansion is too hidden, discoverability will drop.
- If mosaic cards carry too much metadata, they will become heavy and defeat the clean layout goal.
- If keyboard behavior differs too much between list and mosaic, the product will feel inconsistent.

## Definition Of Done

The refactor is successful when:

- the default screen feels centered, calm, and intentionally sparse
- typing immediately transforms the surface into a search workspace
- filters stay available without dominating the layout
- users can switch between dense list scanning and lighter mosaic browsing
- the product feels more like a focused search tool and less like a static form stacked above results
