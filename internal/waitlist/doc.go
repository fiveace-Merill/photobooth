// Package waitlist handles public waitlist submissions and admin waitlist management data.
// It is a platform table (no photographer_id, no RLS). Public responses must not leak
// whether a contact is already on the list.
package waitlist
