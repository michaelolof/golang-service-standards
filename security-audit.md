# Multi-tenant security audit: find and fix access-control and auth flaws

You are auditing the product in this working directory for security flaws that let one customer reach another customer's data, or let an attacker get past authentication.

## Ground rules

- **Phases 0 to 3 are read-only.** Don't change code until I approve the findings.
- **Don't touch anything real.** No production, no real customer data, and no third-party APIs (payments, email, OAuth providers, messaging platforms). Use the repo's mocks and fakes, httptest servers, or a disposable test database.
- **Never print secrets.** No secrets, tokens or keys in your output or in logs.
- **A finding counts only once it is proven.** Prove it with a test, or a scripted request against a local instance, that shows the bad behaviour on the current code. "Looks risky" is a lead, not a finding. Say plainly when a lead turns out to be safe.
- **Sweep each pattern.** When you find one instance, search the whole codebase for the same pattern before moving on. Bugs in this class come in families.

## Phase 0: understand the product

Work this out from the code before auditing anything:

- **Repositories:** the working directory may be one repo or a folder of several (API, dashboard, widget, workers, crawlers). List each one and its role. Check `git` roots, READMEs, and the docs or plans folders.
- **Stack:** languages, frameworks, database, cache and queues, auth libraries, from manifests (`go.mod`, `package.json`, `requirements.txt`, `Gemfile`), Docker and CI files.
- **Tenancy model:**
  - Find what a tenant is (organisation, workspace, account, customer) and what it owns (projects, sites, apps). Use the schema (migrations, models), the foreign keys to the tenant table, and the auth middleware.
  - Find the roles at each level, how a role is resolved for a request, and whether staff or admin use a separate login.
- **Sign-in methods and integrations:** password, 2FA, OAuth providers, SSO, API keys, and every third-party service the product connects to on a customer's behalf.

Write this up in a few lines at the top of your report. If the tenancy model is genuinely ambiguous (for example, two tables could each be the tenant), say so and ask, rather than guessing.

## Phase 1: map the attack surface

Build these inventories before judging anything:

1. **Every HTTP route.** Method, path, middleware chain (authentication, tenant and role checks, CSRF) and handler. Include admin, public, webhook and internal routes.
2. **Every other entry point.** Provider webhooks, queue and stream consumers, cron jobs, CLI commands, AI/MCP tool handlers, and websocket or SSE endpoints.
3. **Every kind of token or signed value.** This includes:
   - sessions, refresh tokens and 2FA/MFA challenges;
   - password reset, email verification and invitation tokens;
   - OAuth state, impersonation tokens and API keys;
   - webhook path tokens, upload and attachment tokens, and share links.

   For each one, record how it is created, what signs it, and every place that accepts it.
4. **Every external identifier the product stores for a tenant or routes by.** Phone numbers, page and account IDs, email addresses and mailboxes, domains and DNS records, installation and team IDs, and provider resource IDs.

## Phase 2: check each class

### A. Tenant boundary in middleware (check first)
- Does the permission middleware check that the parent resource in the URL (organisation, workspace or project) belongs to the caller's tenant? Or does it only check that the caller has a role somewhere? Test: a user of tenant A requests tenant B's project by ID. Expect 404.
- If roles are inherited (account role → project role), make sure the inheritance applies only inside the caller's own tenant.
- Per-route fixes are worthless while this layer is missing, so it comes first.

### B. Object-level authorization on every route with a second ID
- For `/parents/{id}/children/{childId}`, is the child read and written with `WHERE id = ? AND parent_id = ?`? Or, if it is checked after loading, is that check done before any read is returned or any write happens? Check list endpoints too.
- IDs taken from the request body (a project ID, member ID, address ID or storage key): is each one checked to belong to the caller?
- Webhooks and background jobs that take an ID from a payload: do they confirm the object belongs to the tenant the event is for?
- Another tenant's object must get exactly the same answer as a missing one (same status, same body). A 403 vs 404 difference tells an attacker what exists.

### C. Token confusion and session integrity
- Can any non-session token (2FA challenge, refresh, OAuth state, reset, admin or impersonation) be used as a session? Look for a shared signing secret combined with a session parser that doesn't check a token type or audience. Test every token kind against the session middleware.
- What can someone reach halfway through sign-in (password done, 2FA pending)? In particular, can they disable 2FA, change the email or password, switch tenant, or mint a full session?
- Do sensitive actions (disabling 2FA, changing the email, deleting the account) ask for the right re-verification?
- Are refresh tokens rotated and revoked? Do sessions end on password change and logout?

