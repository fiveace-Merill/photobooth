// Package deliveries owns the photographer delivery flow and the public
// gallery-link access path.
//
// Invariants:
//   - Gallery tokens are 32 random bytes, base64url; only SHA-256 hashes are stored.
//   - Raw tokens are returned once (create/regenerate) and are never stored or logged.
//   - resolve_gallery_link is the only pre-tenant lookup. After resolve, all reads
//     run inside database.WithTenant.
//   - Invalid, revoked, and expired tokens return identical 404 responses.
//   - Locked preview payloads never include original URLs/keys; original access is
//     checked on every request.
//   - Manual mark-paid is outside the payments/ledger/commission system.
package deliveries
