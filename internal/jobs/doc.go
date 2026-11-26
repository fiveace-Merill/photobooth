// Package jobs hosts background job wiring.
//
// Slice 3a runs River workers inside the API process. The structure is kept
// reusable so a future cmd/worker can register the same workers.
package jobs
