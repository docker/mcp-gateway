# docker mcp catalog create

<!---MARKER_GEN_START-->
Create a new catalog from a profile, legacy catalog, or community registry

### Options

| Name                        | Type          | Default | Description                                                                                                                                                                                                      |
|:----------------------------|:--------------|:--------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--from-community-registry` | `string`      |         | Community registry hostname to fetch servers from (e.g. registry.modelcontextprotocol.io)                                                                                                                        |
| `--from-legacy-catalog`     | `string`      |         | Legacy catalog URL to create the catalog from                                                                                                                                                                    |
| `--from-profile`            | `string`      |         | Profile ID to create the catalog from                                                                                                                                                                            |
| `--server`                  | `stringArray` |         | Server to include specified with a URI: https:// (MCP Registry reference) or docker:// (Docker Image reference) or catalog:// (Catalog reference) or file:// (Local file path). Can be specified multiple times. |
| `--title`                   | `string`      |         | Title of the catalog                                                                                                                                                                                             |


<!---MARKER_GEN_END-->

