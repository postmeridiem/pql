# Decisions

Confirmed decisions, open questions, and rejected alternatives for pql.

Records follow the `### D|Q|R-NNN: Title` heading convention parsed by
`pql decisions sync`. Run `pql decisions sync` after editing to update
the planning database.

## Domain files

| File | Domain |
|------|--------|
| [architecture.md](architecture.md) | Core design constraints |
| [questions.md](questions.md) | Open questions |

## Record shape

Confirmed decisions (`D-NNN`):

```markdown
### D-NNN: Short title
- **Date:** YYYY-MM-DD
- **Decision:** one-sentence summary, then details.
- **Rationale:** why this over alternatives.
- **Cost:** known downsides.
- **Raised by:** who proposed.
```

Open questions (`Q-NNN`):

```markdown
### Q-NNN: Short question
- **Status:** Open | Resolved → [D-NNN]
- **Question:** ...
- **Context:** ...
```

## Querying

```bash
pql decisions sync                        # parse → pql.db
pql decisions list --type confirmed       # list D-records
pql decisions show D-1 --with-refs        # show with cross-references
pql decisions coverage                    # D-records without tickets
pql decisions validate                    # check for errors
```
