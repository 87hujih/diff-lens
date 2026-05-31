# Product

## Register

product

## Users

diff-lens serves developers and code reviewers who need to understand a GitHub pull request quickly before leaving review feedback. They are usually in a local development or demo environment, comparing PR metadata, diff-derived evidence, rules findings, LLM analysis status, and suggested review comments in one task-focused surface.

## Product Purpose

diff-lens is a local web AI Pull Request Review assistant. A reviewer enters a GitHub PR URL, optionally supplies a GitHub token for API access, and receives a streamed analysis that combines GitHub PR data, deterministic rule scanning, controlled LLM context analysis, a normalized review report, evidence, and copyable review suggestions.

The product does not replace human reviewers. It helps them enter PR context faster, spot higher-risk changes earlier, preserve degraded rules-based output when LLM analysis is unavailable, and copy candidate comments into GitHub after human judgment.

## Brand Personality

Precise, restrained, developer-native. The interface should feel like an operations console for code review: dense enough for repeated use, calm under failure states, and explicit about evidence and uncertainty.

## Anti-references

This should not look like a SaaS marketing landing page, AI chat toy, glossy hero page, or decorative analytics dashboard. Avoid oversized cards, vague gradient decoration, weak gray text, ornamental motion, fake certainty, and any UI that hides evidence behind visual flourish.

## Design Principles

1. Lead with the task. The first screen should be the working review console, not a product pitch.
2. Preserve trust through provenance. Risks, suggestions, degraded states, and model traces must explain where they came from.
3. Keep density readable. Use compact layouts, but preserve enough spacing and hierarchy for scanning long file names, evidence, and comments.
4. Make uncertainty visible. Missing LLM output, truncated context, absent evidence, and unknown severity should stay usable and plainly labeled.
5. Optimize for handoff. Copy actions, filters, and evidence inspection should support a reviewer moving findings into GitHub without changing product scope.

## Accessibility & Inclusion

Target WCAG AA contrast for text and interactive states. Keyboard focus must be visible, controls need semantic labels, touch targets should remain at least 44px, reduced motion preferences must be respected, and long technical strings must wrap without horizontal page overflow.
