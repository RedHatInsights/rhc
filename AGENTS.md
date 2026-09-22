# Agent Guidelines

Read [CONTRIBUTING.md](CONTRIBUTING.md) first — it is the authoritative source
for commit format, PR workflow, and code guidelines. The sections below add
agent-specific context.

## Contribution Rules

Follow all rules in Code Guidelines in [CONTRIBUTING.md](CONTRIBUTING.md#code-guidelines).

## Agent Boundaries
- Do not rewrite existing commits or create fixup/temporary commits unless explicitly
  requested.
- Keep changes scoped to the requested task and avoid unrelated modifications.
- Do not claim to have performed actions (such as creating Jira tickets or getting
  approvals) that are outside the agent's capabilities.
- Do not modify `rhc.spec`. If a spec change is required, inform the human author that
  a Jira card for the owning team is required rather than attempting to create one.

## Code Ownership

Ownership is enforced via [CODEOWNERS](.github/CODEOWNERS).
