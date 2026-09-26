# Analyze existing Gaori logs

Load before summarizing a retained log or diagnosing a parser mismatch. Reuse the existing log instead of rerunning its command. For lost paths or uncertain evidence identity, read [recovery](recovery.md) first.

## Summarize without execution

Complete [command preparation](../SKILL.md#prepare-the-selected-command), including the once-per-root-task [retention advisory](retention.md), before creating new standalone evidence:

```bash
gaori --json --repo <target-root> summarize --parser go-test --tag go --tag unit path/to/unit.raw.log
```

`summarize` defaults to `generic`. It copies the raw log and creates derived artifacts, so it is a local mutation requiring authority for that effect. It has no authoritative child-process result; its status is inferred from the log. Preserve the distinction from an executed command's exit code and follow [result reporting](../SKILL.md#read-and-report-the-result). A fixed run ID and command ID can replace prior artifacts; read [lifecycle](lifecycle.md#run-start-and-replacement) before using them.

## Diagnose a parser mismatch

When `extractor_status` is `no_match` or `degraded` and the selected parser may be wrong, inspect candidates without executing or summarizing:

```bash
gaori --json --repo <target-root> parsers detect <raw-log>
```

Detection reads only that log, loads no config, creates nothing, and shows no log content. Several labels may match. Its order is not a recommendation, and Gaori never chooses the label. Read [authoring](authoring.md#built-in-configuration) for the available labels and support tiers before selecting one; `gaori --json parsers list` and `gaori --json parsers catalog` describe the installed binary.

State why the chosen label fits, then summarize explicitly within the existing authority. Never chain detection into summarization automatically. Experimental or incomplete extraction may need manual confirmation, but it cannot change the original command result.
