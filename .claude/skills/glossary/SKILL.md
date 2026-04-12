---
description: "Manage the Ubiquitous Language glossary. Use when the user says '/glossary', 'add to glossary', 'define term', or 'check glossary'. Also trigger proactively during /design-grill and feature discussions when undefined or ambiguous terms appear."
user_invocable: true
---

# Glossary -- Ubiquitous Language

Manage the shared vocabulary in `GLOSSARY.md`. Every term has a single agreed-upon meaning used consistently across code, specs, conversations, and documentation.

## Modes

### 1. Show Glossary (no arguments)

Display the current glossary contents, organized by section.

### 2. Add Term (`/glossary add <term>`)

When a new term is proposed:

1. **Check for duplicates** -- read GLOSSARY.md and check if the term (or a synonym) already exists
2. **If duplicate found** -- show the existing definition and ask if it needs updating instead
3. **If new** -- propose a definition based on the conversation context and codebase usage
4. **Get agreement** -- present the proposed definition to the user. Do NOT add it until the user confirms.
5. **Add** -- insert the term in the correct section of GLOSSARY.md, maintaining alphabetical order within sections.

### 3. Suggest Terms (proactive mode)

When triggered during design-grill or feature discussions:

1. **Scan the conversation** for terms that:
   - Are used with domain-specific meaning (not standard programming terms)
   - Could be interpreted differently by different people
   - Are new to this project (not in the glossary yet)

2. **Cross-reference with GLOSSARY.md** -- only suggest terms not already defined

3. **Present suggestions** as a list:
   ```
   ## Glossary Suggestions
   These terms came up and aren't in the glossary yet:
   - **<term>** -- used to mean <observed meaning>. Add to glossary?
   - **<term>** -- ambiguous: could mean X or Y. Clarify and add?
   ```

4. **Wait for user** -- do not add any term without explicit confirmation

### 4. Update Term (`/glossary update <term>`)

1. Find the existing definition in GLOSSARY.md
2. Show current definition
3. Propose updated definition based on new context
4. Get user agreement before applying

## Rules

- **Never add a term without user confirmation.** The glossary is a shared agreement, not a unilateral decision.
- **Check for synonyms and overlaps.** Before adding "Budget Overrun", check if "Overrun" already covers it.
- **Keep definitions precise but readable.** One paragraph max.
- **Use the term consistently in code.** If the glossary says "Book Year", the entity should be `BookYear`, not `FiscalYear`.
- **Organize by domain section**, not alphabetically across the whole file.
- **Definitions should be stable.** Don't change a definition just because a new feature uses the term slightly differently.

## Glossary File Location

`GLOSSARY.md` in the project root. Create it if it doesn't exist.
