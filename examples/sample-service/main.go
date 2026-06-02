// Command sample-service is a trivial HTTP service used as the deployment target
// for depl-orch's M1 end-to-end pipeline test.
package main

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"os"
)

// greeting builds the response body; kept pure so it is testable without a server.
func greeting(name string) string {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("hello, %s", name)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, greeting(r.URL.Query().Get("name")))
	})
	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
