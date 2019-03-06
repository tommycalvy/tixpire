// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	//"github.com/logpacker/PayPal-Go-SDK"
	//"github.com/najeira/hikaru"
	"html/template"
	"net/http"
	//"os"
)

var tpl *template.Template

func init() {

	tpl = template.Must(template.ParseGlob("templates/*"))

}

func checkout(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "checkout function entered")
	params := r.URL.Query()

	vendor := r.URL.Path[len("/checkout/"):]
	log.Debugf(ctx, "Vendor: %s", vendor)
	event := params.Get("event")
	variant := params.Get("variant")
	date := params.Get("date")
	totalDue := params.Get("total-due")
	qty := params.Get("qty")
	log.Debugf(ctx, "totalDue: %s", totalDue)
	//t := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	type Checkout struct {
		Vendor string
		Event string
		Variant string
		Date string
		TotalDue string
		Qty string
	}

	v := Checkout{Vendor: vendor, Event: event, Variant: variant, Date: date, TotalDue: totalDue, Qty: qty}
	log.Debugf(ctx, "Checkout Struct: %s", v)
	err := tpl.ExecuteTemplate(w, "checkout.gohtml", v)
	if err != nil {
  	log.Debugf(ctx, "Execute Template Error: %s", err)
  }
}


func order(w http.ResponseWriter, r *http.Request) {
	//ctx := appengine.NewContext(r)

	//err := r.ParseForm()
  //if err != nil {
  	//log.Debugf(ctx, "Parse Form Error: %s", err)
  //}

	type Order struct {
		FName string
		LName string
		Address string
		Address2 string
		City string
		State string
		Zip string
		Email string
		CardFName string
		CardLName string
		CardNum string
		ExpMonth string
		ExpYear string
		CVV string
	}

	orderInfo := Order {
		FName: r.PostFormValue("firstname"),
		LName: r.PostFormValue("lastname"),
		Address: r.PostFormValue("address"),
		Address2: r.PostFormValue("address2"),
		City: r.PostFormValue("city"),
		State: r.PostFormValue("state"),
		Zip: r.PostFormValue("zip"),
		Email: r.PostFormValue("email"),
		CardFName: r.PostFormValue("cardholderFirstName"),
		CardLName: r.PostFormValue("cardholderLastName"),
		CardNum: r.PostFormValue("cardnumber"),
		ExpMonth: r.PostFormValue("expmonth"),
		ExpYear: r.PostFormValue("expyear"),
		CVV: r.PostFormValue("cvv"),
	}

	tpl.ExecuteTemplate(w, "thankyou.gohtml", orderInfo)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
    return
  }
	tpl.ExecuteTemplate(w, "index.gohtml", nil)
}


func main() {

	// Initialize client
	/*
	c, err := paypalsdk.NewClient(os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_SECRET_ID"), paypalsdk.APIBaseSandBox)
	if err != nil {
    log.Debugf(ctx, "New Client Error: %s", err)
	}

	// Retrieve access token
	_, err = c.GetAccessToken()
	if err != nil {
    log.Debugf(ctx, "Get Access Token Error: %s", err)
	}

	// Create credit card payment
	p := paypalsdk.Payment{
    Intent: "sale",
    Payer: &paypalsdk.Payer{
        PaymentMethod: "credit_card",
        FundingInstruments: []paypalsdk.FundingInstrument{{
            CreditCard: &paypalsdk.CreditCard{
                Number:      "4111111111111111",
                Type:        "visa",
                ExpireMonth: "11",
                ExpireYear:  "2020",
                CVV2:        "777",
                FirstName:   "John",
                LastName:    "Doe",
            },
        }},
    },
    Transactions: []paypalsdk.Transaction{{
        Amount: &paypalsdk.Amount{
            Currency: "USD",
            Total:    "7.00",
        },
        Description: "My Payment",
    }},
    RedirectURLs: &paypalsdk.RedirectURLs{
        ReturnURL: "http://...",
        CancelURL: "http://...",
    },
	}
	_, err = c.CreatePayment(p)
	if err != nil {
    log.Debugf(ctx, "Create Payment Error: %s", err)
	}
	*/

	http.HandleFunc("/install", serveInstall)
	http.HandleFunc("/admin", serveAdmin)
	http.HandleFunc("/update", serveUpdate)
	http.HandleFunc("/addCheckout", serveAddCheckout)
	http.HandleFunc("/addProduct", serveAddProduct)
	http.HandleFunc("/getTemplate", serveGetTemplate)
	http.HandleFunc("/checkout/", checkout)
	http.HandleFunc("/order", order)
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
