---
slug: redis-sessions
title: Redis sessions
type: entity
tags: [code, infra, redis]
description: Redis store for refresh tokens and session state
---
# Redis sessions
Backs the JWT refresh-token flow; keyed by user id with short TTLs.
