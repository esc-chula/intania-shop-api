# GCP production deployment

This repository deploys only from a version tag. Pull requests and pushes to
`main` run CI but **cannot** deploy production.

```text
pull request -> CI only
main push    -> CI only
v* tag       -> Cloud Build approval -> migration job -> Cloud Run release
```

Terraform owns the production foundation. Cloud Build owns image versions,
migrations, and Cloud Run revisions, so Terraform does not roll a running API
back to an old image.

The CI triggers and release trigger deliberately use different service
accounts. A pull request can change `cloudbuild/ci.yaml`, but its CI identity
has no permission to deploy, read application secrets, or write images.

## 1. Finish the one-time state bootstrap

The state bucket already exists: `gs://intania-shop-510714-tfstate`. Enable
versioning and confirm it before using it as Terraform's remote backend:

```bash
gcloud storage buckets update gs://intania-shop-510714-tfstate --versioning
gcloud storage buckets describe gs://intania-shop-510714-tfstate \
  --format='yaml(name,location,versioning,uniform_bucket_level_access)'
```

The state bucket itself is intentionally not in Terraform, because Terraform
needs that bucket before its first `init`.

## 2. Decide the three production values before `apply`

Copy the example without committing the resulting file:

```bash
cd infra/terraform
cp production.tfvars.example production.tfvars
```

Replace these values in `production.tfvars`:

| Value | Why it must be real before apply |
| --- | --- |
| `sql_tier` | This creates the paid Cloud SQL machine. The example is a modest 1 vCPU / 3.75 GB baseline, not a free-tier setting. |
| `cors_allowed_origins` | Only the real HTTPS frontend origins should call this API from a browser. |
| `frontend_callback_url` | Exact frontend route that receives the short-lived OAuth code, for example `https://app.example.com/auth/callback`. It must be one of the allowed frontend origins. |
| `google_redirect_url` | For the initial bootstrap use the HTTPS placeholder in the example. The final value must exactly match an authorized redirect URI in the Google OAuth client. |

## 3. Connect GitHub to Cloud Build

In the GCP Console, open **Cloud Build -> Repositories**, install/authorize the
Google Cloud Build GitHub App for `esc-chula/intania-shop-api`, and choose the
production project. This is an interactive GitHub-admin authorization; do not
create or store a personal access token in Terraform.

Also add a GitHub ruleset for `v*` tags: restrict tag creation to the release
managers (and require signed tags if the team uses them). Cloud Build approval
is a second control; it should not be the only guard on who can create a
production release candidate.

## 4. Create the foundation

From `infra/terraform`, run:

```bash
terraform init \
  -backend-config='bucket=intania-shop-510714-tfstate' \
  -backend-config='prefix=intania-shop-api/production'

terraform plan -var-file=production.tfvars -out=tfplan
terraform apply tfplan
```

Review the plan carefully. It creates a private uploads bucket, Artifact
Registry, service accounts and least-privilege IAM, four empty Secret Manager
secrets, PostgreSQL 16, database `intania_shop`, backups/PITR, and the three
Cloud Build triggers. Cloud SQL has both Terraform and platform deletion
protection enabled.

At this stage **no secret values** have been put in Terraform state or Git.

## 5. Add database user and Secret Manager values

Create a dedicated database user in Cloud SQL (Console or `gcloud sql users`)
with a long generated password stored in the team's password manager. Do not
use `postgres` for the application.

The application connects through Cloud Run's Cloud SQL socket, so add this
exact secret value to Secret Manager:

```text
postgres://APP_USER:URL_ENCODED_PASSWORD@/intania_shop?host=/cloudsql/CLOUD_SQL_CONNECTION_NAME&sslmode=disable
```

Use the `cloud_sql_connection_name` Terraform output for the placeholder.
Then add values to these secrets in Secret Manager:

| Secret ID | Value |
| --- | --- |
| `intania-shop-api-database-url` | The socket-based database URL above |
| `intania-shop-api-jwt-secret` | A new randomly generated value of at least 32 characters |
| `intania-shop-api-google-client-id` | Google OAuth client ID |
| `intania-shop-api-google-client-secret` | Google OAuth client secret |

Never paste these values into a shell history, Git, Cloud Build YAML, or
Terraform variables.

## 6. First release

1. Create/configure the Google OAuth web client. The initial placeholder
   redirect is only to let Cloud Run start; do not test Google login yet.
2. Merge the deployment files to `main`. The `main` trigger should finish as
   CI-only.
3. Create and push an annotated version tag, for example:

   ```bash
   git tag -a v1.0.0 -m 'First production release'
   git push origin v1.0.0
   ```

4. In **Cloud Build -> History**, approve the pending release. It runs CI,
   builds an immutable image tagged with the commit SHA, runs the migration job,
   deploys Cloud Run, then calls `/health`.
5. Copy the resulting Cloud Run service URL, update
   `google_redirect_url` to `https://SERVICE_URL/auth/google/callback`, add
   that exact URI to the OAuth client's **Authorized redirect URIs**, and set
   `frontend_callback_url` to the real frontend route that reads `?code=...`.
   Run `terraform apply -var-file=production.tfvars` again. Push and approve
   the next version tag. Google login is ready only after this second release.

The frontend callback must immediately `POST /auth/exchange` with JSON
`{"code":"..."}`. The code is stored only as a hash, expires after five
minutes, and can be exchanged once; the API response contains the JWT and user.

Cloud Run is public at the platform layer for OAuth callbacks. API access still
uses the application's JWT and ADMIN authorization; do not make the uploads
bucket public merely to display sensitive payment evidence.

## Operational notes

- Roll back application code by routing traffic to a previous healthy Cloud Run
  revision. Database migrations are forward-only and require a separate,
  reviewed data-repair plan if a schema rollback is needed.
- A Cloud Run `/health` failure commonly means the database URL, Cloud SQL IAM,
  or database user has not been configured correctly.
- The release trigger intentionally requires a `v*` tag and human approval;
  merging `main` alone cannot change production.
