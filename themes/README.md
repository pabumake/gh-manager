# Theme Repository Files

- `index.json`: curated installable theme list used by `gh-manager theme list --remote` and `gh-manager theme install <id>`.
- `*.json`: theme definitions with hex colors (`#RRGGBB`).
- Remote consumers use: `https://raw.githubusercontent.com/pabumake/gh-manager/main/themes/index.json`

## Contribution

1. Add a new `themes/<id>.json` file.
2. Add the theme entry to `themes/index.json` (relative URLs are allowed and resolved from index location).
3. Open a pull request.
