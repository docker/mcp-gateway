# OAuth Flows Comparison

### 2.2 Basic OAuth 2.1 Authorization

When authorization is required and not yet proven by the client, servers **MUST** respond
with _HTTP 401 Unauthorized_.

Clients initiate the
[OAuth 2.1 IETF DRAFT](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-12)
authorization flow after receiving the _HTTP 401 Unauthorized_.

The following demonstrates the basic OAuth 2.1 for public clients using PKCE.

```mermaid
sequenceDiagram
    participant B as User-Agent (Browser)
    participant C as Client
    participant M as MCP Server

    C->>M: MCP Request
    M->>C: HTTP 401 Unauthorized
    Note over C: Generate code_verifier and code_challenge
    C->>B: Open browser with authorization URL + code_challenge
    B->>M: GET /authorize
    Note over M: User logs in and authorizes
    M->>B: Redirect to callback URL with auth code
    B->>C: Callback with authorization code
    C->>M: Token Request with code + code_verifier
    M->>C: Access Token (+ Refresh Token)
    C->>M: MCP Request with Access Token
    Note over C,M: Begin standard MCP message exchange
```

## Official MCP OAuth diagram mapped

```mermaid
sequenceDiagram
    participant A as Authorisation Server
    participant B as User-Agent (Browser)
    participant C as Client
    participant M as MCP Server
    participant R as Resource

    C->>M: MCP Request
    M->>R: Resource Request
    R-->>M: HTTP 401 Unauthorized
    M->>C: HTTP 401 Unauthorized
    Note over C: Generate code_verifier and code_challenge
    C->>B: Open browser with authorization URL + code_challenge
    B->>M: GET /authorize
    Note over M: User logs in and authorizes
    M->>B: Redirect to callback URL with auth code
    B->>C: Callback with authorization code
    C->>M: Token Request with code + code_verifier
    M->>C: Access Token (+ Refresh Token)
    C->>M: MCP Request with Access Token
    Note over C,M: Begin standard MCP message exchange
```

### Questions


## Docker MCP OAuth Model
Key points:
- We implement the OAuth flows, nobody else has to.
- Via a developer portal, 3rd parties can add OAuth clients (config with client_id, client_secret, redirect_uri).
- We provide a secure and easy way to store and access secrets.

### Pre-authorization (`docker mcp auth notion my-server`)
```mermaid
sequenceDiagram
    participant A as Authorisation Server
    participant B as User-Agent (Browser)
    participant C as CLI
    participant P as DD Auth API<br>(handles all OAuth flows)
    participant SM as secret management API<br>(handles secret storage+access)
    participant M as MCP Server
    participant R as Resource

    C->>P: docker mcp auth notion my-server
    Note over A,P: Auth Phase (outcome: access/refresh token)<br>(can be local/local+partially hub/fully hub)
    P->>B: Open browser with authorization URL + code_challenge
    B->>A: login
    A->>B: consent
    B->>P: custom protocol callback with authorization code
    P->>A: Token Request with code + code_verifier
    A->>P: Access Token (+ Refresh Token)
    Note over P,SM: Secret Management Preparation/Setup Phase
    P->>SM: create policy 'notion'
    P->>SM: add 'my-server' to policy 'notion'
    P->>SM: set secret 'notion-access-token' with policy 'notion'
    Note over SM,R: Do MCP activity (eg requests to resources that require auth)
    M->>M: mount secret<br>x-secret:notion=/access-token/file.txt
    M->>SM: read access token from file
    SM->>M: grant access to token
    M->>R: Resource Request
    R-->>M: Data
```


