# 08 · Security

This covers how services authenticate callers, keep tenants apart, handle secrets and cryptography, treat input and outbound calls, and keep dependencies patched.

The multi-tenant rules also follow [security-audit.md](security-audit.md), the prompt for running a security audit. Use its classes as a checklist when writing code, and the whole prompt when auditing (SEC-26).

A *lead* looks wrong but hasn't been proven by a test or scripted request. It isn't a finding until it is (SEC-26).

The short version: no hard-coded secrets, no secrets in logs, no tenant IDs from request bodies. The rules below add how each one is enforced.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| SEC-1 | API keys have at least 256 random bits, are stored as a peppered HMAC, and are checked for status and expiry on every request. A cached auth result lives at most 60 s and is evicted on revoke. | review, shared test |
| SEC-2 | Access tokens are short-lived and fully validated. Refresh tokens are opaque, stored hashed, rotated on use, and revocable. Session tokens never appear in URLs. | review |
| SEC-3 | Passwords use Argon2id through one package. Login takes the same time for unknown users, and is rate-limited before authentication in a shared store. | review |
| SEC-4 | OAuth and OIDC flows use PKCE, and state that's single-use, short-lived and bound to the browser, user and resource. Verifiers and client secrets stay on the server. JWKS and token responses are size-bounded. | review |
| SEC-5 | Tenant and org come only from the principal. Middleware checks the parent resource in the URL first. Tenant-data repository methods take a typed scope; an empty scope is an error, never "all". | arch test, review |
| SEC-6 | Permissions are deny-by-default, checked per route group, and denials are audited. Config, secrets, members and roles need the right role, not just membership. | server test |
| SEC-7 | Secrets are loaded once, into a type that redacts itself. Outside development, config validation rejects dev defaults, dev-only adapters and disabled TLS checks. | config test |
| SEC-8 | Never log tokens, links that contain tokens, request bodies, or personal data. Log IDs. | the handler's `ReplaceAttr`, review |
| SEC-9 | Encryption at rest is AEAD through one helper: random nonce, AAD that binds the ciphertext to its owner, versioned envelope, KMS data keys. | review |
| SEC-10 | Webhook signatures are HMAC-SHA256 over timestamp and body, verified in constant time within a ±5 min window. A missing secret is a failure, never a skip. | shared test |
| SEC-11 | `crypto/rand` for anything an attacker could guess. `math/rand/v2` only for jitter. | gosec G404 |
| SEC-12 | Every server and upload route sets a body limit. Uploads check size and sniffed type. | review |
| SEC-13 | Customer-supplied URLs go through the shared SSRF client. Every HTTP client has a timeout, a bounded body read, and no redirects unless needed. | depguard, review |
| SEC-14 | TLS verification is never switched off in production. | gosec G402, config test |
| SEC-15 | Values are bound as parameters. Identifiers come from code. JSON, SQL, CQL and Lua are never built with `fmt`. | `constquery`, forbidigo, gosec |
| SEC-16 | Clients see only messages written for clients. Authentication failures get one generic message. | golden test |
| SEC-17 | Every server sends security headers, names its CORS origins, and sets read, write and idle timeouts and a connection limit. | server test |
| SEC-18 | `govulncheck` blocks CI. The Go patch version is pinned. Test infrastructure never ships in production packages. | CI |
| SEC-19 | Every ID in a path, body or payload is checked against the caller. Another tenant's object gets the same answer as a missing one. | shared two-tenant test |
| SEC-20 | Each kind of token does one job, and every parser checks the kind. Halfway through sign-in, the caller can only finish signing in. Sensitive actions ask for verification again. | shared test |
| SEC-21 | Invitations need the invited, verified email. OAuth links to an existing account only on a verified provider email. Claims and redemptions are atomic. | shared test, review |
| SEC-22 | Typed request bodies list what a client may edit. Server-owned fields are never read from client JSON. | review |
| SEC-23 | Identifiers that route inbound traffic are unique across tenants, proven with the provider, and enforced with a partial unique index. | migration test, review |
| SEC-24 | Capability tokens and links are bound to their tenant and resource, expire, and are redeemed atomically when single-use. | shared test |
| SEC-25 | Secrets stay out of URL errors, comparisons are constant-time, and GET endpoints don't return secrets to roles that don't need them. | review |
| SEC-26 | Security audits follow `security-audit.md`: read-only until findings are approved, and a finding counts only once it's proven. | review |
| SEC-27 | The project's primary dependencies are checked against attack publications, exploited-vulnerability lists and CVEs at least every 7 days. Their own dependencies aren't looked up. `security-checks.md` records each run, and the next work to start runs any check that's due. | review |

---

## Authentication

### SEC-1 · API keys

**Rule.**
- **Format:** `<prefix>_<env>_<id>.<secret>`, where the secret is at least 32 bytes from `crypto/rand`, base64url-encoded. The prefix and environment make leaked keys easy to recognise and scan for. The ID lets lookup avoid scanning.
- **Storage:** `HMAC-SHA256(pepper, key)` plus the pepper's version. During a pepper rotation, both the current and the previous pepper are accepted. Unsalted SHA-256 is acceptable only for keys with at least 256 random bits.
- **Checking:** compare with `hmac.Equal`, or look up by hash. Check the key's status and expiry on every request.
- **Caching:** an auth result is cached for at most 60 s, never beyond the key's expiry, and evicted after the revoke commits.

