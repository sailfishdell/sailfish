package main

import (
	"log"
	"net/http"

)

func main() {
	log.Println("starting backend")

	h, _ := NewHandler()

	log.Println(http.ListenAndServe(":8080", h))
}
