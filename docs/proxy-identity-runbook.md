# Proxy Identity Runbook

ShipSystem proxy identity integration is for training, demonstration, replay, and audit deployments only. It must not be used to authorize real device control, weapon-control, fire-control, electronic-warfare, radar-control, or tactical engagement workflows.

## Role Model

Use one of these application roles:

- `viewer`: read runs, replay, reports, metrics, and request WebSocket tickets.
- `operator`: viewer access plus run creation/control, abstract training actions, annotations, and run metadata.
- `instructor`: operator access plus scenario, course template, and retention management.
- `admin`: same application permissions as instructor, reserved for deployment policy.

ShipSystem enforces these roles on HTTP mutations. The proxy or identity provider owns user identity and group membership.

## Header Contract

Recommended proxy headers:

```text
SHIP_SIM_AUTH_MODE=proxy
SHIP_SIM_AUTH_USER_HEADER=X-Forwarded-User
SHIP_SIM_AUTH_ROLE_HEADER=X-Forwarded-Role
SHIP_SIM_AUTH_DEFAULT_ROLE=viewer
SHIP_SIM_AUTH_ROLE_MAP=
```

Role assignment priority:

1. Trusted role header, if present.
2. `SHIP_SIM_AUTH_ROLE_MAP`, for example `alice=instructor,bob=operator`.
3. `SHIP_SIM_AUTH_DEFAULT_ROLE`, recommended `viewer` in production.
4. Backward-compatible `instructor` only when no default role is configured.

## Proxy Hardening

The reverse proxy must remove any client-supplied identity headers before setting trusted values. For example:

```nginx
proxy_set_header X-Forwarded-User "";
proxy_set_header X-Forwarded-Role "";
proxy_set_header X-Forwarded-User $oidc_user;
proxy_set_header X-Forwarded-Role $shipsim_role;
```

Prefer deriving `$shipsim_role` from identity-provider groups outside ShipSystem. Keep group-to-role policy in the identity platform or proxy configuration repository, not in browser code.

Do not expose the ShipSystem app directly to the public internet in `proxy` auth mode. Only the trusted proxy should reach the app port.

## Validation

After deployment, verify each representative role:

```powershell
Invoke-RestMethod -Uri https://training.example.com/api/session
```

Expected response fields:

- `authenticated: true`
- `user_id`: the trusted proxy user id
- `role`: one of `viewer`, `operator`, `instructor`, `admin`
- `role_source`: `header`, `map`, or `default`
- `permissions`: descriptive UI capability flags

Then verify authorization with safe simulator-only calls:

- `viewer`: `GET /api/runs` succeeds, `POST /api/runs` returns `403`.
- `operator`: `POST /api/runs` succeeds, `POST /api/scenarios` returns `403`.
- `instructor` or `admin`: managed scenario/course-template mutations succeed when payload validation passes.

## Audit Notes

Request logs include `user_id`, `role`, `run_id`, status, and duration. Persisted audit logs record training-domain actions such as scenario changes, course template changes, run metadata updates, annotations, training actions, and report exports.

Logs and audit payloads must not include bearer tokens, raw Authorization headers, proxy cookies, or WebSocket tickets.
