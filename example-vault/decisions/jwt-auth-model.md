---
slug: jwt-auth-model
title: JWT auth model
type: decision
tags: [code, engram, auth]
description: JWT + refresh tokens, redis-backed sessions
created: 2026-06-19
updated: 2026-06-19
links: [redis-sessions, user-store]
---

# JWT auth model

**What**: Use JWT access tokens with rotating refresh tokens.
**Why**: Stateless API auth; refresh tokens let us revoke without a per-request DB hit.
**Where**: internal/server/auth.go
**Learned**: Store refresh tokens in redis keyed by user; short TTL on access tokens.

---
*Links*: [[redis-sessions]] · [[user-store]]
