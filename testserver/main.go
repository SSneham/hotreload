package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hot reload updated"))
	})

	fmt.Println("server started")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
