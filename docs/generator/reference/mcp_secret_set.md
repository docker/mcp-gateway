# docker mcp secret set

<!---MARKER_GEN_START-->
Set a secret in the local OS Keychain


<!---MARKER_GEN_END-->

## Examples

### Use secrets for postgres password with default policy

```console
docker mcp secret set postgres_password=my-secret-password
```

Inject the secret by querying by ID:
```console
docker run -d -e POSTGRES_PASSWORD=se://docker/mcp/postgres_password -p 5432 postgres
```

Another way to inject secrets would be to use a pattern:
```console
docker run -d -e POSTGRES_PASSWORD=se://**/postgres_password -p 5432 postgres
```

### Pass the secret via STDIN

```console
echo my-secret-password > pwd.txt
cat pwd.txt | docker mcp secret set postgres_password
```