# Gaori Requirements Test Matrix

Status: Complete for the current checked requirements
Scope: Primary executable or documentary evidence for each completed requirement in `requirements-specs.md`

This matrix records the primary evidence for every requirement marked complete. The audit regression suite fails when a completed requirement is missing, duplicated, has no evidence, cites a `Test*` identifier that does not resolve to a repository Go test, or references an unknown or incomplete requirement. Every row must cite at least one resolvable test unless its requirement ID is an explicit non-test evidence exception in the audit regression test.

| Requirement | Primary evidence |
|---|---|
| `GAORI-REQ-RQCLI-001` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; `TestMakeInstallTargetsAndResolver` |
| `GAORI-REQ-RQCLI-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `GAORI-REQ-RQCLI-003` | `TestAdHocRunWithoutConfig`; `TestBinaryTagsSelectRulesByAllTags`; `TestBinaryTagInterfacesFailBeforeExecution`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQCLI-004` | `TestSummarizeRawLogUsesConfigRedaction`; `TestRunAndSummarizeSelectRulesByAllTags`; `TestFrameworkParsersFromRawLogFile`; `TestSummarizeParserValidationPreventsArtifacts`; `TestBinaryTagsSelectRulesByAllTags`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQCLI-005` | `TestConfiguredRunAndExcerpt`; `TestExcerptRejectsUnsafeReferences` |
| `GAORI-REQ-RQCLI-006` | `TestTimeoutPreservesPartialArtifacts`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQCLI-007` | `TestFrameworkParsersFromCapturedStream`; `TestAdHocParserMissPreservesCommandResult`; `TestAdHocParserAndTagsSelectRules`; `TestAdHocParserOptionValidationPreventsExecution`; `TestAdHocChildParserArgumentIsPreserved`; `TestBinaryAdHocParserContract`; `TestBinaryAdHocParserValidationFailsBeforeExecution` |
| `GAORI-REQ-RQCLI-008` | `TestHelpSurfacesExitSuccessfully`; `TestHelpDoesNotConsumeChildArguments`; `TestHelpAfterPositionalOptionNameIsRecognized`; `TestUnknownHelpTopicFailsClosed`; `TestBinaryHelpHierarchy` |
| `GAORI-REQ-RQCLI-009` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestSummarizeRawLogUsesConfigRedaction`; `TestBinaryJSONRedactsCommandMetadata` |
| `GAORI-REQ-RQCLI-010` | `TestConfigCheckReportsSafeDeterministicMetadataWithoutArtifacts`; `TestConfigCheckFailsClosedOnInvalidStoredRule`; `TestBinaryConfigCheckIsReadOnly` |
| `GAORI-REQ-RQCLI-011` | `TestParseGlobalOptionsAcceptsFlexiblePositions`; `TestParseGlobalOptionsDoesNotTreatPositionalOptionNamesAsFlags`; `TestParseGlobalOptionsPreservesCommandValuesAndChildBoundary`; `TestParseGlobalOptionsPreservesExplicitSearchOperandBoundary`; `TestParseGlobalOptionsRejectsInvalidValues`; `TestRulesLifecycleCommands`; `TestBinaryGlobalOptionAfterPositionalOptionName`; `TestBinaryGlobalOptionsArePositionIndependentBeforeChildBoundary`; `TestBinaryRulesSearchEscapesGlobalOptionNames`; `TestBinaryRemovedOptionsFailBeforeVersionDispatch` |
| `GAORI-REQ-RQCLI-012` | `TestAdHocTimeoutSelection`; `TestAdHocParserOptionValidationPreventsExecution`; `TestAdHocChildTimeoutArgumentIsPreserved`; `TestBinaryAdHocTimeoutContract` |
| `GAORI-REQ-RQCFG-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig`; portable Git policy in `README.md` and `ADR-0011` |
| `GAORI-REQ-RQCFG-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQCFG-003` | `TestValidateAcceptsImplementedParsers`; `TestLoadCanonicalizesTags`; `TestValidateRejectsMissingAndUnsafeTags`; `TestBinaryTagsSelectRulesByAllTags` |
| `GAORI-REQ-RQCFG-004` | `TestSummarizeRawLogUsesConfigRedaction` |
| `GAORI-REQ-RQCFG-005` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `GAORI-REQ-RQCFG-006` | `TestLoadRejectsUnknownFieldsAndMultipleDocuments`; `TestBinaryRejectsUnknownConfigFields`; `TestBinaryTagInterfacesFailBeforeExecution` |
| `GAORI-REQ-RQCFG-007` | `TestConfigCheckReportsSafeDeterministicMetadataWithoutArtifacts`; `TestDisplayConfigPathKeepsExternalOverrideAbsolute`; `TestBinaryConfigCheckIsReadOnly` |
| `GAORI-REQ-RQRUN-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig` |
| `GAORI-REQ-RQRUN-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `GAORI-REQ-RQRUN-003` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `GAORI-REQ-RQRUN-004` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `GAORI-REQ-RQRUN-005` | `TestExecuteTimeout`; `TestExecuteTimeoutReportsRawLogWriteFailure`; `TestTimeoutPreservesPartialArtifacts` |
| `GAORI-REQ-RQRUN-006` | `TestExecuteForwardsTerminationAndNormalizesResult`; `TestExecuteInterruptedReportsRawLogWriteFailure`; `TestBinaryPreservesInterruptedEvidence` |
| `GAORI-REQ-RQRUN-007` | `TestAdHocTimeoutSelection`; `TestBinaryAdHocTimeoutContract`; `TestTimeoutPreservesPartialArtifacts` |
| `GAORI-REQ-RQART-001` | `TestRunIDArtifactLayout`; `TestBinaryArtifactContainment` |
| `GAORI-REQ-RQART-002` | `TestArtifactOutputDirectories`; `TestBinaryStandaloneCollisionResistance` |
| `GAORI-REQ-RQART-003` | `TestConfiguredRunAndExcerpt`; `TestOversizedSummarizeUsesBoundedExtraction`; `TestNoisyRunsWriteBoundedTerminalArtifacts`; `TestWriteSummaryJSONIncludesFalseTruncationFields`; `TestArchitectureJSONContractExamplesMatchFreshRunArtifacts` |
| `GAORI-REQ-RQART-004` | `TestWriteSummaryMarkdownMatchesDocumentedShape`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQART-005` | `TestConfiguredRunAndExcerpt`; `TestBinaryJSONRedactsCommandMetadata`; `TestArchitectureJSONContractExamplesMatchFreshRunArtifacts` |
| `GAORI-REQ-RQART-006` | `TestConfiguredRunAndExcerpt`; `TestExcerptSymlinkContainment` |
| `GAORI-REQ-RQART-007` | `TestArtifactOutputDirectories`; `TestRunIDArtifactLayout` |
| `GAORI-REQ-RQCLE-001` | `TestCleanCommandContract`; `TestBinaryCleanContract` |
| `GAORI-REQ-RQCLE-002` | `TestCleanCommandContract`; `TestBinaryCleanContract` |
| `GAORI-REQ-RQCLE-003` | `TestCleanStandaloneSelectsCompletedRunsAndPreservesOtherState`; `TestCleanCutoffUsesWholeUTCDays`; `TestBinaryCleanContract` |
| `GAORI-REQ-RQCLE-004` | `TestCleanCommandContract`; `TestBinaryCleanContract`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQCLE-005` | `TestCleanStandaloneRejectsUnsafeTargetsBeforeDeletion`; `TestCleanStandaloneContainsGaoriSymlinks`; `TestBinaryCleanContract` |
| `GAORI-REQ-RQEXT-001` | `TestProcessGenericFailureProducesPreciseSpan` |
| `GAORI-REQ-RQEXT-002` | `TestValidateAcceptsImplementedParsers`; `TestProcessVitestFixture`; `TestProcessPytestFixture`; `TestProcessGoTestFixture`; `TestProcessPlaywrightFixture`; `TestProcessExtendedParserFixtures`; `TestGoTestFailureVariants`; `TestFrameworkParsersFromCapturedStream`; `TestFrameworkParsersFromRawLogFile`; `TestConfiguredRunsUseSpecializedParsers`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQEXT-003` | `TestProcessGenericFailureProducesPreciseSpan`; `TestProcessRulesBoundsUnvalidatedContext` |
| `GAORI-REQ-RQEXT-004` | `TestProcessGenericFailureProducesPreciseSpan`; `TestProcessVitestFixture`; `TestProcessPytestFixture`; `TestProcessGoTestFixture`; `TestProcessPlaywrightFixture`; `TestProcessExtendedParserFixtures`; `TestFrameworkParsersFromCapturedStream`; `TestFrameworkParsersFromRawLogFile`; `TestConfiguredRunsUseSpecializedParsers`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQEXT-005` | `TestProcessExtractorStatusContract`; `TestNoisyRunsWriteBoundedTerminalArtifacts` |
| `GAORI-REQ-RQEXT-006` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQEXT-007` | `TestMaterializeArtifactsExtractionErrorContract`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQRUL-001` | `TestRulesLifecycleCommands`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQRUL-002` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands`; portable Git policy in `README.md` and `ADR-0011` |
| `GAORI-REQ-RQRUL-003` | `TestValidateStoredRuleRejectsInvalidContextAndStatus`; `TestCreateSearchAndDeleteRule` |
| `GAORI-REQ-RQRUL-004` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands` |
| `GAORI-REQ-RQRUL-005` | `TestTestRuleMatchesExpectedSpan`; `TestRuleMatchesCRLFLineEndings`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQRUL-006` | `TestRuleDetectsOvermatch`; `TestProcessRulesRejectsInvalidRegex`; `TestBinaryRejectsOversizedRuleContext` |
| `GAORI-REQ-RQRUL-007` | `TestProposeWritesRunLocalProposal`; `TestProposePreservesMeaningfulLineWhitespace` |
| `GAORI-REQ-RQRUL-008` | `TestLoadApplicableRequiresAllRuleTags`; `TestRunAndSummarizeSelectRulesByAllTags`; `TestBinaryTagsSelectRulesByAllTags` |
| `GAORI-REQ-RQRUL-009` | `TestSummaryProposalCapturesBoundedSpanDuringChecksum`; `TestSummaryProposalBindsSelectedBytesToChecksumStream`; `TestRulesProposeFromSummaryStreamsLargeRawLog`; `TestRulesProposeFromSummaryAcceptsExtractorLineBoundaries`; `TestRulesProposeFromSummaryFailsClosedOnUntrustedEvidence`; `TestRulesProposeModesAreMutuallyExclusive`; `TestBinaryProposesRuleFromGeneratedFailureSummary`; `TestBinaryProposesRuleFromGeneratedLFTerminalFailureSummary`; `TestBinaryRejectsRuleProposalFromSummaryThatNoLongerMatchesStatus` |
| `GAORI-REQ-RQSEC-001` | `TestRedactSummaryCoversSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `GAORI-REQ-RQSEC-002` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `GAORI-REQ-RQSEC-003` | `TestExecuteReportsRawLogWriteFailure`; `TestExecuteTimeoutReportsRawLogWriteFailure`; `TestExecuteInterruptedReportsRawLogWriteFailure`; `TestRawLogWriteFailureDoesNotPublishDerivedArtifacts`; `TestProcessRulesRejectsInvalidRegex`; `TestBinaryRejectsUnknownConfigFields`; `TestBinaryRejectsOversizedRuleContext` |
| `GAORI-REQ-RQSEC-004` | `TestWriteSummaryJSONFailsWhenTooLarge`; `TestProcessOversizedLogUsesBoundedTail`; `TestProcessPytestDetailScanIsBounded`; `TestProcessRulesRejectsOversizedInput`; `TestTestRuleBoundsFixtureBeforeExtraction`; `TestBoundSummaryEvidenceCapsRecordsAndKeepsCountsAligned`; `TestBoundSummaryEvidenceUsesRenderedByteBudget`; `TestBoundSummaryEvidenceIncludesJSONTrailingNewlineInByteBudget`; `TestBoundSummaryEvidenceUsesRemainingBudgetForWarnings`; `TestNoisyRunsWriteBoundedTerminalArtifacts`; `TestLoadEnforcesInputSizeLimit`; `TestLoadAllEnforcesRuleFileSizeLimit`; `TestRuleSourceFilesEnforceInputSizeLimit`; `TestProposeEnforcesRawLogInputSizeLimit`; `TestRulesProposeFromSummaryStreamsLargeRawLog`; `TestExcerptRejectsOversizedEvidence`; `TestBinaryEnforcesRuleAndConfigInputSizeLimits` |
| `GAORI-REQ-RQSEC-005` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `GAORI-REQ-RQWAT-001` | `TestConfiguredRunAndExcerpt`; `TestRawLogWriteFailureDoesNotPublishDerivedArtifacts`; status-hash assertions in CLI and binary tests; `TestNoisyRunsWriteBoundedTerminalArtifacts` |
| `GAORI-REQ-RQWAT-002` | `TestComputeStatusHashIncludesTags`; status-hash assertions in CLI and binary tests |
| `GAORI-REQ-RQWAT-003` | `TestBinaryJSONRedactsCommandMetadata`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQMCP-001` | `TestMCPServerAdvertisesExpectedTools`; `TestBinaryMCPExitsCleanlyOnEOF`; `TestBinaryMCPImmediateEOFIsCleanShutdown`; `TestBinaryMCPEmptyEOFIsCleanShutdown`; `TestBinaryMCPMalformedInputFails` (including truncated JSON, partial UTF-8, and trailing partial frames); `TestBinaryMCPLifecycleAndBoundedEvidence` |
| `GAORI-REQ-RQMCP-002` | `TestExecuteRunContextReportsLifecycleAndPassesCallerContext`; `TestMCPManagerWaitDoesNotCancelAndExplicitCancelFinishes`; `TestMCPMaterializingTransitionWakesRevisionWait`; `TestMCPTimeoutInputsRejectExplicitInvalidValues`; `TestBinaryMCPLifecycleAndBoundedEvidence` |
| `GAORI-REQ-RQMCP-003` | `TestMCPManagerWaitDoesNotCancelAndExplicitCancelFinishes`; `TestMCPWaitCancellationDoesNotCancelRun`; `TestMCPWaitRejectsFutureRevision`; `TestMCPTimeoutInputsRejectExplicitInvalidValues`; `TestBinaryMCPLifecycleAndBoundedEvidence` |
| `GAORI-REQ-RQMCP-004` | `TestMCPManagerWaitDoesNotCancelAndExplicitCancelFinishes`; `TestMCPManagerCloseCancelsActiveRun`; `TestWaitMCPInvocationsUsesOneDrainContext`; `TestStartGateSerializesCancellationAfterProcessStart`; `TestBinaryMCPEOFCancelsActiveProcessGroup`; `TestBinaryMCPSignalsShutDownServerAndProcessGroup`; `TestBinaryMCPLifecycleAndBoundedEvidence` |
| `GAORI-REQ-RQMCP-005` | `TestMCPRunErrorsUseValidatedRedaction`; `TestMCPRunErrorsHideDetailsBeforeRedactorIsAvailable`; `TestMCPInvocationLookupErrorsAreBoundedAndNonReflective`; `TestMCPExcerptReturnsFinalizedContentWithoutSecondRedaction`; `TestBinaryMCPLifecycleAndBoundedEvidence` (including finalized excerpt integrity and request-data non-reflection) |
| `GAORI-REQ-RQMCP-006` | `TestMCPManagerWaitDoesNotCancelAndExplicitCancelFinishes`; `TestMCPDocumentationAndSkillContract` |
| `GAORI-REQ-RQDOC-001` | authoritative documents listed in `AGENTS.md` and `README.md`; `TestArchitectureJSONContractExamplesMatchFreshRunArtifacts` |
| `GAORI-REQ-RQDOC-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQDOC-003` | parser fixtures under `internal/extract/testdata`; `TestTestRuleMatchesExpectedSpan`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `GAORI-REQ-RQDOC-004` | release-readiness checklist in `implementation-note.md`; `make test` |
| `GAORI-REQ-RQHAR-001` | `TestBinaryArtifactContainment`; `TestBinaryMCPLifecycleAndBoundedEvidence` (relocated MCP evidence); path, artifact, and rule symlink tests |
| `GAORI-REQ-RQHAR-002` | `TestBinaryPreservesInterruptedEvidence`; `TestExecuteInterruptedReportsRawLogWriteFailure`; Unix runner signal tests |
| `GAORI-REQ-RQHAR-003` | `TestBinaryStandaloneCollisionResistance`; concurrent artifact allocation tests |
| `GAORI-REQ-RQHAR-004` | `TestBinaryJSONRedactsCommandMetadata`; CLI redaction integration tests |
| `GAORI-REQ-RQHAR-005` | `TestBinaryExtractionContracts`; CLI extraction contract tests |
| `GAORI-REQ-RQHAR-006` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; toolchain script E2E tests |
| `GAORI-REQ-RQHAR-007` | `make test`; focused binary containment, signal, collision, extraction, install, and workflow E2E tests |

The matrix is traceability evidence, not acceptance authority. Command exit status and the artifact contracts remain authoritative for individual runs.
