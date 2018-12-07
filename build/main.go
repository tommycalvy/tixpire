// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"net/http"
	"html/template"
	"log"
	"io/ioutil"
	"url"
)

var tpl *template.Template

func init() {
	tpl = template.Must(template.ParseGlob("templates/*"))
}


func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
    return
  }
	tpl.ExecuteTemplate(w, "index.gohtml", nil)
}

func eventSearch(w http.ResponseWriter, r *http.Request) {
	rootUrl := "https://app.ticketmaster.com/discovery/v2/events.json?"
	apiKey := "apikey=YoEXzVKdkKvHHtivISGCzEJrrtfMYGxE"
	if r.Method == "POST" {
		searchInput := r.Form["userSearchTerms"]
		keyword := "keyword=" + url.PathEscape(searchInput)
		url := rootUrl + searchInput + keyword
		resp, err := http.Get(url)
		if err != nil {
			log.Fatal(err)
		} else {
			data, _ := ioutil.ReadAll(response.Body)
			log.Println(string(data))
			tpl.ExecuteTemplate(w, "searchResults.gohtml", data)
		}
	}


}


func main() {
	http.HandleFunc("/eventSearch", eventSearch)
	http.HandleFunc("/", indexHandler)
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	appengine.Main()
}