**Why.**
- **Rotation.** A check that requires the stored pepper version to equal the current one, with no previous pepper accepted, invalidates every key the moment the pepper is bumped. Accept both during the rotation.
- **Caching.** Caching auth results in a shared store such as Redis for min(60 s, expiry), and evicting after commit on revoke, is the caching to copy.
- **Expiry.** A key with no expiry stays valid until someone revokes it, and the expiry check has nothing to catch. Give keys an expiry.

### SEC-2 · Human sessions

**Rule.**
- **Access tokens** live 15 minutes at most. They are parsed with the algorithm, issuer and audience pinned, and with expiry required:

  ```go
  jwt.Parse(raw, keyFunc,
  	jwt.WithValidMethods([]string{"EdDSA"}),
  	jwt.WithIssuer(cfg.Issuer),
  	jwt.WithAudience(cfg.Audience),
  	jwt.WithExpirationRequired(),
  )
  ```

- **Refresh tokens** are opaque random values, stored hashed on the server, and rotated on every use. Reusing an old one revokes the whole session. Each refresh token is bound to the user's credential version.
- **Revocation.** Logout, a password change or a role change takes effect on the next request. Do it with a per-request version check.
- **Session tokens never appear in URLs.** That includes query strings, redirects and links. Return them in the response body, or set them as `HttpOnly; Secure; SameSite` cookies. A single-purpose link, such as a password reset or an export download, is different (SEC-24).

**Do.** Revocation in one indexed query per request:

```go
const q = `SELECT EXISTS(SELECT 1 FROM admin_users WHERE id=$1 AND status='ACTIVE' AND admin_role=$2 AND auth_version=$3)`
```

**Don't.** Refresh tokens are stateless 7-day JWTs. They are never stored and never rotated, and there is no logout route. A password reset can't end an existing session. The OAuth callbacks also put both tokens in the redirect URL, so they end up in browser history, `Referer` headers and proxy logs:

```go
"?access_token=" + url.QueryEscape(result.AccessToken) +
"&refresh_token=" + url.QueryEscape(result.RefreshToken)
```

**Also:**
- One auth middleware makes the session check and another skips it, so a revoked session keeps working on the routes behind the second. Make the same check in every middleware that accepts a session.
- A validator that accepts any HMAC algorithm and checks no issuer or audience accepts any token signed with that key, whatever it was issued for. Pin the algorithm, issuer and audience, as above.
- A parser that doesn't require an expiry accepts a token that never expires. Require one with `jwt.WithExpirationRequired()`.

### SEC-3 · Passwords and login

**Rule.**
- Hash passwords with Argon2id through one shared package, with a fixed minimum cost. Rehash on login when the stored parameters are below the current ones, using a check such as `NeedsRehash`.
- When the user doesn't exist, still hash a dummy password, so the response takes the same time.
- Rate-limit login, password reset and signup **before** authentication, by IP and by account. Keep the counters in a shared store, never in process memory.
- Every authentication failure returns one generic message.

**Why.**
- If an unknown email returns before any hashing, the response time reveals which emails have accounts.
- With no rate limit on login, forgot-password or signup, nothing slows an attacker who tries one password after another.
- A limiter that lives in process memory counts separately in each process, and a map that never shrinks keeps growing.
- A database-backed limiter, which runs before auth and is keyed by a trusted IP, is the one to copy.

### SEC-4 · OAuth, OIDC and third-party connect flows

**Rule.** For every start and callback pair:
- **Bind the state** to the browser that started the flow (a nonce in an `HttpOnly; Secure; SameSite=Lax` cookie, compared in constant time), and to the user, tenant and resource it's for.
- **Make the state single-use and short-lived**, and consume it atomically.
- **Use PKCE.** The verifier and client secrets stay on the server, out of the state and out of any URL.
- **Check every ID in the start request against the caller.** A tenant or resource ID in that request could otherwise select another tenant's app credentials.
- **A connection that should only come from the callback** can't be created through a generic "create connection" endpoint with a client-supplied identity and tokens.
- Refresh the JWKS when a token arrives with an unknown `kid`.
- Bound the size of every JWKS and token response (SEC-13).

**Do.** One statement rejects a replay, an expired state and a different browser, and the verifier stays in the database:

```go
const q = `DELETE FROM admin_oidc_logins WHERE state_hash=$1 AND browser_hash=$2 AND expires_at>now() RETURNING code_verifier`
```

**Do.** PKCE, state bound to a cookie, algorithms limited to RS256/384/512, audience and expiry checked, and a 5-minute JWKS cache with forced refresh on an unknown `kid`.

**Don't.** The state is single-use and lasts 10 minutes, which is right, but it isn't tied to a browser, and there is no PKCE. An attacker can start a sign-in with their own account, stop before the callback, and send that callback link to a victim. The victim is then signed in as the attacker, and anything they enter goes into the attacker's account.

### SEC-20 · One token, one job

**Rule.**
- **Each kind of token does one job, and every parser checks the kind.** That covers session, refresh, 2FA challenge, reset, OAuth state, portal, invitation and impersonation tokens. A token of one kind is never accepted as another. Separate signing keys or audiences are best. If kinds share a key, each parser must require a type claim.
- **Halfway through sign-in** (password done, 2FA pending), the caller can only finish signing in. They can't disable 2FA, change the email or password, switch tenant, or get a full session.
- **Sensitive actions ask again.** Disabling 2FA, changing the email or password, and deleting an account need the current password or second factor.
- **Sessions end when they should.** Refresh tokens are rotated and revocable, and sessions end on logout and password change (SEC-2).

