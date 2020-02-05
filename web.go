package main

import (
	"html/template"
	"net/http"
)

func web(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("index.html")
		checkError(t.Execute(w, nil))
	}
}
