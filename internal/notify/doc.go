// Package notify sends delivery emails and records notification outcomes.
//
// It owns the notifications table and email sender implementations. It must not
// import another domain package's store/ package; callers pass plain values.
//
// SECURITY: never persist or log a raw delivery token or URL. Delivery links are
// sent inline in the request that creates them, never from a background job.
package notify
