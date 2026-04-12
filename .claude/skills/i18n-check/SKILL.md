---
description: "Check translation completeness and detect untranslated strings. Use when the user says '/i18n-check', 'check translations', 'find untranslated strings', 'verify i18n', or 'translate missing'. Three modes: SCAN (detect hardcoded strings not wrapped in translation functions), TRANSLATE (generate missing translations from source language), VERIFY (compare key sets across languages)."
user_invocable: true
---

# i18n-check -- Translation Completeness Checker

You are an i18n quality checker for a multi-language application.

<!-- TODO: Configure for your project:
  - Languages: e.g., en, nl, de (or en, fr, es, etc.)
  - Source language: e.g., en
  - Translation file path: e.g., frontend/src/locales/{lang}/{namespace}.json
  - Translation function: e.g., t() for react-i18next, $t() for vue-i18n, intl.formatMessage for react-intl
  - Component directories to scan: e.g., frontend/src/pages/, frontend/src/components/
-->

## Arguments

The argument after `/i18n-check` determines the mode:
- `/i18n-check scan` or `/i18n-check scan src/pages/MyPage.tsx` -- detect hardcoded strings
- `/i18n-check translate` -- generate missing translations from source language
- `/i18n-check verify` -- compare key sets across languages
- No argument -- run all three modes in sequence

## Mode 1: SCAN -- Detect Hardcoded Strings

Find user-visible hardcoded strings in components that should use translation functions.

**What counts as a hardcoded string:**
- String literals inside JSX/template elements: `<h1>Welcome</h1>`
- String literals in user-visible attributes: `placeholder="Enter name"`, `title="..."`, `aria-label="..."`
- Template literals with user-visible text
- String constants assigned and rendered

**What to IGNORE:**
- CSS class names, data attributes, test IDs
- Route paths, API paths
- Event handler strings, console.log messages
- Import paths, module names
- Technical identifiers (enum values, API keys)
- Strings already wrapped in translation functions
- Numbers, booleans

**Output format:**
```
## SCAN Results

Found N hardcoded strings in M files:

### path/to/MyPage.tsx
- Line 42: "Welcome back" -- suggested key: `dashboard:welcomeBack`
- Line 58: "Save changes" -- suggested key: `common:saveChanges`
```

## Mode 2: TRANSLATE -- Generate Missing Translations

Compare source language keys against other languages and generate translations.

**Steps:**
1. Read all source language translation files
2. For each namespace, read the corresponding files for other languages
3. Find keys present in source but missing in target languages
4. Generate translations (match the style and tone of existing translations)
5. Write the missing keys into the target language files

## Mode 3: VERIFY -- Cross-Check Completeness

Compare key sets across all languages and report discrepancies.

**Output format:**
```
## VERIFY Results

| Namespace | EN keys | NL keys | DE keys | Status |
|-----------|---------|---------|---------|--------|
| common    | 45      | 45      | 45      | OK     |
| auth      | 12      | 12      | 11      | MISSING|

### Issues Found
auth.json:
- Missing in DE: `passwordRulesTitle`
```

## Guidelines

- Never modify the source language files in TRANSLATE mode
- Prefer natural-sounding language over literal translation
- Flag uncertain translations with `[REVIEW]` prefix
- Preserve JSON key ordering
