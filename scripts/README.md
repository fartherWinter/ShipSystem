# Research scripts

The `.mjs` research scripts are UTF-8 encoded and contain Chinese workbook labels.
On Windows PowerShell 5, `Get-Content` may display UTF-8 text as mojibake when the
console code page is not UTF-8. Use Node or an editor that honors `.editorconfig`
when inspecting or editing these files.

Rebuild the GitHub statistics workbook:

```powershell
node scripts/build_ship_system_github_stats.mjs
node scripts/add_function_analysis.mjs
```

Set `REFRESH_GITHUB=1` before the first command to refresh the GitHub API cache.
