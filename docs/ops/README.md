# Gaori Operations

Status: No independently operated Gaori environment

Gaori is a standalone local CLI with an attached, session-local STDIO MCP adapter. This delivery scope owns no deployed service, daemon, hosted control plane, production environment, durable job ledger, or independently operated runtime, so there is currently no Gaori operations runbook to invent or maintain.

The public [README](../../README.md) owns installation and direct-user troubleshooting. The [user interface](../user-interface.md) and [integration guide](../integration-guide.md) own CLI operation and parent-project integration. The [implementation tips](../implementation-tips/README.md) own development, testing, and release-engineering guidance. Each parent project owns the real environment in which it invokes Gaori.

Add a runbook here only after repository evidence establishes an independently operated target and owner. Every future runbook must state its environment, prerequisites and authority, read-only diagnosis, bounded resolution, success checks, rollback or recovery, and escalation owner without recording credentials or live secret values.