### Handling 401 Unauthorized
Note: Option 2 is the most elegant and safest solution. Neither the MCP server nor the MCP client need to know about all the details of the OAuth flow.
```mermaid
sequenceDiagram
    participant C as MCP Client
    participant G as Gateway<br/>mcp/docker
    participant P as DD Auth API<br>(handles all OAuth flows)
    participant SM as secret management API<br>(handles secret storage+access)
    participant M as MCP Server
    participant R as Resource

    C->>G: MCP Request
    G->>M: MCP Request
    M->>SM: read access token from file
    SM->>M: grant access to token
    M->>R: Resource Request
    R-->>M: HTTP 401 Unauthorized
    Note over P,M: Option 1: MCP servers can trigger refresh
    M->>P: refresh
    Note over G,M: Option 2: The gateway can trigger refresh
    M->>G: HTTP 401 Unauthorized
    G->>P: refresh
    Note over C,M: Option 3: The client can trigger refresh
    G->>C: HTTP 401 Unauthorized
    C->>P: refresh
    Note over P,SM: Do refresh + propagate new access token
    P->>P: refresh<br>(if refresh token is expired, then re-authenticate)
    P->>SM: set secret 'notion-access-token'
    Note over SM,R: Do MCP activity (eg requests to resources that require auth)
    M->>SM: read access token from file
    SM->>M: grant access to token
    M->>R: Resource Request
    R-->>M: Data
```

### Notes and observations
Docker has some advantages:
- We are already running in the background all the time.
- This allows us to handle secret storage and management seamlessly in the background on the user's machine instead of on our infrastructure.
- We don't need to be an OAuth server to register dynamic clients. 
  The client can use the credentials stored by DD, since we are in a trusted environment (on the user's machine).
  We just need to be a good proxy.
- Usually clients have the problem of not being trusted.
  We also have an advantage here as we can use the Docker Desktop login to provide a secure environment for clients.


#### 2.9.2 Flow Description

[src](https://spec.modelcontextprotocol.io/specification/2025-03-26/basic/authorization/#292-flow-description)

The third-party authorization flow comprises these steps:

1. MCP client initiates standard OAuth flow with MCP server
2. MCP server redirects user to third-party authorization server
3. User authorizes with third-party server
4. Third-party server redirects back to MCP server with authorization code
5. MCP server exchanges code for third-party access token
6. MCP server generates its own access token bound to the third-party session
7. MCP server completes original OAuth flow with MCP client

```mermaid
sequenceDiagram
    participant B as User-Agent (Browser)
    participant C as MCP Client
    participant M as MCP Server
    participant T as Third-Party Auth Server

    C->>M: Initial OAuth Request
    M->>B: Redirect to Third-Party /authorize
    B->>T: Authorization Request
    Note over T: User authorizes
    T->>B: Redirect to MCP Server callback
    B->>M: Authorization code
    M->>T: Exchange code for token
    T->>M: Third-party access token
    Note over M: Generate bound MCP token
    M->>B: Redirect to MCP Client callback
    B->>C: MCP authorization code
    C->>M: Exchange code for token
    M->>C: MCP access token
```

Questions:
- Alano: The diagram implies that the MCP server runs remotely. Is that correct? Because why otherwise would you need the MCP client to be authorized?

#### All details included
```mermaid
sequenceDiagram
    participant B as User-Agent (Browser)
    participant C as MCP Client
    participant M as MCP Server
    participant T as Third-Party Auth Server<br>Google, Notion, etc.

    C->>M: Initial OAuth Request
    M->>C: Auth browser URL
    C->>B: Open browser URL (Redirect to Third-Party /authorize)
    B->>T: request authorization
    T->>B: request user consent
    B->>T: user approval (consent/accept)
    T->>B: auth code in redirect URL
    B->>M: Redirect to MCP Server callback (authorization code)
    M->>T: Exchange code for token
    T->>M: access + refresh token
    M->>M: Store access + refresh token
    Note over M: Generate bound MCP token<br>MCP server becomes an auth server towards MCP client
    M->>B: Proxy auth code (redirect)
    M->>M: Store link between proxy auth code and access+refresh token
    B->>C: Proxy auth code (redirect)
    C->>M: Exchange proxy code for token
    M->>M: Link proxy access+refresh token to earlier 3rd party session
    M->>C: Proxy access+refresh token
```


