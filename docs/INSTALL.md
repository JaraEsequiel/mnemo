# Instalar y probar mnemo en Claude Code

> **Modelo mental:** el **markdown es la verdad** (tu vault). mnemo lo indexa en
> FTS5 (`.mnemo/wiki.db`, desechable) y lo expone por MCP a tu agente.

## 0. Requisitos

- **Go 1.23+** (para compilar): `go version`
- **Claude Code** (CLI): `claude --version`
- **Obsidian** (opcional, para el grafo) · **`claude` CLI en PATH** (opcional, para `mnemo lint --semantic`)

---

## 1. Instalación pública (sin clonar el repo)

**Paso 1 — el binario** (elige uno):

```bash
# Release prebuilt (sin Go)
curl -fsSL https://raw.githubusercontent.com/JaraEsequiel/mnemo/main/scripts/get.sh | sh

# …o con Go
go install github.com/JaraEsequiel/mnemo/cmd/mnemo@latest
```

**Paso 2 — conectarlo a Claude Code** (elige uno):

```bash
# A) Wizard interactivo (vault + scope MCP + skills + graph)
mnemo setup

# B) Como plugin de Claude Code (skills + MCP; usa ~/.mnemo/vault por defecto)
claude plugin marketplace add JaraEsequiel/mnemo
claude plugin install mnemo
```

> El plugin arranca `mnemo mcp`, que es **zero-config**: si no hay `--vault` ni
> `MNEMO_VAULT` ni un `.mnemo/` en el árbol, crea y usa `~/.mnemo/vault`.

---

## 1-bis. Desde el repo (contribuir)

```bash
./install.sh
```

Esto compila el binario en `~/.local/bin/mnemo` y lanza el **wizard TUI** `mnemo setup`:
- **dónde** quieres tu vault (por defecto `~/brain`),
- el **scope** del MCP (`user` = todos tus proyectos, recomendado),
- si escribir la **config de graph** de Obsidian.

Y deja todo hecho — **sin `export` ni copiar-pegar**:

```
✓ vault ready at ~/brain
✓ index built (.mnemo/wiki.db)
✓ Obsidian graph config written
✓ skills installed (/mnemo:start, /mnemo:ingest, /mnemo:query, /mnemo:lint)
✓ MCP server registered (scope: user)
```

> Si `~/.local/bin` no está en tu PATH, el script te avisa con la línea a añadir.

### Sin interacción (CI / scripts)

```bash
./install.sh --yes --vault "$HOME/brain" --scope user
```

(Flags de `mnemo setup`: `--vault --scope --skills-dest --no-mcp --no-graph --yes`.)

---

## 2. Lanzar Claude Code en el vault

El hot cache **L0 es el `CLAUDE.md`** del vault, que Claude Code carga solo:

```bash
cd ~/brain && claude
```

Dentro de Claude Code:
- `/mcp` → debe listar **mnemo** como *Connected*.
- `/mnemo:start` → te entrevista para afinar el vault (taxonomía, niveles).
- O pídele en lenguaje natural: *"busca en mi memoria sobre X"* → usa `wiki_search`.

---

## 3. Qué hace el instalador (referencia)

`mnemo setup` ejecuta, de forma idempotente:

| Paso | Acción |
|------|--------|
| vault | `mnemo init` — estructura plana por tipo + `CLAUDE.md`/`working.md`/`log.md`/`.gitignore` |
| índice | construye `.mnemo/wiki.db` (FTS5) + los `index.md` por carpeta |
| graph | escribe `.obsidian/graph.json` (modo *preserve*) |
| skills | copia `plugin/skills` a `~/.claude/skills/mnemo/` (comandos `/mnemo:*`) |
| MCP | `claude mcp add mnemo --scope <s> -e MNEMO_VAULT=<vault> -- mnemo mcp` |

`MNEMO_VAULT` (variable que pasa el instalador al servidor MCP) hace que mnemo
encuentre tu vault **desde cualquier directorio**.

---

## 4. (Opcional) Extras

```bash
# Auto-reindex al guardar cualquier .md (déjalo corriendo)
mnemo watch --vault ~/brain &

# Juez de contradicciones headless (sin agente activo)
export MNEMO_AGENT_CLI=claude
mnemo lint --semantic --vault ~/brain --verbose --max 20

# Estadísticas / sugerencias del hot cache
mnemo stats --vault ~/brain
mnemo hot suggest --vault ~/brain
```

---

## 5. Actualizar

```bash
cd <repo> && git pull        # si aplica
./install.sh --yes --vault ~/brain --scope user
```

Reinicia Claude Code para que recargue el binario MCP (un proceso MCP ya en
marcha no se reemplaza solo).

---

## 6. Desinstalar

```bash
claude mcp remove mnemo --scope user
rm -rf ~/.claude/skills/mnemo
rm -f  ~/.local/bin/mnemo
# Tu vault queda intacto — es solo markdown.
```

---

## Alternativa manual (sin el instalador)

```bash
go build -o ~/.local/bin/mnemo ./cmd/mnemo
mnemo init --vault ~/brain
claude mcp add mnemo --scope user -e MNEMO_VAULT="$HOME/brain" -- mnemo mcp
mkdir -p ~/.claude/skills/mnemo
cp -r plugin/.claude-plugin plugin/skills ~/.claude/skills/mnemo/
cd ~/brain && claude
```
