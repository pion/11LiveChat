package main

import (
	"html/template"
	"net/http"
)

func web(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("sfu.html")
		checkError(t.Execute(w, nil))
	}
}

func js(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("sfu.js")
		checkError(t.Execute(w, nil))
	}
}
func alice(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("alice.html")
		checkError(t.Execute(w, nil))
	}
}

func bob(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("bob.html")
		checkError(t.Execute(w, nil))
	}
}
