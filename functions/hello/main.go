package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	port := "8080"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Optional artificial delay for chaos testing: ?delay=5 sleeps
		// 5 seconds before responding, giving a real window to kill the
		// pod mid-request and observe what happens to the in-flight call.
		if d := r.URL.Query().Get("delay"); d != "" {
			if secs, err := strconv.Atoi(d); err == nil {
				time.Sleep(time.Duration(secs) * time.Second)
			}
		}
		fmt.Fprintf(w, "hello from mini-faas, pod=%s\n", os.Getenv("HOSTNAME"))
	})
	log.Printf("hello-function listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
