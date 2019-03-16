// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"github.com/logpacker/PayPal-Go-SDK"
	"encoding/base64"
	"html/template"
	"strconv"
	"net/http"
	"net/url"
	"strings"
	"time"
	"math"
	"os"
)

var tpl *template.Template
var paymentPlanToken string

Parameters {
	Vendor: string
	Event: string
	Variant: string
	Date: params.Get("date")
	TotalDue: params.Get("total-due")
	Qty: params.Get("qty")
}


type Events struct {
	Date:			string
	Price:		string
	Qty:			string
}

type PaymentSchedule {
	Name 			string
	Cycles 		string
	Interval 	string
	Amount 		string
	Fee				string
	Tax 			string
	Dates 		[]string
}

type Checkout struct {
	Vendor 		string
	Event 		string
	Variant 	string
	Date 			string
	TotalDue 	string
	Qty 			string
	Plans 		[]PaymentSchedule
}

func init() {

	tpl = template.Must(template.ParseGlob("templates/*"))

}

func createBillingSchedule(today time.Time, event time.Time, total float64, feePercent float64, taxPercent float64, cycles int, interval int) (*PaymentSchedule, err string) {

	estimated := today.AddDate(0, 0, 7 * interval * (cycles - 1) + 30)
	if (estimated.After(event)) {
		return nil, "Estimated date is after actual date"
	}
	time := event.Sub(today).Hours()
	if (interval == 0) {
		interval = math.Floor((time - 29 * 24) / ((cycles - 1) * 24 * 7))
		if (interval == 0) {
			return nil, "Too many cycles"
		}
	} else if (cycles == 0) {
		cycles = math.Floor((time - 29 * 24) / (interval * 24 * 7))
		if (cycles == 1) {
			feePercent = 0.03
		} else if (cycles == 0) {
			return nil, "Interval is too large"
		}
	}
	dates := make([]string, cycles)
	for i := 0; i < cycles; i++ {
		dates[i] = today.AddDate(0, 0, 1 + interval * i).Format(time.UnixDate)
	}
	total, _ := strconv.ParseFloat(totalDue, 64)

	amount := math.Ceil(total * 100 / cycles) / 100
	fee := math.Ceil(total * feePercent * 100) / 100
	tax := math.Ceil(amount * taxPercent * 100) / 100

	cyclesStr := strconv.Itoa(cycles)
	intervalStr := strconv.Itoa(interval)

	name := cyclesStr + " PAYMENTS OF " + amount " - " + interval + " WEEK INTERVALS"

	ps := PaymentSchedule {
		Name:				name,
		Cycles: 		cyclesStr,
		Interval: 	intervalStr,
		Amount: 		amount,
		Fee: 				fee,
		Tax: 				tax,
		Dates: 			dates,
	}

	return &ps, nil
}

func createPlans(vendor string, events []Events], totalDue float64, feePercent float64, taxPercent float64, planTypes ...string) []PaymentSchedule {
	today := time.Now()
	date := today
	totalDue := 0.0
	dateForm := "2001 January 01"

	// Convert dates of type string to type time.Time
	// Find the furthest date
	// Calculate totalDue
	var eventDate time.Time
	for i := 0; i < len(events); i++ {
		if (events[i].Date == "none") {
			eventDate = today.AddDate(0, 6, 0)
		} else if (vendor == "taste-of-travel") {
			thisDate, _ := time.Parse(dateForm,  strconv.Itoa(today.Year()) + " " + events[i].Date)
			if (thisDate.Before(today)) {
				eventDate, _ = time.Parse(dateForm, strconv.Itoa(today.Year() + 1) + " " + events[i].Date)
			} else {
				eventDate = thisDate
			}
		} else {
			eventDate, _ = time.Parse(dateForm, events[i].Date)
		}
		if (eventDate.After(date)) {
			date = eventDate
		}
		totalDue += strconv.ParseFloat(events[i].Price, 64) * strconv.Atoi(events[i].Qty)
	}

	plans := make([]PaymentSchedule, len(planTypes))
	for _, planType := range planTypes {
  	if (strings.ContainsAny(planType, ":")) {
			nums := strings.Split(planType, ":")
			low := strconv.Atoi(nums[0])
			high := strconv.Atoi(nums[1])
			for i := high; i <= low; i-- {
				plan, err := createBillingSchedule(today, date, totalDue, feePercent, taxPercent, i, 0)
				if (err == nil) {
					plans = append(plans, plan)
					break
				}
			}
		} else if (strings.ContainsAny(planType, "-")) {
			nums := strings.Split(planType, "-")
			low := strconv.Atoi(nums[0])
			high := strconv.Atoi(nums[1])
			for i := high; i <= low; i-- {
				plan, err := createBillingSchedule(today, date, totalDue, feePercent, taxPercent, 0, i)
				if (err == nil) {
					plans = append(plans, plan)
					break
				}
			}
		} else if (strings.ContainsAny(planType, ",")) {
			nums := strings.Split(planType, ",")
			cycles := strconv.Atoi(nums[0])
			interval := strconv.Atoi(nums[1])
			plan, err := createBillingSchedule(today, date, totalDue, feePercent, taxPercent, cycles, interval)
			if (err == nil) {
				plans = append(plans, plan)
			}
		}
  }
	length := len(plans)
	for j := 0; j < length; j++ {
		for (k := j + 1; k < length; k++) {
			if (plans[j].Cycles == plans[k].Cycles && plans[j].Interval == plans[k].Interval) {
				plans = append(plans[:j], plans[j+1]...)
			}
		}
	}
	return plans
}

