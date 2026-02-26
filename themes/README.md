# Theme Repository Files

- `index.json`: curated installable theme list used by `gh-manager theme list --remote` and `gh-manager theme install <id>`.
- `*.json`: theme definitions with hex colors (`#RRGGBB`) and optional variables.
- Remote consumers use: `https://raw.githubusercontent.com/pabumake/gh-manager/main/themes/index.json`

Theme variable syntax:
- Declare shared tokens in top-level `vars`.
- Reference tokens from `colors` (or other vars) via `var(--token)`.
- `colors` still accepts direct hex values for one-off assignments.
- Unknown variables, invalid refs, or cycles fail theme parsing.

## Contribution

1. Add a new `themes/<id>.json` file.
2. Add the theme entry to `themes/index.json` (relative URLs are allowed and resolved from index location).
3. Open a pull request.
