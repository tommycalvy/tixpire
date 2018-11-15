// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"html/template"
	"net/http"
	"io"
	"crypto/md5"
	"time"
	"strconv"
	"google.golang.org/appengine"
	//"log"
)





func login(w http.ResponseWriter, r *http.Request) {
    fmt.Println("method:", r.Method) // get request method
    if r.Method == "GET" {
        crutime := time.Now().Unix()
        h := md5.New()
        io.WriteString(h, strconv.FormatInt(crutime, 10))
        token := fmt.Sprintf("%x", h.Sum(nil))

        t, _ := template.ParseFiles("index.html")
        t.Execute(w, token)
    } else {
        // log in request
        r.ParseForm()
        token := r.Form.Get("token")
        if token != "" {
            // check token validity
        } else {
            // give error if no token
        }
        fmt.Println("username length:", len(r.Form["username"][0]))
        fmt.Println("username:", template.HTMLEscapeString(r.Form.Get("username"))) // print in server side
        fmt.Println("password:", template.HTMLEscapeString(r.Form.Get("password")))
        template.HTMLEscape(w, []byte(r.Form.Get("username"))) // respond to client
    }
}



func indexHandler(w http.ResponseWriter, r *http.Request) {
	// if statement redirects all invalid URLs to the root homepage.
	// Ex: if URL is http://[YOUR_PROJECT_ID].appspot.com/FOO, it will be
  // redirected to http://[YOUR_PROJECT_ID].appspot.com.
  if r.URL.Path != "/" {
  	http.Redirect(w, r, "/", http.StatusFound)
    return
  }
	//tmpl, err := template.ParseFiles("layout.html")
	fmt.Fprintln(w, "Hello, Gopher Network!")
}


func main() {
	appengine.Main()
	//http.HandleFunc("/login", login)
	http.HandleFunc("/", indexHandler)
	//log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("/build/assets/templates"))))
}
