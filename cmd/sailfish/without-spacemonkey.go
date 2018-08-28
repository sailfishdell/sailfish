// +build !spacemonkey

package main

import (
	"log"
	"net/http"
)

func runSpaceMonkey(addr string, handler http.HandlerFunc) {
	log.Println("OPENSSL(spacemonkey) NOT ENABLED IN BUILD, SKIPPING!")
	log.Println("OPENSSL(spacemonkey) Build with \"-tags 'spacemonkey'\" to enable.")
}
