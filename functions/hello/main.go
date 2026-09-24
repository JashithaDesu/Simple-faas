package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := "8080"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from mini-faas, pod=%s\n", os.Getenv("HOSTNAME"))
	})
	log.Printf("hello-function listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