### D. Sign-up, invitations and account linking
- Does accepting an invitation require the invited, verified email address? Check every path: email sign-up with an invite token, OAuth sign-up through an invite, and accepting while signed in.
- Can any token value, even an invalid one, bypass a disabled sign-up or another gate?
- OAuth sign-in: is the provider's `email_verified` checked before you link to an existing account by email, accept an invitation, or mark the email verified? An unverified provider email that auto-links is an account takeover.
- Claim and transfer flows (claiming a demo, transferring a project, taking ownership) need:
  - a role check and CSRF protection;
  - a single-use token;
  - an atomic conditional update, so two concurrent claims can't both succeed.

### E. OAuth and third-party connect flows
For every start/callback pair:
- **Binding:** is the state tied to the browser that started the flow (a nonce cookie compared in constant time) and to the user, tenant and resource? Is it single-use (consumed atomically) and short-lived? If not, two attacks open up: OAuth CSRF (the victim's account linked into the attacker's tenant) and replay.
- **Secrets:** are the PKCE verifier and client secrets kept out of the state, and out of any URL that can end up in logs or error messages?
- **Start-request IDs:** is any tenant or resource ID in the start request checked against the caller, rather than parsed and trusted? Could it select another tenant's app credentials?
- **Generic create:** can a generic "create connection" endpoint create a connection that should only come from the OAuth callback, with a client-supplied identity (email address, account ID) and tokens?

### F. Server-owned fields (mass assignment)
- For every PATCH or PUT that accepts a JSON object (metadata, settings, config), which fields can the client set? These must be server-owned:
  - provider resource IDs, routing keys and rate-limit buckets;
  - billing fields and verification status;
  - owner and tenant IDs.

  Keep an explicit list of the fields a client may edit.
- Before a destructive or verifying provider call (deleting a domain or DNS record, starting verification), confirm the resource really belongs to this connection: from server-created records, or by asking the provider.

### G. External identifiers: uniqueness, ownership, takeover
- **Inbound routing:** if two tenants register the same phone number, page, mailbox or domain, who receives the other's messages? Active routing keys must be unique across tenants, enforced by a code check plus a partial unique index, with a race treated as a conflict.
- **Proof of ownership:** can a tenant claim an identifier without proving they control it? Verify with the provider that the token can access that exact number, page or account. Otherwise a squatter blocks the real owner.
- **Re-matching flows:** can sync, pair or re-import flows that match existing rows by external identifier reuse or overwrite another tenant's row?

### H. Bearer artifacts and capability tokens
- Upload and attachment tokens, share links, webhook path tokens and signed URLs: is each one bound to its tenant, resource and (where relevant) user? Does it expire? Is it single-use where it should be, and redeemed atomically?

### I. Role gates on configuration
- Does every change to integration config, secrets, webhooks, billing, members and roles require the right role (usually admin), not just membership? Compare each with the strictest equivalent route.
- Is every admin or staff route behind both the admin login and the right admin role?
- Can a role only be granted to a member of the same tenant?

### J. Secret handling and comparisons
- Secrets in query strings that reach logs or error messages, including errors that wrap the request URL.
- Token and secret comparisons with `==` instead of a constant-time comparison.
- Query strings or SQL built by string concatenation from client input.
- GET endpoints returning secrets to roles that don't need them.

## Phase 3: report (stop here for approval)

For each confirmed finding, give:
- an ID and a severity (critical, high, medium or low), with one line on impact, e.g. "any signed-up customer can change tenant B's voice settings";
- the affected routes and files, with line numbers;
- the failing test or request that proves it;
- the proposed fix;
- the behaviour changes it will cause, e.g. who will now get a 404 or 409;
- deploy risks, e.g. a migration that could fail on existing data.

Order by severity, with tenant-boundary findings first. Also list the leads that turned out to be safe, and anything you couldn't verify.

## Phase 4: fix (after approval)

- **One focused commit per issue.** Each contains the test that failed before, the fix, and positive controls showing legitimate users still work: the tenant's own resources, admin flows and normal sign-in.
- **Fix at the shared layer where you can** (middleware, repository scoping), with tests per route.
- **Migrations are additive only.** If a unique index could fail on existing duplicates, check first and stop with a clear message rather than deleting data.
- **Keep existing sessions working.** If you add token types, accept existing tokens of the right shape until they expire, and document how long that window is.
- **Verify live.** After merging, check on a local instance with two tenants: each tenant's own resources still work, and the other tenant's return 404.
- **Sweep again.** Re-run the whole search for every pattern you fixed, to catch stragglers.

If Phase 2 is too much for one pass, split it into five groups: A and B; C and D; E; F, G and H; I and J. Work through them in turn, with at most one sub-agent running at a time. Each sub-agent must prove its findings with tests.
