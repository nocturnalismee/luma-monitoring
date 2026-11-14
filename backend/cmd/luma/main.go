// Command luma is the LUMA Monitoring server: a single binary that serves the
// agent ingest endpoints, the REST API and the embedded frontend.
//
// ASSUMPTION(T-000): this is a bootstrap stub so that `go build ./...` and
// `go test ./...` are green for T-000. The real server (config loading, slog,
// graceful shutdown, /healthz, middleware) is implemented in task T-002 per
// docs/TASKS.md.
package main

import "fmt"

func main() {
	fmt.Println("luma: not yet implemented (see task T-002)")
}