**Do.** Access, refresh and portal tokens all use `JWTSecret`, so this check is what stops a refresh token working as a session. Portal tokens have no `token_type`, so it rejects them too, and `ValidatePortalToken` requires its own scope:

```go
if claims.TokenType != auth.TokenTypeAccess {
	return httperr.New(1793625361450, httperr.UnauthorizedErr, "token is not an access token").WithMsg("invalid or expired token")
}
```

The risk that remains is that one forgotten check on a new parser would let a token through. A verify function that can't be called without naming an audience and an issuer makes that harder.

**Do.** Changing a password checks the current one first.

**Test.** A shared test sends every kind of token to the session middleware, and expects a 401 for everything except a session token.

### SEC-21 · Sign-up, invitations and account linking

**Rule.**
- **Accepting an invitation needs the invited, verified email address**, on every path: email sign-up with an invite token, OAuth sign-up through an invite, and accepting while signed in.
- **No token value gets past a gate.** Even an invalid one mustn't bypass a disabled sign-up.
- **OAuth sign-in checks the provider's `email_verified`** before it links to an existing account by email, accepts an invitation, or marks the email as verified. If an unverified provider email links automatically, whoever controls that provider account takes over the real account.
- **Claims and transfers** (claiming a demo, transferring a project, taking ownership) need a role check, CSRF protection, a single-use token and an atomic conditional update, so two concurrent claims can't both succeed.

**Do.** One statement marks the invite as used and creates the user, so a second attempt finds nothing. The email comes from the invite row, so the person accepting can't choose it:

```sql
WITH mark_used AS (
	UPDATE admin_invite_tokens
	SET used_at = now()
	WHERE id = $1 AND used_at IS NULL AND expires_at > now()
	RETURNING email, admin_role
)
INSERT INTO admin_users (id, email, password_hash, auth_provider, admin_role, status)
SELECT $2, email, $3, 'EMAIL', admin_role, $4
FROM mark_used
```

**Do.** SSO refuses to take over an account that uses another sign-in method: "an account with this email already exists with a different sign-in method".

**Don't.** A sign-in gets a session for whichever account has that email. Nothing reads a verified flag, so anyone whose provider account shows a victim's address, unverified, signs in as the victim:

```go
existing, err := deps.Repo.GetUserByEmail(ctx, info.Email)
// ...
if err == nil {
	// Existing user — issue tokens using the stored org and role.
```

With no email check at sign-up, invitations have the same hole. They compare the signed-in user's email with the invited one, but that email was never verified, so someone can register an invitee's address and accept their invitation. Compare against a verified address instead.

**Don't.** A signup link is read, checked, used to add the member, and only then marked as redeemed with a plain `UPDATE`. Two requests at the same moment both pass the check. Use a conditional write instead. If your project uses Scylla, `UPDATE … IF redeemed = false` does it.

Parsing `email_verified` isn't enough. A claim that's parsed and never read protects nothing, so check its value.

---

## Authorization and tenant isolation

### SEC-5 · Tenant scope comes from the principal

**Rule.**
- Tenant and org IDs come only from the authenticated principal. Never from a request body, query or header. (A path ID is checked against the principal; see the last bullet.)
- Repository methods that read tenant data take a typed scope, such as `domain.TenantScope{OrgID, TenantID}`, not loose strings.
- An empty scope is an error, never "all tenants".
- Cross-tenant access uses a separate method whose name says so (`…AnyScope`). Only admin handlers with a global permission call it.
- A resource ID in the path that belongs to another tenant returns 404, never 403. A 403 would confirm that the resource exists.
- **Middleware checks the parent resource first.** When a URL names an organisation, product or project, middleware confirms it belongs to the caller's tenant before any role check or handler runs. A role only means something inside the caller's own tenant, and inherited roles (account role to project role) stop at that boundary. This comes first, because fixes to individual routes can't make up for a missing layer.

**Do.** Repository signatures force org and tenant (`WHERE id=$1 AND org_id=$2 AND tenant_id=$3`), and the unscoped variant is visible in its name: `GetByIDAnyAdminScope`.

**Do.** `projectScopeMiddleware` checks that the project belongs to the caller before any `/projects/:project_id` handler runs.

**Do.** `productScopeMiddleware` does the same for products, and runs before the role check. It loads the product using the caller's organisation and stores the result, so the handlers use that record and never load one from the path ID:

```go
rec, err := deps.Repo().GetProductByOrg(c.Context(), targetOrgID, productID)
// ...
keys.SetValidatedProduct(c, rec)
```

An `org_id` that isn't the caller's gets a 404 too. Answering every such ID with a 403, whether or not it exists, reveals nothing, but it should still be a 404.

**Don't.** An empty tenant silently means "every tenant", so isolation depends on every handler resolving its scope correctly:

```go
filter := bson.M{"_id": string(args.RecordID)}
if args.TenantID != "" {
	filter["tenant_id"] = args.TenantID
}
```

**Enforced by** an arch test: an exported repository method whose name doesn't end in `AnyScope` must take a scope parameter.

### SEC-6 · Permissions

**Rule.**
- Every route group declares its required permission or scope. A route without one is rejected by a server test that walks the route table.
- Denials are audited.
- Every change to integration config, secrets, webhooks, billing, members and roles needs the right role, usually admin, not just membership. Compare each route with the strictest route like it.
- Every admin or staff route sits behind both the admin login and the right admin role.
- A role can only be granted to a member of the same tenant.

**Do.** Changing a member's role needs admin, and the member is looked up inside the checked product (SEC-19):

