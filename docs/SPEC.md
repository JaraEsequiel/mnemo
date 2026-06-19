# mnemo — Spec (v0)

> **mnemo** — Memoria de conocimiento MD-first para agentes de IA. El markdown es la
> fuente de verdad; SQLite + FTS5 es solo un índice derivado, reconstruible.
> Inspirado en engram (técnicas), el patrón LLM Wiki (arquitectura), y los
> plugins de productividad de Anthropic (SKILL.md como contrato).

## Decisiones del grill (primer usuario / dogfood)

| Eje | Decisión |
|-----|----------|
| Dominio | Mixto / segundo cerebro de builder: código + proyectos personales + ideas (apps/dashboards) + vida/productividad |
| Niveles | L0 + L1 + L2 + L3 + crónica |
| **L0 hot cache** | **Es un `CLAUDE.md`** (carga nativa de Claude Code). Auto promote/demote. |
| Organización | **Plano por tipo**; tags para el contexto |
| Código | Misma memoria plana (sin subtree); decisiones tagueadas por proyecto |
| Alcance | Solo yo, una máquina. Sin `scope:`. Git solo historial. `wiki.db` gitignored |
| Ciclo de vida | Revisa-in-place; contradicciones marcadas en `/lint`; `review_after` por tipo (suave) |
| Salidas | Graph view, file-back, tablas/dashboards, slides Marp |

## Formato: MD plano, no MDX

La fuente de verdad es **CommonMark + YAML frontmatter + `[[wikilinks]]`**. **No MDX.** Razones:
- Obsidian (el IDE del vault) y `CLAUDE.md` (L0, cargado por Claude Code) requieren markdown plano.
- MDX mete JSX/imports que ensucian el índice FTS5 y los snippets, y obligan a un runtime/bundler.
- El agente mantiene plain MD de forma robusta; MDX invita a JSX roto y acoplamiento a componentes.
- Lo "dinámico" se cubre sin MDX: Dataview/callouts de Obsidian, tablas MD generadas, Marp.

**MDX es un target de render, no un formato de memoria.** Si en el futuro hay un dashboard/app,
ese front *consume* el markdown y *genera* MDX/componentes en la capa de presentación — derivado,
descartable, nunca la verdad (igual que `wiki.db`).

## Las tres capas (LLM Wiki)

```
RAW (raw/, inmutable)  →  WIKI (.md por tipo, VERDAD)  →  SCHEMA.md (contrato) + CLAUDE.md (L0)
                              │
                              ├─ index.md por carpeta (slug → descripción)
                              ├─ log.md (crónica append-only)
                              ▼
                     ÍNDICE FTS5 derivado (.mnemo/wiki.db, gitignored)
                              ▲ refresco a demanda: mnemo index / ingest / lint
                              │  (el MCP además reindexa en cada búsqueda)
                     MCP server  ←→  cualquier agente (Claude Code, …)
```

## Estructura del vault (lo que produce `/start` para este usuario)

```
brain/
  CLAUDE.md                 ← L0 hot cache (nativo Claude Code). Top entidades/proyectos/activo.
  SCHEMA.md                 ← contrato de mantenimiento (el agente lo relee cada sesión)
  index.md                  ← catálogo raíz (enlaza el index.md de cada carpeta)
  log.md                    ← crónica  ## [2026-06-19] ingest | título
  working.md                ← L1 working memory (hilos/objetivos activos ahora)
  entities/   index.md + <slug>.md    (personas, tecnologías, herramientas, orgs)
  concepts/   index.md + <slug>.md    (patrones, modelos mentales, conceptos)
  projects/   index.md + <slug>.md    (proyectos personales, estado/seguimiento)
  ideas/      index.md + <slug>.md    (propuestas de apps/dashboards)
  decisions/  index.md + <slug>.md    (decisiones, incl. de código, tag por proyecto)
  sources/    index.md + <slug>.md    (una página-resumen por fuente ingerida)
  raw/                                (L3: originales inmutables; mnemo lee, nunca edita)
  .obsidian/graph.json                (graph view opinado por tipo)
  .mnemo/
    config.json                       (salida del grill)
    start-answers.json
    wiki.db                           (índice FTS5 derivado — gitignored)
  .gitignore                          (.mnemo/wiki.db)
```

## Formato de página

```markdown
---
slug: jwt-auth-model
title: JWT auth model
type: decision
tags: [code, engram, auth]
description: JWT + refresh tokens, sesiones en redis   # ← alimenta index.md
created: 2026-06-19
updated: 2026-06-19
review_after: 2026-12-19          # opcional, por tipo
links: [redis-sessions, user-store]   # salientes (slugs) → [[wikilinks]]
---

# JWT auth model

**What**: ...
**Why**: ...
**Where**: ...
**Learned**: ...

---
*Links*: [[redis-sessions]] · [[user-store]]
```

## `index.md` por carpeta (requisito clave del usuario)

Auto-generado/refrescado desde el `description:` de cada página:

