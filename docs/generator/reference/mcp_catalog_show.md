# docker mcp catalog show

<!---MARKER_GEN_START-->
Show a catalog

### Options

| Name         | Type     | Default | Description                                                                                                                    |
|:-------------|:---------|:--------|:-------------------------------------------------------------------------------------------------------------------------------|
| `--format`   | `string` | `human` | Supported: json, yaml, human.                                                                                                  |
| `--no-tools` | `bool`   |         | Exclude tools from output (deprecated, use --yq instead)                                                                       |
| `--pull`     | `string` | `never` | Supported: missing, never, always, initial, exists, or duration (e.g. '1h', '1d'). Duration represents time since last update. |
| `--yq`       | `string` |         | YQ expression to apply to the output                                                                                           |


<!---MARKER_GEN_END-->

