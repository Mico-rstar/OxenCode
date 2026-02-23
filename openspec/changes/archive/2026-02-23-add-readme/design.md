# Design: Add README for OxenCode

## Context

OxenCode is an innovative AI programming assistant built with Go, featuring a unique architecture with agent/tool environment separation and an innovative context management system. The project currently has comprehensive technical documentation in the `docs/` directory but lacks a project-level README that serves as the entry point for new users and contributors.

**Current State:**
- Empty `README.md` file exists at project root
- Rich technical documentation exists in `docs/` directory (architecture.md, react-loop.md, tool-integration.md, etc.)
- Configuration example file exists at `config.example.toml`
- Project is active with recent commits and features

**Constraints:**
- No code changes allowed - documentation only
- Must support both English and Chinese audiences
- Must accurately represent the project's current state and capabilities
- Should be maintainable as the project evolves

**Stakeholders:**
- New users trying to understand and use OxenCode
- Potential contributors looking to participate in development
- Project maintainers who need to maintain the documentation

## Goals / Non-Goals

**Goals:**
- Create comprehensive, professional README documentation that serves as the project's landing page
- Support both English (primary) and Chinese audiences with separate, fully-localized files
- Accurately communicate OxenCode's unique architectural advantages
- Provide clear quick-start guidance for new users
- Link to existing detailed documentation where appropriate

**Non-Goals:**
- No changes to code or build system
- No new infrastructure or deployment requirements
- No changes to existing documentation structure (docs/ directory remains as-is)
- Not creating API reference or detailed technical guides (those exist in docs/)
- Not implementing automated translation or localization system (manual translation is acceptable)

## Decisions

### Decision 1: Dual README Files (README.md + README.zh-CN.md)

**Choice:** Create two separate README files rather than a single file with language switching.

**Rationale:**
- GitHub and code hosting platforms automatically display `README.md` as the repository's landing page
- Chinese users can easily access `README.zh-CN.md` via direct link or language switcher
- Separation allows each version to be properly formatted and optimized for its audience
- Common pattern in bilingual projects (e.g., many Chinese open-source projects)
- No JavaScript or complex navigation needed

**Alternatives Considered:**
- Single README with sections for both languages - Rejected because it makes the file too long and harder to navigate
- Using i18n/L10n frameworks - Rejected as overkill for a simple documentation change
- Hosting documentation on separate site - Rejected due to additional infrastructure requirements

### Decision 2: Markdown with GitHub-Flavored Extensions

**Choice:** Use standard Markdown with GitHub-Flavored Markdown (GFM) extensions.

**Rationale:**
- Native GitHub rendering without additional tooling
- Supports tables, task lists, strikethrough, and code blocks
- Widely understood and easy to edit
- Works well with GitHub's code highlighting and linking
- No build step required

**Alternatives Considered:**
- AsciiDoc - Rejected due to requiring build tooling
- Custom documentation framework (Docusaurus, MkDocs) - Rejected as overkill for README-only documentation

### Decision 3: README Structure and Sections

**Choice:** Follow standard open-source README conventions with OxenCode-specific customizations.

**Structure:**
1. Logo/Banner (image or ASCII art)
2. Project tagline and brief description
3. Key features (with emphasis on architectural advantages)
4. Quick Start (minimal working example)
5. Installation (prerequisites, build, configure)
6. Configuration (reference to config.example.toml)
7. Usage (common patterns)
8. Documentation links (to docs/ directory)
9. Contributing guidelines
10. License

**Rationale:**
- Follows established patterns from successful open-source projects
- Provides progressive disclosure (high-level → detailed)
- Balances marketing (features) with practical information (installation, usage)
- Links to existing detailed docs rather than duplicating content

**Alternatives Considered:**
- Minimal README with just installation - Rejected, doesn't showcase project's unique advantages
- Comprehensive documentation in README - Rejected, would duplicate docs/ directory content

### Decision 4: Feature Emphasis

**Choice:** Highlight three key architectural differentiators:

1. **Agent/Tool Environment Separation**: Agent runs in isolated environment from tools
2. **Long-Running Task Support**: Can handle extended tasks without context overflow
3. **Innovative Context Management**: Multi-batch, user-transparent, asynchronous context compression (vs. traditional full-context compression)

**Rationale:**
- These are OxenCode's core innovations vs. other AI coding assistants
- Directly addresses user pain points (context limits, long-running tasks)
- Based on actual implementation (docs/context-manage-idea.md, docs/architecture.md)
- Technical but understandable to target audience

**Alternatives Considered:**
- Focus on tool capabilities (Glob, Grep, Bash, etc.) - Rejected, these are commodity features
- Focus on TUI/UX - Rejected, nice-to-have but not core differentiation
- Generic feature list - Rejected, doesn't communicate unique value proposition

### Decision 5: Translation Approach

**Choice:** Manual translation with technical accuracy prioritized over literal translation.

**Rationale:**
- Technical terms should remain in English where appropriate (e.g., "Agent", "ReAct Loop", "TUI")
- Chinese version should read naturally to Chinese developers, not like machine translation
- Maintain consistency between versions (same sections, same depth)
- Project maintainer is native Chinese speaker, ensuring quality

**Quality Standards:**
- Technical accuracy preserved
- Natural phrasing in each language
- Consistent terminology across both versions
- Code examples identical (only text translated)

## Risks / Trade-offs

### Risk 1: Documentation Drift

**Risk:** README becomes outdated as project evolves.

**Mitigation:**
- Keep README focused on high-level overview and stable features
- Link to detailed documentation for implementation details
- Add README update checklist to release process
- Use examples that are likely to remain stable

### Risk 2: Translation Inconsistency

**Risk:** Chinese and English versions diverge over time.

**Mitigation:**
- Maintain identical section structure in both files
- Add comment in both files reminding to update both versions
- Consider adding translation sync check to CI (future enhancement)

### Risk 3: Over-Promising Features

**Risk:** README describes features that are planned but not fully implemented.

**Mitigation:**
- Base features section on actual implemented functionality
- Reference existing documentation (docs/react-loop.md, docs/architecture.md)
- Avoid mentioning roadmap items as current features
- Use "Planned" or "Roadmap" section for future work if needed

### Trade-off: Detail Level

**Trade-off:** README could include more technical details, but that would duplicate docs/.

**Decision:** Keep README high-level, link to docs/ for details. This reduces maintenance burden and keeps README focused on onboarding.

## Migration Plan

**Deployment Steps:**

1. Create `README.md` with English content
2. Create `README.zh-CN.md` with Chinese content
3. Verify both files render correctly on GitHub
4. Update any references to point to README (if needed)
5. Test with users from both language communities

**Rollback Strategy:**

- Simple file deletion if needed
- No code changes, so zero technical risk
- Git history preserves previous versions

**Open Questions:**

None - this is a straightforward documentation change with no technical unknowns.

## Future Considerations

**Out of Scope for This Change:**

- Automated screenshot/screencast generation
- Interactive tutorial or playground
- API documentation generation
- Contributing translation workflow
- Localization beyond README (e.g., code comments, error messages)

**Potential Future Enhancements:**

- Add badges (build status, coverage, version)
- Add project logo or mascot
- Create architecture diagram for README
- Add video demo link
- Implement bilingual navigation in README footer