func createPayPalBillingPlan(event string, variant string, days string, interval string, cycles string, amount string, tax string, fee string, returnURL string, cancelURL string, c *paypalsdk.Client, r *http.Request) string {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "createBillingPlan amount: %s", amount)
	plan := paypalsdk.BillingPlan {
		Name:        "Payment plan for " + event + " - " + cycles + " payments",
		Description: cycles + " payments over the course of " + days + " days for " + event + " - " + variant + ".",
		Type:        "fixed",
		PaymentDefinitions: []paypalsdk.PaymentDefinition{
			paypalsdk.PaymentDefinition{
				Name:              "Payment Plan - " + cycles + " Payments Over " + days + " Days",
				Type:              "REGULAR",
				Frequency:         "WEEK",
				FrequencyInterval: interval,
				Amount: paypalsdk.AmountPayout{
					Value:    amount,
					Currency: "USD",
				},
				Cycles: cycles,
				ChargeModels: []paypalsdk.ChargeModel{
					paypalsdk.ChargeModel{
						Type: "TAX",
						Amount: paypalsdk.AmountPayout{
							Value:    tax,
							Currency: "USD",
						},
					},
				},
			},
		},
		MerchantPreferences: &paypalsdk.MerchantPreferences{
			SetupFee: &paypalsdk.AmountPayout{
				Value:    fee,
				Currency: "USD",
			},
			ReturnURL:               returnURL,
			CancelURL:               cancelURL,
			AutoBillAmount:          "YES",
			InitialFailAmountAction: "CONTINUE",
			MaxFailAttempts:         "0",
		},
	}
	log.Debugf(ctx, "createBillingPlan plan %s", plan)
	resp, err := c.CreateBillingPlan(plan)
	if err != nil {
		log.Debugf(ctx, "Create Billing Plan Error: %s", err)
		log.Debugf(ctx, "Response Err: %s", resp)
	}
	log.Debugf(ctx, "Reponse no err: %s", resp)
	return resp.ID
}

func parseEncodedString(encodedQuery string) (*Parameters, err string) {
	decodedQuery, err := base64.StdEncoding.DecodeString(encodedQuery)
	if err != nil {
		return nil, "Decode String Error: " + err
	}
	if (strings.ContainsAny(string(decodedQuery), "�")) {
		return nil, "Contains Weird Characters"
	}
	parsedQuery, err := url.Parse(string(decodedQuery))
	if err != nil {
		return nil, "Parse URL error"
	}

	params := parsedQuery.Query()

	parameters := Parameters {
		Vendor: params.Get("vendor")
		Event: params.Get("event")
		Variant: params.Get("variant")
		Date: params.Get("date")
		TotalDue: params.Get("total-due")
		Qty: params.Get("qty")
	}
	return &parameters, nil
}