```markdown
# Index — decisions

| Slug | Descripción | Tags |
|------|-------------|------|
| [[jwt-auth-model]] | JWT + refresh tokens, sesiones en redis | code, engram, auth |
| [[flat-vault-layout]] | Vault plano por tipo, tags para contexto | mnemo, architecture |
```

Sirve de navegación en Obsidian **y** de tabla de ruteo barata para el agente
(lee el index → baja al archivo: progressive disclosure).

## Motor (módulo Go `mnemo`)

```
cmd/mnemo/main.go        init|start · index · search · mcp · lint · graph · relate · hot
internal/
  vault/    page.go (parse frontmatter+body+links), index.go (genera index.md), config.go
  ftsindex/ index.go (pages_fts external-content), reindex.go (incremental por hash)
  (refresco a demanda vía `mnemo index` / ingest / lint; sin demonio en background)
  mcp/      wiki_search · wiki_get · wiki_list_index · wiki_ingest · wiki_update · wiki_lint
  scaffold/ /start (grill → config.json + SCHEMA.md + scaffold)
  outputs/  graph.json, marp, tablas (fase 2)
  llm/      juez de contradicciones vía claude/opencode CLI (fase 2)
skills/     start · ingest · query · lint · mnemo-maintainer (SKILL.md = contrato)
plugin/.claude-plugin/  plugin.json · .mcp.json
```

## Índice FTS5

`pages(path PK, slug, type, tags, title, description, body, mtime, hash)` +
`pages_fts` FTS5 external-content sobre `(slug, title, description, tags, body)`.
Reindex: walk vault → comparar hash → upsert cambios / borrar ausentes. Se dispara a demanda
(`mnemo index`, `/mnemo:ingest`, `/mnemo:lint`) y en cada búsqueda del MCP. Search: BM25 +
snippet, filtro por type/tags, fast-path exacto por slug.

## Plan de entrega (slices verticales)

1. ✅ **Esqueleto + índice + search (CLI)** — `mnemo init/index/search`, FTS5 BM25 + snippet, reindex incremental por hash.
2. ✅ **`index.md` jerárquico** — `GenerateIndexes`: catálogo `slug → descripción` por carpeta + raíz, escritura idempotente. Fundido en `mnemo index`; comando `mnemo indexes`.
3. ✅ **Refresco a demanda** — sin demonio en background (el watcher se eliminó por simplicidad): el índice + catálogos se regeneran con `mnemo index` / `/mnemo:ingest` / `/mnemo:lint`, y el MCP reindexa en cada búsqueda. Modelo deliberado estilo LLM-Wiki: vuelcas crudo → decides cuándo procesarlo.
4. ✅ **MCP server** — `internal/mcp` (mcp-go v0.44.0): `wiki_search`, `wiki_get`, `wiki_list` por stdio. `mnemo mcp`. Probado con JSON-RPC real.
5. ✅ **Plugin + skills** — `plugin/.claude-plugin/plugin.json`, `.mcp.json`, skills `mnemo-maintainer` (contrato siempre activo, incl. L0=CLAUDE.md promote/demote), `/start`, `/ingest`, `/query`, `/lint`.
6. 🟡 **Fase 2 (en curso)**:
   - ✅ **Stemming porter** en FTS5 + versionado de esquema (rebuild del índice desechable).
   - ✅ **`graph.json`** de Obsidian (`internal/graph`, color por carpeta) + `mnemo graph`.
   - ✅ **Tests** Go (`vault`, `ftsindex`, relaciones) — `go test ./...` verde.
   - ✅ **Motor de relaciones/contradicciones** — relaciones tipadas con razón obligatoria en un
     bloque gestionado `## Related` del body (MD = verdad). Vocab: supersedes, conflicts_with,
     related, refines, depends_on. Tabla `relations` derivada. Tools MCP `wiki_candidates`
     (detección OR) + `wiki_relate` (juez = el agente, $0). Anotación bidireccional en search.
     CLI `mnemo candidates|relate`. Skills `/ingest` `/lint` `mnemo-maintainer` actualizadas.
   - ✅ **Promote/demote asistido** — `mnemo hot suggest` + tool MCP `wiki_hot`: señales (inbound
     links via tabla `links`, recencia via mtime); el agente decide y edita CLAUDE.md.
   - ✅ **Juez headless** — `internal/llm` (claude/opencode runner, parser de envelope con tests);
     `mnemo lint --semantic [--verbose] [--max N]` juzga candidatos (recientes primero) vía
     `claude -p`/`opencode` y escribe relaciones fuertes con la razón del modelo. **Probado end-to-end
     con el `claude` CLI real**: detectó tabs-vs-spaces → `conflicts_with` (0.99) → escrito.
   - ⬜ **Pendiente (fase 3)**: salidas Marp / tablas Dataview generadas; prefix-search opcional;
     bloque gestionado en CLAUDE.md escrito por el agente; concurrencia en el juez headless.
