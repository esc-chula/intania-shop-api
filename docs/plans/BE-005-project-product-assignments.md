# BE-005 — Project product assignments and prices

## Contract

`PUT /projects/{project_id}/products` accepts the final selection in
`items`. Each item identifies `(product_id, variant_id)` and supplies a
non-negative THB price with exactly two decimal places. The response is the
fully resolved assigned catalogue. An empty `items` array clears selection.

## Design

- Migration `00004` creates `project_products`. A null-safe expression unique
  index prevents duplicate variantless items.
- The service rejects malformed IDs, duplicate identities, missing `items`, and
  out-of-range/non-two-decimal THB prices before opening a database mutation.
- The repository locks the project, rejects completed projects, validates every
  requested sellable item, then deletes and inserts within one transaction.
  Therefore a bad item cannot leave partial replacement behind.
- A null variant is accepted only when the product has no variants. A non-null
  variant must belong to its product.
- Product and variant rows are only referenced, never deleted or stock-mutated.
- Promotion items reference a stable `project_product_id`; removing or
  repricing a referenced item is rejected. Unrelated additions and an identical
  retry remain safe and idempotent.

## Verification

1. Unit tests cover format, duplicate, project ID, handler mapping, and role
   protection.
2. Integration tests (with `TEST_DATABASE_URL`) seed products/projects and
   verify variantless and variant assignment, replacement, rollback, and
   completed-project conflict.
3. Run `make test`, `make test-integration`, and `make docs-check`.

## Manual API smoke test

With an admin bearer token and a non-completed project:

```sh
curl -X PUT "$API/projects/1/products" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"product_id":1,"variant_id":null,"project_price":"299.00"}]}'
```
