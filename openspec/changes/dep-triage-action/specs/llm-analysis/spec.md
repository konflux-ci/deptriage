## ADDED Requirements

### Requirement: Render prompt from template with context
The system SHALL render a prompt by substituting `{{BUMP_TYPE}}`, `{{PR_TITLE}}`, and `{{PACKAGE_CONTEXT}}` placeholders in an embedded Go template with actual values from the classify output and context JSON.

#### Scenario: All placeholders substituted
- **WHEN** the template contains `{{BUMP_TYPE}}`, `{{PR_TITLE}}`, and `{{PACKAGE_CONTEXT}}`
- **THEN** the system SHALL replace each with the corresponding value and produce the final prompt text

#### Scenario: Context JSON is large
- **WHEN** the rendered prompt exceeds the LLM's context window
- **THEN** the system SHALL truncate the `{{PACKAGE_CONTEXT}}` (starting from the longest changelog) to fit within limits

### Requirement: Support multiple LLM providers via common interface
The system SHALL support Gemini and Claude as LLM providers. The provider SHALL be selected via a `--provider` flag or `LLM_PROVIDER` environment variable. Each provider SHALL implement a common interface that accepts a prompt string and returns the LLM response text.

#### Scenario: Gemini provider selected
- **WHEN** the provider is set to `gemini`
- **THEN** the system SHALL call the Gemini `generateContent` API with the prompt and return the response text

#### Scenario: Claude provider selected
- **WHEN** the provider is set to `claude`
- **THEN** the system SHALL call the Anthropic Messages API with the prompt and return the response text

#### Scenario: Unknown provider specified
- **WHEN** an unsupported provider name is specified
- **THEN** the system SHALL exit with an error listing the supported providers

### Requirement: Handle LLM API errors gracefully
The system SHALL never block CI due to LLM API failures. All API errors SHALL result in graceful degradation.

#### Scenario: API key not configured
- **WHEN** the API key environment variable for the selected provider is empty
- **THEN** the system SHALL skip analysis and post a "skipped: API key not configured" comment

#### Scenario: API call timeout
- **WHEN** the LLM API call does not complete within 120 seconds
- **THEN** the system SHALL abort and post an "analysis unavailable" comment

#### Scenario: API returns non-200 status
- **WHEN** the LLM API returns an HTTP error (4xx, 5xx)
- **THEN** the system SHALL log the error and post an "analysis unavailable" comment

#### Scenario: Malformed API response
- **WHEN** the API response cannot be parsed (invalid JSON, missing fields)
- **THEN** the system SHALL log the error and post an "analysis unavailable" comment

### Requirement: Parse risk level from LLM response
The system SHALL extract the risk level (LOW, MEDIUM, HIGH) from the LLM response text by matching the pattern `Risk Level: <LEVEL>`.

#### Scenario: Risk level present in response
- **WHEN** the LLM response contains `### Risk Level: HIGH`
- **THEN** the system SHALL extract `high` as the risk level

#### Scenario: Risk level not found in response
- **WHEN** the LLM response does not contain a recognizable risk level pattern
- **THEN** the system SHALL default to risk level `unknown`

### Requirement: Redact secrets from LLM output before posting
The system SHALL scan LLM response text for potential secrets (API keys, tokens, credentials) using regex patterns and replace them with `[REDACTED]` markers before the text is posted to GitHub.

#### Scenario: LLM output contains API key pattern
- **WHEN** the LLM response text contains a string matching common API key patterns (e.g., `AKIA...`, `ghp_...`, `sk-...`)
- **THEN** the system SHALL replace the matched string with `[REDACTED]`

#### Scenario: LLM output contains no secrets
- **WHEN** the LLM response text contains no patterns matching known secret formats
- **THEN** the system SHALL pass the text through unchanged

#### Scenario: Multiple secrets in output
- **WHEN** the LLM response contains multiple secret-like patterns
- **THEN** the system SHALL redact all of them
