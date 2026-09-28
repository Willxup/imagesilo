# ImageSilo — Prism workspace redesign

## Intent

Rebuild the experience as a calm, image-first creative workspace. Preserve every existing operation and route. The existing blue/cyan/green folded logo is the brand: overlapping planes, careful depth, and precise movement provide the visual language. This is a structural redesign, not a color override of the old admin template.

## Art direction

- Light: pale mineral canvas (#f3f5f6), white working surfaces, near-black blue-gray text (#19252d), secondary text with accessible contrast. Dark: charcoal/navy canvas (#10171c), raised #192329 surfaces; avoid a saturated blue wash.
- Blue (#2563eb) is the main action; cyan and green are small accents in brand artwork, progress, active markers, and success. Do not put gradients on every button/card.
- Typography uses the installed system sans stack including Chinese fonts. Page titles 30–34px desktop / 26px mobile; body 14px; useful metadata 12px. No remote fonts.
- Consistent 8px spacing rhythm; content gaps 24–32px; controls 40px, mobile essential targets >=44px; panels 18–24px radius, controls 10–12px. Fine borders and small shadows establish hierarchy, not many nested boxes.
- Artwork derives from three overlapping tilted rounded planes in logo colors. Implement with CSS or small SVG, not a stock photo or new raster asset. Decorative artwork must be hidden from accessibility.

## Workspace shell

- Replace the TailAdmin composition with a compact 224px desktop navigation column and an inset content canvas. Keep branding top-left, group library/upload/migrations/aliases vs API/settings/system clearly. All routes stay directly accessible.
- Active navigation uses a quiet pill and a brand-colored marker. Keep a deliberate desktop collapse control, but remove hover-driven sidebar expansion and accidental layout shifts.
- Top bar is slim contextual navigation plus a compact global search affordance, theme, language, and account controls. Search remains available through a visible control and Cmd/Ctrl+K. Avoid displaying the same large search field in header and library.
- Mobile uses a top bar and a properly accessible drawer, with Escape, focus containment, background inertness, focus return, and no invisible focusable navigation. Navigation closes after selecting a route.
- Keep repository/logo link, version/license footer, account menu/logout, themes and language persistence.

## Page composition

### Upload

Create a composed upload studio: page heading with a short localized eyebrow, a large upload stage with folded-plane artwork and distinct drag-active state, and a compact options side panel on wide screens (stacked on mobile). Keep upload count/visibility/clear in obvious positions. File constraints are supporting text, not developer-facing hero metrics. Avoid a second giant empty-state box below an already empty dropzone.

Queue rows have actual local thumbnail previews, filename/size, understandable status, progress, cancel/retry and completed link controls. Revoke object URLs. Preserve concurrency and every cancellation/retry/error behavior. Progress fills interpolate smoothly, and completion changes feel continuous.

### Image library and migrations

Images take precedence: generous thumbnail grid, uncluttered metadata, subtle hover lift/image scale, visually distinct selected cards, a compact view toggle and filter affordance. Retain list mode and all filters, with dates/dimensions/sizes behind advanced disclosure. On desktop useful filters can be visible but should not dominate the first viewport; on mobile default collapsed. Preserve preference storage and URL filters.

Batch selection presents a floating action dock. All copy formats, private/public changes and confirmed deletion stay accessible. The dock and its upward menus must remain inside 375px screens without overflow clipping. Hover-revealed controls must also be available on focus and touch. Empty/loading/error states should look designed and provide a useful next action where applicable.

Migrations uses the same image language, preserving scan status, refresh, mode notice, original paths, copy/open, and mutation-gated deletion.

### Detail

Use a large image stage alongside a narrower inspector: filename, core dimensions/format/size, visibility and copy/open/delete actions are grouped. Metadata and aliases belong in clearly separated secondary sections. Preserve return-to-filtered-library behavior and all link formats. Mobile stacks image then actions then inspector. Do not crop the full preview.

### Settings, aliases, API tokens, system

Give each page a shared heading language and a purposeful layout. Settings should use clearly named sections (profile, defaults, processing, security), with compact local section navigation if useful, while keeping every form and independent save boundary. Alias creation/resolution and existing mappings are distinct. Tokens split creation from existing token management, with the one-time secret conspicuous and all scopes/expiry/revocation preserved. System metrics become a calm compact overview, with maintenance/inspection/index rebuilding still available and confirmed as before. Do not invent statistics or decorative charts from nonexistent data.

### Login and setup

Desktop login is an editorial split layout: brand composition on one side and a focused form on the other. Use short bilingual product copy, oversized folded-plane artwork, and small restrained color accents. Mobile prioritizes the form with a compact brand area, no forced screen overflow. Preserve language/theme, error feedback, autofill, redirect, and footer. Setup uses the same language with a readable form; retain every setup field and option.

## Motion choreography

Motion must communicate state and spatial continuity, not delay the task.

- Shared tokens: fast 140ms, normal 220ms, reveal 340ms; easing cubic-bezier(.22,1,.36,1). Primarily transform/opacity. Avoid transition: all and continuous expensive blur/shadow animations.
- Navigation: active marker/pill transition and modest incoming content translateY(8px)+fade. Use browser view transitions for route changes if implemented safely, with clean fallback and no double entrance. Never remount upload queues on theme/search toggles.
- Initial content: short stagger capped around 180ms total for cards, not a delay proportional to a huge collection. Images load lazily with stable aspect ratios.
- Cards lift 2px / image scale ~1.025 on pointer hover; buttons press scale .98; focus ring visible and never dependent on hover.
- Drawer, dialogs, menus and filter disclosures animate open AND close. Preserve focus management through exit. Menus must not be clipped by animated ancestors.
- Upload drag highlights and plane spread settle naturally; completion/progress, batch dock, and toast transitions are coordinated.
- Respect prefers-reduced-motion comprehensively: no transform flights/stagger/loop; controls remain immediate. Essential progress still updates. No infinite decorative animation by default.

## Implementation and preservation requirements

- Reuse React, existing data/query/auth code and accessible primitives where sensible. No backend or new animation dependency is needed unless a concrete requirement warrants it.
- Replace/reorganize the existing 3106-line stylesheet deliberately; do not append an enormous override layer. Remove obsolete shell styles when replacing their markup. Maintain all component styles actually in use.
- All new user-facing copy belongs in both zh-CN and en-US. Do not expose design codename or technical architecture in product copy.
- Preserve routes, API contract, visibility semantics, upload processing, CSRF, settings data, persisted preferences, migration safety and destructive confirmations.
- Read the current worktree and tests. Update obsolete layout-dependent test selectors where necessary; do not weaken user-flow assertions to hide regressions.

## Acceptance

1. Visually inspect real rendered login/setup/upload/gallery/detail/migrations/aliases/tokens/settings/system; include populated images and empty/error states where practical.
2. Check light and dark, Chinese and English, 375px mobile, 768px tablet, 1024px compact desktop and 1440px desktop. No horizontal overflow, clipped menus or inaccessible actions.
3. Verify keyboard navigation, modal/drawer focus return and Escape, visible focus, readable contrast, and reduced motion.
4. Verify upload (file/drop/paste, progress/cancel/retry, multi-copy), filters/list/grid/selection/copy/visibility/delete, aliases, tokens, settings, system, auth flows with existing tests plus focused coverage for new interactions.
5. Run project `make check e2e` and inspect results. Review final diff for lost features and unnecessary complexity.
6. Primary agent independently inspects screenshots and runtime; developer handoff alone does not establish acceptance.

Worktree: `/Users/will/Projects/imagesilo/.worktrees/prism-ui`, branch `worktree/prism-ui`. No commit, push, PR, release or deployment is requested.


## Approved console refinement — 2026-09-30

The user selected a floating bottom console, preserving the existing page and card style, and chose the original blue/cyan/green gradient for primary buttons and active navigation. This supersedes the earlier sidebar and solid-blue shell direction.

- Desktop (1024px and wider): seven labeled destinations in a fixed bottom console, with a translating active background; no sidebar or separate full-width header bar.
- Compact screens: upload and library remain directly accessible; the third button opens a labeled navigation sheet and shows the current destination for other pages. The sheet uses the existing Modal focus trap, Escape handling and focus restoration.
- Content and the batch toolbar reserve space above the console including the bottom safe-area inset. All existing routes and workflows remain intact.
- Original gradient `#2563eb → #08a7c5 → #12b76a` is restored for default primary actions. Explicit solid, neutral and destructive variants remain distinct. No continuously animated gradient.
- Global tools remain available in a compact top-right capsule. Navigation respects reduced-motion preferences and browser history.

Validation: `make check` succeeded (Go checks, frontend build/lint/typecheck, 35 unit tests). The 13 E2E scenarios passed across the full run and focused reruns after updating two old sidebar-era locators to target the navigation dialog. Coverage includes 375/768/1024/1440px route matrices, 320px compact navigation, 844×390 landscape, dark mode, English, focus restoration, batch menu placement, and interrupted navigation. Real iOS Safari and Android devices were not tested.


## 控件与动效精修最终验收 — 2026-09-30

保留底部控制台和原蓝青绿渐变 B，统一次级控件、复制原位反馈、图片异步淡入、上传进度动画、加载骨架、浮层层级、短屏菜单与日历。手机主要操作触摸区统一 44px，并修复英文标题挤压与导航标签截断。深色控件和上传区已按实际截图精修。

最终 `make check e2e` 退出码 0：39 项单元测试、17 项正式浏览器场景，Go 检查与 lint/typecheck 全部通过。另一次实图截图测试通过。4 倍 CPU 降速连续切页诊断 p95 约 16.8ms，属于本机短时模拟结果；未做真机 Safari 验证。详情和证据见 `polish-audit.md`；原始帧间隔见 `frame-pacing-4x-cpu.json`。

最终图片位于任务截图目录的 `polished-*.png`，包括 PC/手机图库、详情、批量操作、深色与手机导航。没有新增运行依赖、后端改动、提交、推送或部署。
