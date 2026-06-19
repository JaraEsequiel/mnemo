---
slug: oauth-migration
title: Migrate auth to OAuth2 provider
type: decision
tags: [code, engram, auth]
description: Drop self-managed JWT; delegate auth to an external OAuth2 provider
created: 2026-06-19
updated: 2026-06-19
---
# Migrate auth to OAuth2 provider
**What**: Replace the in-house JWT + refresh token model with an external OAuth2 provider.
**Why**: Offload token rotation, revocation, and session storage.

<!-- mnemo:relations -->
## Related
- supersedes [[jwt-auth-model]] — OAuth2 provider replaces the self-managed JWT model
<!-- /mnemo:relations -->
