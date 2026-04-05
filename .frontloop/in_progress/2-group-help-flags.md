---
title: Group help flags into categories
priority: high
---

## Goal

Group the 40+ flags in `juggle --help` into logical categories so users can scan for what they need instead of reading a flat wall of flags.

## Acceptance Criteria

- Flags are grouped under clear headings (e.g., Lifecycle Hooks, Limits & Stopping, Agent Configuration, Output, Watch Mode)
- Grouping is implemented using cobra's flag group annotations or custom help template
- `juggle --help` output reads cleanly with visual separation between groups
- No flags are lost or duplicated