```go
r.Use(userOnlyMiddleware, requireRoleMiddleware(models.AdminRole))

r.Patch("/products/:product_id/members/:user_id", productmemberhandlers.UpdateProductMemberRoleHandler)
```

### SEC-19 · Every ID is checked against the caller

**Rule.**
- **A second ID in the path.** For `/parents/{id}/children/{childId}`, read and write the child with both IDs: `WHERE id = $1 AND parent_id = $2`. If the check has to happen after loading, do it before any data is returned or any write happens. This applies to list endpoints too.
- **IDs in the request body.** A project ID, member ID, address ID or storage key in the body must be checked to belong to the caller.
- **IDs in webhooks and jobs.** Confirm that the object belongs to the tenant the event is for.
- **The same answer.** Another tenant's object gets exactly the same answer as a missing one: same status, same body. A difference tells an attacker what exists.

**Do.** The product comes from the record the middleware already checked (SEC-5), and the member is looked up inside that product:

```go
memberRec, err := repo.GetProductMemberByProduct(ctx, productRec.ProductID, s.Request.Path.UserID)
```

**Don't.** A missing invitation gets a 404. One issued to someone else gets a 403 saying "this invitation was not issued to your email address", so a caller can tell the two apart. Invitation IDs are UUIDs, so the risk is small, but it's the pattern this rule stops.

**Test.** A shared test uses two tenants: tenant A requests tenant B's object, and the response must match a missing object's, byte for byte.

### SEC-22 · Server-owned fields

**Rule.**
- **A request body is a typed struct that lists what a client may edit.** That struct is the allowlist.
- **Free-form JSON stays caller data.** A `map[string]any` such as `metadata` is stored and returned. Nothing server-owned is ever read from it.
- **These are server-owned:**
  - provider resource IDs, routing keys and rate-limit buckets;
  - billing fields and verification status;
  - owner and tenant IDs.
- **Before a destructive or verifying provider call** (deleting a domain or DNS record, starting verification), confirm the resource belongs to this connection, from records the server created or by asking the provider.

**Do.** `PATCH /users/me` can only change these two fields. A client that sends `role` or `org_id` has nothing to bind them to:

```go
type UpdateMeBody struct {
	FirstName string `json:"first_name" validate:"required,min=2,max=50"`
	LastName  string `json:"last_name" validate:"required,min=2,max=50"`
}
```

A `Metadata map[string]any` field described as "Opaque caller metadata" is allowed, as long as it stays opaque.

### SEC-23 · External identifiers

**Rule.** This covers anything the service stores for a tenant or routes by: phone numbers, page and account IDs, mailboxes, domains, wallet addresses and provider resource IDs.
- **Active routing keys are unique across tenants.** Otherwise, one tenant receives another's messages or money. Enforce this with a code check and a partial unique index, and treat a race as a conflict.
- **Prove ownership.** Ask the provider whether the token can access that exact resource. Otherwise, a squatter can block the real owner.
- **Sync, pair and re-import flows** must never reuse or overwrite another tenant's row when they match by external ID.

**Do.** A deposit address can belong to only one live wallet, and archived wallets don't count:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS wallets_active_address_unique
  ON wallets (vendor_mode, network, address, coalesce(address_memo, ''), coalesce(address_tag, ''), coalesce(address_descriptor, ''))
  WHERE status IN ('PENDING', 'ACTIVE', 'SUSPENDED') AND address IS NOT NULL;