func checkout(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	originalPath := r.URL.String()
	vendorQuery := r.URL.Path[len("/checkout/"):]
	path := strings.Split(vendorQuery, "/")
	encodedQuery := path[1]

	if (qty == "") {
		http.Redirect(w, r, "/", http.StatusFound)
    return
	}

	// num:num for Cycles
	// num-num for interval
	// num,num for interval, cycle
	plans := createPlans(eventDate, totalDue, 0.07, 0.0825, 1:3, 4:4, 4-4)


	v := Checkout {
		Vendor: vendor,
		Event: event,
		Variant: variant,
		Date: date,
		TotalDue: strconv.FormatFloat(totalDue, 'f', -1, 64),
		Qty: qty,
		Plans: plans,
	}

	log.Debugf(ctx, "Checkout Struct: %s", v)
	errr := tpl.ExecuteTemplate(w, "checkout.gohtml", v)
	if errr != nil {
  	log.Debugf(ctx, "Execute Template Error: %s", err)
  }

	// Initialize client
	c, err := paypalsdk.NewClient(ctx, os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_SECRET_ID"), paypalsdk.APIBaseSandBox)
	if err != nil {
    log.Debugf(ctx, "New Client Error: %s", err)
	}
	log.Debugf(ctx, "paypalsdk Client: %s", c)
	// Retrieve access token
	_, err = c.GetAccessToken()
	if err != nil {
    log.Debugf(ctx, "Get Access Token Error: %s", err)
	}

	plan1ReturnPath := []byte("?vendor=" + vendor + "&event=" + event + "&variant=" + variant + "&event-date=" + date + "&amount=" + payPlan1.Amount + plan1DatesString)
	encodedPlan1ReturnPath := base64.StdEncoding.EncodeToString(plan1ReturnPath)
	plan1ReturnUrl := "https://tixpire.appspot.com/thank-you/" + path[0] + "/" + encodedPlan1ReturnPath

	plan2ReturnPath := []byte("?vendor=" + vendor + "&event=" + event + "&variant=" + variant + "&event-date=" + date + "&amount=" + payPlan2.Amount + plan2DatesString)
	encodedPlan2ReturnPath := base64.StdEncoding.EncodeToString(plan2ReturnPath)
	plan2ReturnUrl := "https://tixpire.appspot.com/thank-you/" + path[0] + "/" + encodedPlan2ReturnPath

	plan3ReturnPath := []byte("?vendor=" + vendor + "&event=" + event + "&variant=" + variant + "&event-date=" + date + "&amount=" + payPlan3.Amount + plan3DatesString)
	encodedPlan3ReturnPath := base64.StdEncoding.EncodeToString(plan3ReturnPath)
	plan3ReturnUrl := "https://tixpire.appspot.com/thank-you/" + path[0] + "/" + encodedPlan3ReturnPath

}


func order(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "Entered order function")
	err := r.ParseForm()
  if err != nil {
  	log.Debugf(ctx, "Parse Form Error: %s", err)
  }
	log.Debugf(ctx, "Payment Plan Id: %s", r.PostFormValue("payment-plan"))

	// Initialize client
	c, err := paypalsdk.NewClient(ctx, os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_SECRET_ID"), paypalsdk.APIBaseSandBox)
	if err != nil {
    log.Debugf(ctx, "New Client Error: %s", err)
	}

	// Retrieve access token
	_, err = c.GetAccessToken()
	if err != nil {
    log.Debugf(ctx, "Get Access Token Error: %s", err)
	}

	err = c.ActivatePlan(r.PostFormValue("payment-plan"))
	log.Debugf(ctx, "Activate Plan Error: %s", err)


	agreement := paypalsdk.BillingAgreement {
		Name:        "Payment plan agreement for " + r.PostFormValue("event") + " - " + r.PostFormValue("cycles") + " payments",
		Description: r.PostFormValue("cycles") + " payments over the course of " + r.PostFormValue("total-days") + " days for " + r.PostFormValue("event") + " - " + r.PostFormValue("variant") + ".",
		StartDate:   paypalsdk.JSONTime(time.Now().Add(time.Hour * 24)),
		Plan:        paypalsdk.BillingPlan{ID: r.PostFormValue("payment-plan")},
		Payer: paypalsdk.Payer{
			PaymentMethod: "paypal",
		},
	}
	resp, err := c.CreateBillingAgreement(agreement)
	log.Debugf(ctx, "Create Billing Agreement Response: %s", resp)
	log.Debugf(ctx, "Create Billing Agreement Error: %s", err)


	approvalUrl := resp.Links[0].Href

	token := strings.Split(approvalUrl, "token=")
	paymentPlanToken = token[1]

	http.Redirect(w, r, resp.Links[0].Href, http.StatusFound)

	/*

	type PaymentPlan struct {
		Id string
		Name string
		Checked string
		Days  string
		Amount string
		Tax string
		Fee string
		Interval string
		Cycles string
		StartDate string
		Dates []string
	}

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
		TotalDue string
	}

	payment := Order {
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
		TotalDue: r.PostFormValue("total-due"),
	}
	*/
}

func thankyou(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "Entered Thank You Page")

	// Initialize client
	c, err := paypalsdk.NewClient(ctx, os.Getenv("PAYPAL_CLIENT_ID"), os.Getenv("PAYPAL_SECRET_ID"), paypalsdk.APIBaseSandBox)
	if err != nil {
		log.Debugf(ctx, "New Client Error: %s", err)
	}

	// Retrieve access token
	_, err = c.GetAccessToken()
	if err != nil {
		log.Debugf(ctx, "Get Access Token Error: %s", err)
	}
	log.Debugf(ctx, "Got New Client and Access Token")
	resp, err := c.ExecuteApprovedAgreement(paymentPlanToken)
	if err != nil {
		log.Debugf(ctx, "Decode String Error: %s", err)
	}
	log.Debugf(ctx, "Execute Approved Agreement Response: %s", resp)

	vendorQuery := r.URL.Path[len("/thank-you/"):]
	path := strings.Split(vendorQuery, "/")
	encodedQuery := path[1]
	decodedQuery, err := base64.StdEncoding.DecodeString(encodedQuery)
	if err != nil {
		log.Debugf(ctx, "Decode String Error: %s", err)
	}
	if (strings.ContainsAny(string(decodedQuery), "�")) {
		http.Redirect(w, r, "/", http.StatusFound)
    return
	}
	log.Debugf(ctx, "Encoded Query String: %s", encodedQuery)
	log.Debugf(ctx, "Decoded Query String: %s", string(decodedQuery))
	parsedQuery, err := url.Parse(string(decodedQuery))
	if err != nil {
		log.Debugf(ctx, "Parse Query Error: %s", err)
	}
	params := parsedQuery.Query()
	vendor := params.Get("vendor")
	event := params.Get("event")
	variant := params.Get("variant")
	date := params.Get("event-date")
	amount := params.Get("amount")

	type ThankYou struct {
		Vendor string
		Event string
		Variant string
		Date string
		Amount string
	}

	v := ThankYou {
		Vendor: vendor,
		Event: event,
		Variant: variant,
		Date: date,
		Amount: amount,
	}

	tpl.ExecuteTemplate(w, "thankyou.gohtml", v)

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
    return
  }
	tpl.ExecuteTemplate(w, "index.gohtml", nil)
}


