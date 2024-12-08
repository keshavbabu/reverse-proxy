package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			fmt.Println("error dumping request:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println(string(b))
		w.Write([]byte("ds1"))
	})

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}
	server.ListenAndServe()
}
