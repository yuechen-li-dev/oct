# Oct MCP publication checklist

- [x] Local Codex plugin package with one repository-first workflow skill.
- [x] Streamable HTTP `/mcp`, health endpoint, source-only hosted workflow tools, published limits and deployment contract.
- [x] Privacy, terms, support, threat-model and public claims drafts.
- [ ] Owner supplies HTTPS MCP endpoint, privacy URL, terms URL, support/security contact, legal entity/effective date, hosting/subprocessor list, rate-limit policy and review credentials/test mode.
- [ ] Complete the publisher identity/organization verification for the directory name, create the app record if the current portal requires one, and verify the deployed integration in Developer Mode before review.
- [ ] Build/push an immutable Linux image to owner-selected registry; deploy behind TLS, ingress authentication/quotas, no-egress sandbox, logs/metrics and rollback policy.
- [ ] Run the ChatGPT developer-mode connection using the deployed HTTPS `/mcp` URL; verify discovery, `oct_test`, `oct_artifact`, scoped retrieval, expiry, and explicit fallback reporting.
- [ ] In the current OpenAI submission UI, enter listing metadata, endpoint/auth model, privacy/terms/support URLs, data-retention and safety answers, reviewer credentials, categories and example prompts; submit for review.

Do not represent the plugin as approved or listed until OpenAI completes review.