func main() {

	http.HandleFunc("/install", serveInstall)
	http.HandleFunc("/admin", serveAdmin)
	http.HandleFunc("/update", serveUpdate)
	http.HandleFunc("/addCheckout", serveAddCheckout)
	http.HandleFunc("/addProduct", serveAddProduct)
	http.HandleFunc("/getTemplate", serveGetTemplate)
	http.HandleFunc("/checkout/", checkout)
	http.HandleFunc("/order", order)
	http.HandleFunc("/thank-you/", thankyou)
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}



/*
setupFee :=  totalDue * 7 / 100
var plan1Interval string
var plan2Interval string

var totalPlans int
showPlan1 := true
showPlan2 := true
showPlan3 := true
var payPlan1 PaymentPlan
var payPlan2 PaymentPlan
var payPlan3 PaymentPlan





if (days < 56) {
	showPlan1 = false
	showPlan2 = false
	showPlan3 = false
	totalPlans = 0
} else if (days < 70) {
	showPlan2 = false
	showPlan3 = false
	totalPlans = 1
	plan1Interval = "2"

} else if (days < 140) {
	showPlan3 = false
	totalPlans = 2
	if (days > 83) {
		plan1Interval = "4"
	} else {
		plan1Interval = "3"
	}

	if (days > 111) {
		plan2Interval = "4"
	} else if (days > 90) {
		plan2Interval = "3"
	} else {
		plan2Interval = "2"
	}

} else {
	plan1Interval = "4"
	plan2Interval = "4"
	plan3Interval := "4"
	totalPlans = 3
	plan3Cycles := int(math.Floor(days / 28.0))
	plan3Days := (plan3Cycles - 1) * 4 * 7
	plan3Amount := math.Ceil(totalDue * 100 / float64(plan3Cycles)) / 100
	plan3Tax := math.Ceil(plan3Amount * 8.25) / 100

	var plan3Dates = make([]string, plan3Cycles - 1)
	plan3DatesString := "&payment-date=" + startDate.Format(time.UnixDate)
	for i := 0; i < plan3Cycles - 1; i++ {
		plan3Dates[i] = startDate.AddDate(0, 0, plan3Days * (i + 1) / plan3Cycles).Format(time.UnixDate)
		plan3DatesString += "&payment-date=" + plan3Dates[i]
	}


	payPlan3 = PaymentPlan {
		Id: "placeholder",
		Name: "payment-plan-3",
		Checked: "",
		Days: strconv.Itoa(plan3Days),
		Amount: strconv.FormatFloat(plan3Amount, 'f', -1, 64),
		Tax: strconv.FormatFloat(plan3Tax, 'f', -1, 64),
		Fee: strconv.FormatFloat(setupFee, 'f', -1, 64),
		Interval: plan3Interval,
		Cycles: strconv.Itoa(plan3Cycles),
		StartDate: startDate.Format(time.UnixDate),
		Dates: plan3Dates,
	}



	payPlan3.Id = createBillingPlan(event, variant, payPlan3.Days, plan3Interval, payPlan3.Cycles, payPlan3.Amount, payPlan3.Tax, payPlan3.Fee, plan3ReturnUrl, originalPath, c, r)
	log.Debugf(ctx, "payPlan3: %s", payPlan3)
}

if (showPlan1) {
	plan1Days, _ := strconv.Atoi(plan1Interval)
	plan1Days = plan1Days * 7 * 2
	plan1Amount := math.Ceil(totalDue * 100 / 3.0) / 100
	plan1Tax := math.Ceil(plan1Amount * 8.25) / 100
	plan1Dates := []string{startDate.AddDate(0, 0, plan1Days / 2).Format(time.UnixDate), startDate.AddDate(0, 0, plan1Days).Format(time.UnixDate)}
	plan1DatesString := "&payment-date=" + startDate.Format(time.UnixDate) + "&payment-date=" + plan1Dates[0] + "&payment-date=" + plan1Dates[1]
	log.Debugf(ctx, "plan1Amount: %s", plan1Amount)
	log.Debugf(ctx, "plan1Amount: %s", strconv.FormatFloat(plan1Amount, 'f', -1, 64))
	payPlan1 = PaymentPlan {
		Id: "placeholder",
		Name: "payment-plan-1",
		Checked: "checked",
		Days: strconv.Itoa(plan1Days),
		Amount: strconv.FormatFloat(plan1Amount, 'f', -1, 64),
		Tax: strconv.FormatFloat(plan1Tax, 'f', -1, 64),
		Fee: strconv.FormatFloat(setupFee, 'f', -1, 64),
		Interval: plan1Interval,
		Cycles: "3",
		StartDate: startDate.Format(time.UnixDate),
		Dates: plan1Dates,
	}


	log.Debugf(ctx, "payPlan1.Amount: %s", payPlan1.Amount)
	payPlan1.Id = createBillingPlan(event, variant, payPlan1.Days, plan1Interval, payPlan1.Cycles, payPlan1.Amount, payPlan1.Tax, payPlan1.Fee, plan1ReturnUrl, originalPath, c, r)
	log.Debugf(ctx, "payPlan1: %s", payPlan1)
}

if (showPlan2) {
	plan2Days, _ := strconv.Atoi(plan2Interval)
	plan2Days = plan2Days * 7 * 3
	plan2Amount := math.Ceil(totalDue * 100 / 4.0) / 100
	plan2Tax := math.Ceil(plan2Amount * 8.25) / 100
	plan2Dates := []string{startDate.AddDate(0, 0, plan2Days / 3).Format(time.UnixDate), startDate.AddDate(0, 0, plan2Days * 2 / 3).Format(time.UnixDate), startDate.AddDate(0, 0, plan2Days).Format(time.UnixDate)}
	plan2DatesString := "&payment-date=" + startDate.Format(time.UnixDate) + "&payment-date=" + plan2Dates[0] + "&payment-date=" + plan2Dates[1] + "&payment-date=" + plan2Dates[2]

	payPlan2 = PaymentPlan {
		Id: "placeholder",
		Name: "payment-plan-2",
		Checked: "",
		Days: strconv.Itoa(plan2Days),
		Amount: strconv.FormatFloat(plan2Amount, 'f', -1, 64),
		Tax: strconv.FormatFloat(plan2Tax, 'f', -1, 64),
		Fee: strconv.FormatFloat(setupFee, 'f', -1, 64),
		Interval: plan2Interval,
		Cycles: "4",
		StartDate: startDate.Format(time.UnixDate),
		Dates: plan2Dates,
	}



	payPlan2.Id = createBillingPlan(event, variant, payPlan2.Days, plan2Interval, payPlan2.Cycles, payPlan2.Amount, payPlan2.Tax, payPlan2.Fee, plan2ReturnUrl, originalPath, c, r)
	log.Debugf(ctx, "payPlan2: %s", payPlan2)
}


payPlans := make([]PaymentPlan, totalPlans)
for i := 0; i < totalPlans; i++ {
	if (showPlan1) {
		payPlans[i] = payPlan1
		showPlan1 = false;
	} else if (showPlan2) {
		payPlans[i] = payPlan2
		showPlan2 = false;
	} else if (showPlan3) {
		payPlans[i] = payPlan3
		showPlan3 = false;
	}
}
log.Debugf(ctx, "PayPlans: %s", payPlans)
*/
