package main

import (
	"html/template"
	"net/http"
)

func subcribe(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("index.html")
		checkError(t.Execute(w, nil))
	}
}

func publish(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("publish.html")
		checkError(t.Execute(w, nil))
	}
}