```

**Do.** A manual bind asks the provider about the exact resource first, and refuses if it belongs to a different account.

### SEC-24 · Bearer links and capability tokens

**Rule.** Anyone holding one of these can use it: upload and attachment tokens, share links, webhook path tokens, signed URLs, and reset and invitation links. Each one:
- is bound to its tenant, its resource and, where relevant, its user;
- expires;
- is single-use where it should be, and redeemed atomically;
- never appears in logs (SEC-8).

**Do.** An export download link can only fetch that one job in that one project. The signature covers the project, job and expiry, and it's compared with `hmac.Equal`. A fixed prefix means a signature made for another purpose can't be reused here:

```go
func downloadSignature(secret, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("export-download:v1|" + payload))
	return mac.Sum(nil)
}
```

**Do.** The update only matches an active token, so a second attempt changes nothing and fails:

```go
res, err := r.tokens.UpdateOne(ctx,
	bson.M{"_id": tokenID, "status": string(domain.ResetTokenStatusActive)},
	bson.M{"$set": bson.M{"status": string(domain.ResetTokenStatusConsumed), "consumed_at": now.UTC()}},
)
```

---

## Secrets

### SEC-7 · Loading and validating secrets

**Rule.**
- **Load secrets only in `config`**, from files (`*_FILE`) or the environment, into a `Secret` type that can't print itself:

  ```go
  // internal/foundation/secret/secret.go
  type Secret struct{ v []byte }

  func (s Secret) Reveal() []byte               { return s.v }
  func (Secret) String() string                 { return "[REDACTED]" }
  func (Secret) LogValue() slog.Value           { return slog.StringValue("[REDACTED]") }
  func (Secret) MarshalJSON() ([]byte, error)   { return []byte(`"[REDACTED]"`), nil }
  ```

- **Every local default goes through `developmentOnly(env, value)`**, a small helper in `internal/config` that returns `value` only in development. Outside development, config validation rejects:
  - dev defaults and `.local/` paths;
  - dev-only adapters: the logging email sender, fake KMS, local crypto;
  - TLS skip-verify flags;
  - keys and peppers shorter than 32 bytes.

**Why.**
- Without a type that redacts itself, any secret that reaches a log, error or `%v` is printed.
- Config validation only helps if nothing gets around it:
  - A private key or pepper file that defaults to `.local/...` in every environment, because it isn't wrapped in `developmentOnly`, keeps that dev default outside development. Wrap every such default.
  - A pepper loader with no minimum length accepts a short pepper. Reject anything shorter than 32 bytes.
- A check that an HMAC JWT secret is non-empty accepts a secret of any length. Check the length too.

### SEC-8 · What never gets logged

**Rule.** Never log:
- tokens, keys, secrets or signatures;
- links that contain tokens, such as password-reset and signup links;
- request or response bodies;
- email addresses, names or other personal data.

Log the IDs instead: `user_id`, `key_id`, `request_id`.

**Don't.** The "log" email provider writes the whole email, including reset links, to the log. Config accepts `EMAIL_PROVIDER=log` in any environment, so one wrong setting puts bearer tokens in production logs:

```go
logger.InfoT(1793625669329, "email.logged_not_sent", "to", msg.To, "subject", msg.Subject, "body", msg.Body)
```

The same goes for user email addresses in flows such as forgot-password and account setup. Log the `user_id` instead.

### SEC-25 · Secrets in URLs, comparisons and responses

**Rule.**
- **Strip the query string from URL errors** before logging or storing them. Go's HTTP client puts the whole URL in its error text, and a customer-supplied URL may carry a token. This is the error for `http.Get` on a closed port (tested):

  ```text
  Get "http://127.0.0.1:1/hook?token=PLACEHOLDER": dial tcp 127.0.0.1:1: connect: connection refused
  ```

- **Compare secrets with `hmac.Equal` or `subtle.ConstantTimeCompare`**, never `==` or `!=`.
- **Never build a query or SQL by joining client input into a string** (SEC-15).
- **GET endpoints don't return secrets to roles that don't need them.** Show a secret once when it's created, then only a prefix.

**Do.** A helper removes the query string from a `*url.Error` before the error is stored. Its comment explains why: a `health_check_url` is customer-supplied and may carry a token.

**Don't.** A reset token's verifier is compared with `!=`, while a shared `Verify` helper already does this with `hmac.Equal`. The compared value is an HMAC, so it's hard to exploit, but using `Verify` also adds support for the previous pepper during a rotation.

```go
presentedVerifier := hex.EncodeToString(pepper.ComputeVerifier([]byte(s.Request.Body.Token)))
if presentedVerifier != resetToken.TokenVerifier {
```

---

## Cryptography

### SEC-9 · Encryption at rest

**Rule.**
- Encrypt with AEAD (AES-256-GCM) through one shared helper.
- The nonce comes from `crypto/rand`.
- The additional authenticated data (AAD) binds the ciphertext to its owner and field. A ciphertext copied to another row or tenant then fails to decrypt.
- The stored envelope starts with a format version and a key version, so either can change later.
- Data keys come from KMS `GenerateDataKey` with an encryption context, and plaintext keys are zeroed after use.

**Do.** GCM with AAD, KMS data keys with an encryption context, and zeroed plaintext. A per-project data key, stored as `[version][nonce][ciphertext]` with the project ID as AAD, follows the same shape.

### SEC-10 · Webhook signatures

**Rule.**
- **Signing:** `v1=hex(HMAC-SHA256(secret, "<timestamp>.<body>"))`, sent with timestamp, scheme and secret-version headers.
- **Verifying:**
  - accept a timestamp within ±5 minutes;
  - compare with `hmac.Equal`;
  - accept two secrets during a rotation.
- **A missing secret or verification mode is a failure, never a skip.**

**Do.** Constant-time comparison and a replay window.

**Don't:**
- Give a source with no configured verification mode a "none" mode. It then accepts unsigned requests. Treat the missing mode as a failure instead.
- Write a `VerifyWebhookSignature` that returns `nil` when the secret is empty and has no timestamp check. If a function like this has no callers yet, delete it, or fix it before it gets one.

### SEC-11 · Randomness

**Rule.**
- `crypto/rand` (or `rand.Text()`) for anything an attacker could guess: keys, tokens, nonces, IDs that grant access.
- `math/rand/v2` only for jitter and test data. Never import the old `math/rand`.

gosec G404 enforces this.

---

## Input and outbound calls

### SEC-12 · Input limits

**Rule.**
- Every server sets its body limit explicitly. Routes that accept more set their own, larger limit.
- Uploads:
  - cap the size of each file;
  - check the type with `http.DetectContentType` against an allowlist;
  - never trust the client's `Content-Type`.
- The server ignores an inbound `X-Request-ID` and generates every request ID itself (HTTP-6), so no client value reaches `request_id` in the logs.

**Don't.** The whole upload is read into memory and stored with the client's MIME type. There is no per-file size or type check.

### SEC-13 · Outbound HTTP

**Rule.**
- **Customer-supplied URLs** (webhook endpoints, JWKS URLs, receipt URLs) are fetched only through the shared SSRF-safe client, which each project keeps as `foundation/ssrf`. The private-network exception is a config method that returns false outside dev and test.
- **Every `http.Client` has a `Timeout`.** Dial and header timeouts alone don't cover a slow body.
- **Every response body read is bounded** with `io.LimitReader`.
- **Redirects are off** (`CheckRedirect` returns `http.ErrUseLastResponse`), unless a flow needs them.

**Do.** The model to copy:
1. It resolves the host and rejects the request if *any* resolved address is private, loopback, link-local, carrier-grade NAT (CGNAT), reserved or non-global.
2. It then dials that exact IP, which defeats DNS rebinding.
3. It disables redirects and caps response headers at 64 KB.
4. The private-network exception only applies in development and test:

```go
func (e *Env) AllowPrivateOutboundNetworks() bool {
	return e != nil && e.OutboundAllowPrivateNetworks && (e.AppEnv == DevelopmentAppEnv || e.AppEnv == TestAppEnv)
}
```

**Don't:**
- Decode a JWKS fetched from a customer's `jwks_url` with no size limit. Read it through `io.LimitReader`.
- Call `io.ReadAll` on an OAuth provider's response with no limit. Read it through `io.LimitReader` too.
- Follow redirects in a client that calls a customer's URL, even when each hop still uses the safe dialer. Turn them off with `CheckRedirect`, unless the flow needs them.
- Leave a client for an internal service with no overall `Timeout`. Set one on every `http.Client`.

### SEC-14 · TLS verification

**Rule.**
- `InsecureSkipVerify: true` is allowed only in one of two forms:
  - **with a `VerifyConnection`** that still verifies the chain against a pinned CA (a Kafka client's TLS config, for example);
  - **behind a named flag** that config validation rejects outside development (SEC-7).
- Each use carries `//nolint:gosec // <reason>`.

**Why.** A skip-verify setting, such as one for a Redis client, that config validation rejects in production can't reach production by mistake. With no such check, it turns off verification wherever it's set.

---

## Output

### SEC-15 · No injection

**Rule.**
- Bind every value as a query parameter.
- Identifiers (table and column names) come only from a code-owned allowlist.
- Never build JSON, SQL, CQL or Lua with `fmt`. Encode JSON with `encoding/json`; Lua scripts are constants.

**Why.** Parameterised queries and constant Lua scripts don't cover JSON. An error encoder that builds the response body with `fmt.Appendf` breaks on a message such as `invalid BeneficiaryKind %q`. The message contains quotes, so the body becomes invalid JSON. Build the body with `json.Marshal` instead (ERR-7).

**Enforced by** `constquery` (DATA-5), forbidigo on `fmt.(Sprintf|Appendf)` in `httperr`, and gosec G201/G202.

### SEC-16 · What clients see

**Rule.**
- Clients see only messages written for clients. `UseIssueAsMsg` is allowed only for kinds whose messages are created with `NewKind*` in client-safe words (ERR-7).
- Parser, driver and JWT library errors never reach the client.
- Every authentication failure returns the same message: unknown user, wrong password, disabled account, bad token.

**Don't:**
- Return JWT parser errors to the client, as `.UseIssueAsMsg()` on a parser error does. Return the one generic message instead.
- Echo the first line of gofi's error text on a validation path. Write the message for the client instead.

Since v0.10.0, gofi no longer sends raw error text for 5xx responses (HTTP section, upgrade notes), but our own encoders must not reintroduce it.

### SEC-17 · HTTP hardening

**Rule.**
- **One security-headers middleware on every server** (HTTP-6): `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and a `Content-Security-Policy`. Scripts loaded from a CDN carry subresource-integrity (SRI) hashes.
- **CORS lists its origins.** Never `*` by default, and never credentials with a wildcard.
- **Client IP:** behind a proxy, use the `X-Forwarded-For` entry your proxy added, usually the last. Never the first, which the client controls.
- **Every server sets read, write and idle timeouts, and a connection limit.** See "Server limits" below.

**Why.**
- Setting only HSTS on API routes leaves out the rest of the header set. Send the full set on every server.
- A CSP that allows scripts from a public CDN on every path, with no SRI, runs whatever that CDN serves. Add the SRI hashes.
- A CORS origin that defaults to `*` lets every site call the API. Name the origins.
- A client-IP path that trusts the leftmost `X-Forwarded-For` uses a value the client controls, so a client can claim any IP. Use the entry your proxy added.

**Server limits.** Each open connection uses one of the process's open files, and the number of open files is limited. Limits free the connections that have stopped doing anything. These are the results on gofi v0.10.4, with `ContextTimeout` on every route (tested):

| What a client does | `r.Listen`, no limits | With limits |
|---|---|---|
| Stops partway through sending a request | Stays open. The middleware never runs. | 408 after `ReadTimeout` |
| Connects and sends nothing | Stays open | 408 after `ReadTimeout` |
| Holds more stalled connections than the open-file limit | New clients get no answer until the stalled ones leave. The server logs `too many open files; retrying`, then recovers. | Clients over `MaxConnections` get a 503, and the server doesn't run out of files. |
| Finishes a request, then keeps the connection idle | Stays open | Closed after `IdleTimeout` |
| Stops reading a large response | Waits for the client | Closed after `WriteTimeout` |
| Asks for something the handler takes too long to do | 503 from `ContextTimeout` | The same |

Timeout middleware covers only the last row. It starts once a whole request has arrived, so it never sees the other five.

**Do.** Set the limits with `r.Configure`, as gofi's SRV-5 requires, and start the server with `r.Listen`. This needs gofi v0.10.4:

```go
r := gofi.NewRouter()
r.Configure(gofi.Config{
	BodyLimit:      4 << 20,
	ReadTimeout:    30 * time.Second, // your largest upload on your slowest client
	WriteTimeout:   60 * time.Second, // your largest response on your slowest client
	IdleTimeout:    60 * time.Second,
	MaxConnections: 5000, // above your busiest load, below the open-file limit
})
// register routes on r ...

return r.Listen(addr)
```

Shut down with `r.ShutdownWithContext(ctx)`, which stops the server however it was started (tested).

The values above are examples, so choose your own:
- **`ReadTimeout`** is what closes a connection that never sends anything. `IdleTimeout` only starts after the first request, so it doesn't replace it (gofi SRV-5).
- **`MaxConnections`** must be below the process's open-file limit. On a server allowed 200 open files, with `MaxConnections: 30` and 300 stalled clients, 270 of them got a 503 straight away, and requests worked again once the clients left. With no limit, the same 300 clients left new requests unanswered until they went away.
- **`WriteTimeout`** limits the whole response, so it cuts off slow downloads and streams (gofi SRV-5). With 1s, a client that paused got 0.9 MiB of a 50 MiB file. Long exports need a longer limit or a link to object storage.
- **`MaxConnsPerIP`** is optional. Don't set it behind a proxy, where every client shares the proxy's IP (gofi SRV-5).
- **Don't use `DefaultServerOptions()` as it is.** Its comment calls the values an initial experiment. Its 128-connection limit gave a 503 with 128 healthy keep-alive clients connected, and its 1 MiB body limit gave a 413 on a 2 MiB upload.
- **`gofi.NewFasthttpServer(gofi.ServerOptions{…})`** is only for settings `Config` doesn't have, such as `ReadBufferSize` (gofi SRV-4). A `Config` timeout overrides the same option, and a `Config.BodyLimit` above `MaxRequestBodySize` makes `Listen` return an error (tested).

**Don't.** A server that calls `r.Listen` with no `r.Configure` has no timeouts or connection limit. `gofi.NewFasthttpServer()` with no options has the same result. A proxy in front may set some limits, but only for traffic that goes through it.

On gofi older than v0.10.4, running out of open files stops the server for good, even after the clients leave (tested on gofi v0.10.3).

---

## Dependencies

### SEC-18 · Vulnerabilities and supply chain

**Rule.**
- `task check` runs `govulncheck` at a pinned version over what ships, `./cmd/...`, or `./...` for a library. The template runs `./...` because it works with any layout, so a service narrows it to `./cmd/...`. CI fails on any finding that reaches that code.
- The `go` line in `go.mod` (or a `toolchain` line) pins the patch version. Bump it on every Go security release.
- Code that imports testcontainers or docker never compiles into a production binary. Moving it behind a build tag is paused with the other test changes (TEST-14). Check what a production binary imports with `go list -deps ./cmd/...`.
- gosec is on in the shared lint config. Each `//nolint:gosec` carries a reason.

**Why.**
- With no pinned patch version, whatever toolchain is installed locally is used, so `govulncheck` reports standard-library findings that a newer Go patch release has already fixed. A small program using `net/http` on Go 1.27.1 failed with 5 findings, and passed after `go mod edit -toolchain=go1.27.3` (tested). Pin the patch version.
- An ordinary package of test infrastructure that imports testcontainers pulls in containerd, buildkit and docker, and most of a scan's findings can come from them. Keep that code out of what ships.
- Findings also reach production code through common modules, such as `golang.org/x/net` and `golang.org/x/crypto`, and through indirect dependencies, such as an old database driver that a migration library pulls in. Update the module that brings in the fix.
- Running a pinned version, such as `govulncheck@v1.9.0`, from the Taskfile, with the Go patch version pinned as well, is the setup to copy.

---

## Audits

### SEC-26 · Security audits

**Rule.** Run security audits with [security-audit.md](security-audit.md). Hand that prompt to an agent as it is, or follow it yourself. Its ground rules aren't optional:
- **Read-only until approved.** Phases 0 to 3 change no code. Fixes start only after the engineer approves the findings.
- **Nothing real.** No production, no real customer data, and no third-party APIs. Use the repo's mocks, httptest servers or a disposable database.
- **Never print secrets**, in output or in logs.
- **A finding counts only once it's proven**, by a test or scripted request that shows the bad behaviour on the current code. "Looks risky" is a lead. Say plainly when a lead turns out to be safe.
- **Sweep each pattern.** After finding one instance, search the whole codebase for the same pattern before moving on.

The audit runs in order: understand the product, map the attack surface, check each class, report, and stop for approval. Fixes come last, as one focused commit per issue.

**Use the classes when writing code too.** Before adding a route, token, webhook or integration, check the classes that apply:

| Class | What it checks | Rules |
|---|---|---|
| A | The tenant boundary in middleware (check this first) | SEC-5 |
| B | Every route with a second ID, and IDs in bodies and payloads | SEC-5, SEC-19 |
| C | Token confusion and session integrity | SEC-2, SEC-20 |
| D | Sign-up, invitations and account linking | SEC-21 |
| E | OAuth and third-party connect flows | SEC-4 |
| F | Server-owned fields | SEC-22 |
| G | External identifiers | SEC-23 |
| H | Bearer links and capability tokens | SEC-24 |
| I | Role gates on configuration | SEC-6 |
| J | Secret handling and comparisons | SEC-1, SEC-8, SEC-10, SEC-15, SEC-25 |

**Why.** Reading code isn't an audit. An email-linking problem like the one in SEC-21 stays hidden until something looks for that class, and it's still a lead until a test proves it. Bugs like these come in families, so one fix without a sweep leaves the rest in place.

### SEC-27 · Regular security checks

**Rule.** New attacks and vulnerabilities get published whether or not the code changes, so each project checks for them at least every 7 days. An agent isn't a long-running job, and nothing wakes it on a timer. So the check is due by date, and the next piece of work picks it up.

**When it runs.**
- Before any work starts, whether it's a task or a small change, the agent reads `security-checks.md`, beside `tasklist.md`.
- If today is on or after the top row's "Next due" date, or there's no file or no row, the check is due. The agent runs it first, as part of the work. Otherwise it carries on.
- It also runs early if the engineer asks, or if the agent learns of an exploited vulnerability in something the project uses.
- If it can't run, such as with no network, the agent says so under "Warnings" (COL-8) and adds no row, so it stays due.
- The report always has a "Security check" line: what it found, or when the next one is due.

**The file.** The newest run goes at the top. Only the engineer changes the interval. A row is added even when nothing was found, because that's the record of the last run. Here's an invented example:

```markdown
# Security checks

Interval: 7 days

| Run on | Next due | Run by | Looked at | Result |
|---|---|---|---|---|
| 2026-11-04 | 2026-11-11 | Agent, before TRF-017 | 12 primary dependencies, for anything published since 2026-10-28 | 1 finding: `golang.org/x/net` CVE-2023-39325, TRF-018 proposed. 2 ruled out: the affected code isn't called. |
| 2026-10-28 | 2026-11-04 | Agent, before TRF-016 | 12 primary dependencies, for anything published since 2026-10-20 | Nothing found. |
```

The file holds public IDs and task links only. A weakness in the project's own code, such as one a new kind of attack reveals, goes to the engineer. It stays out of the file until it's fixed, because other people can read the repo.

**What it covers.** The project's primary dependencies, and whatever has been published about them since the last run. The first run covers everything still unfixed.

A primary dependency is one the project chose itself: the Go version, a module in `go.mod` without `// indirect`, a package named directly in another manifest, and the database, cache, queue, proxy and base image it runs on. The dependencies of those dependencies aren't looked up. That would take far too long, and they're harder to fix, because a fix usually has to come through the primary dependency anyway. If `govulncheck` reports one that the code calls, it's handled like any other finding.

1. **List them.** This command lists the Go ones, and any newer version:

   ```sh
   go list -m -u -f '{{if not (or .Main .Indirect)}}{{.Path}} {{.Version}}{{with .Update}} -> {{.Version}}{{end}}{{end}}' all | grep .
   ```

   It asks the Go module proxy about each module, so set `GOPRIVATE` for private modules first.
2. **Look each one up in these sources:**

| Source | What it adds |
|---|---|
| `govulncheck`, using the [Go vulnerability database](https://pkg.go.dev/vuln/) | Known vulnerabilities in Go and Go modules, and whether the code calls the affected function |
| [CISA's Known Exploited Vulnerabilities catalog](https://www.cisa.gov/known-exploited-vulnerabilities-catalog) | Vulnerabilities being used in real attacks, in any product |
| [OSV](https://osv.dev) and the [GitHub Advisory Database](https://github.com/advisories) | Known vulnerabilities for one package at one version, including packages that aren't Go |
| [NVD](https://nvd.nist.gov) | The details and severity of a CVE (a public ID for one vulnerability) |
| The dependency's own security advisories and release notes | Security fixes with no CVE. A private library is in no public database, so this is its only source. |
| Security bulletins and published attack write-ups about these dependencies | New kinds of attack with no CVE yet. Compare each with the classes in [security-audit.md](security-audit.md), and search the code for the same pattern. |

These commands cover the first three, with invented inputs for the date, module and version:

```sh
task vuln

curl -s https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json \
  | jq -r --arg since 2026-10-28 '.vulnerabilities[] | select(.dateAdded >= $since) | [.dateAdded, .cveID, .vendorProject, .product] | @tsv'

curl -s https://api.osv.dev/v1/query -d '{"package":{"name":"golang.org/x/net","ecosystem":"Go"},"version":"0.16.0"}' \
  | jq -r '.vulns[].id'
```

3. **Sort every match:**
   - **Finding:** proven. The version in use is affected, and the project reaches the affected code or setting.
   - **Lead:** not proven yet. Say what would prove it.
   - **Ruled out:** say why, such as "the affected function is never called".

   A newer version isn't a finding by itself. Upgrading is only part of this check when a source above names a security problem.

**The ground rules are SEC-26's.** The check changes no code. It tries nothing against production or a third party, and any proof runs locally. It never prints secrets. Look up public packages only, and never send private module names, code or config to a public service.

**What happens to a finding.**
- **Being exploited, and the project reaches it:** stop and tell the engineer straight away (COL-11).
- **Any other finding:** propose a task with a priority (COL-15), and carry on with the current work. A fix goes into the current work only if the engineer says so.
- A finding still open goes under "Warnings" in the report and the completion document (COL-8, COL-9).

**Why.** The databases only list what someone has reported to them. A library can fix a security bug in an ordinary release with no CVE, and a private library is never listed, so each primary dependency's own advisories and release notes are read too. `govulncheck` in `task check` (SEC-18) also only sees Go code. HTTP/2 Rapid Reset shows the gap. CISA's catalog lists it as CVE-2023-44487, under the product "HTTP/2". The Go advisory for the same attack has another ID, CVE-2023-39325, and it says nothing about a proxy in front of the service (tested: looked up both). A date in a file makes the check happen on a project where no job runs on a schedule. Seven days is short enough that a published attack doesn't sit for long, and long enough that the check isn't most of the work.

It isn't part of `task check`, because the build would then start failing on a date, with no code change. Nothing runs while no work is happening, so an idle project isn't checked until its next task.

**How it's checked.** A person checks it in review: the report has no "Security check" line, or the top row was overdue when the work started. No script checks it yet.
