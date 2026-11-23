// Package delivery is the client-facing side: resolve a gallery link, approve, unlock after payment.
// Clients have no account; access is by magic link only. (Not started: slice 3.)
//
// Boundary: do not import another domain package's store/ package. Call its
// exported service interface instead.
package delivery
