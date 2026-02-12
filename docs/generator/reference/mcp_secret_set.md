# docker mcp secret set

<!---MARKER_GEN_START-->
Set a secret in the local OS Keychain


<!---MARKER_GEN_END-->

## Examples

### Pass the secret via STDIN

```console
echo my-secret-password > pwd.txt
cat pwd.txt | docker mcp secret set postgres_password
```