// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/datastore"
	"github.com/dommmel/go-shopify"
	"github.com/gorilla/sessions"
	"github.com/dsoprea/goappenginesessioncascade"
	"github.com/LarryBattle/nonce-golang"
	//"github.com/logpacker/PayPal-Go-SDK"
	"net/http"
	"net/url"
	"html/template"
	"strings"
	"os"
	"io"
	"io/ioutil"
)

var tpl *template.Template
var app *shopify.App
var api *shopify.API

var (
    sessionSecret = []byte(os.Getenv("SESSION_SECRET"))
    sessionStore = cascadestore.NewCascadeStore(cascadestore.DistributedBackends, sessionSecret)
		tokens map[string]string
		state string
		assetParams interface{}
)

type Shop struct {
	Name string
	Token string
}

func init() {
	tpl = template.Must(template.ParseGlob("templates/*"))

	var key, secret, redirect string

	if key = os.Getenv("SHOPIFY_API_KEY"); key == "" {
		panic("Set SHOPIFY_API_KEY")
	}

	if secret = os.Getenv("SHOPIFY_API_SECRET"); secret == "" {
		panic("Set SHOPIFY_API_SECRET")
	}

	if redirect = os.Getenv("SHOPIFY_API_REDIRECT"); redirect == "" {
		panic("Set SHOPIFY_API_REDIRECT")
	}

	app = &shopify.App {
		RedirectURI: redirect,
		APIKey:      key,
		APISecret:   secret,
	}
	tokens = map[string]string{}
}

func getSession(r *http.Request) *sessions.Session {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "getSession RAN")
	session, err := sessionStore.Get(r, "shopify_app")
	if err != nil {
  	panic(err)
  }
	log.Debugf(ctx, "Returning session")
	return session
}

func serveInstall(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "serveInstall RAN")
	params := r.URL.Query()
	log.Debugf(ctx, "Params error length: %s", len(params["error"]))
	if len(params["error"]) == 1 {
		log.Debugf(ctx, "Install error: %s", params["error"])
	} else if len(params["code"]) == 1 {
		log.Debugf(ctx, "No params error")
		// auth callback from shopify
		log.Debugf(ctx, "state: %s", state)
		if app.AdminSignatureOk(r.URL, state) != true {
			http.Error(w, "Invalid signature", 401)
			log.Debugf(ctx, "Invalid signature from Shopify")
			return
		}
		log.Debugf(ctx, "Admin Signature is ok")
		if len(params["shop"]) != 1 {
			http.Error(w, "Expected 'shop' param", 400)
			log.Debugf(ctx, "Invalid signature from Shopify")
			return
		}
		log.Debugf(ctx, "Only one shop parameter. Good")
		shop := params["shop"][0]
		log.Debugf(ctx, "serveInstall 2nd shop: %s", shop)
		token, _ := app.AccessToken(ctx, shop, params["code"][0])
		// persist this token
		tokens[shop] = token
		shopEntity := Shop {
			Name: shop,
			Token: token,
		}
		if _, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, "Shop", nil), &shopEntity); err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }
		log.Debugf(ctx, "Token succesfully stored")

		// log in user
		session := getSession(r)
		session.Values["current_shop"] = shop
		err := session.Save(r, w)
		if err != nil {
			panic(err)
		}

		log.Debugf(ctx, "logged in as %s, redirecting to admin", shop)
		target := "https://" + shop + "/admin/apps/tixpire-payments"
		http.Redirect(w, r, target, 303)

	} else if len(params["shop"]) == 1 {
		// install request, redirect to Shopify
		shop := params["shop"][0] + ".myshopify.com"
		log.Debugf(ctx, "serveInstall first shop: %s", shop)
		log.Debugf(ctx, "starting oauth flow")
		state = nonce.NewToken()
		log.Debugf(ctx, "state1: %s", state)
		http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
	}
}


/*
func serveAppProxy(w http.ResponseWriter, r *http.Request) {
	if app.AppProxySignatureOk(r.URL) {
		http.ServeFile(w, r, "static/app_proxy.html")
	} else {
		http.Error(w, "Unauthorized", 401)
	}
}
*/
// initial page served when visited as embedded app inside Shopify admin
func serveAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "serveAdmin RAN")
	session := getSession(r)
	params := r.URL.Query()


	// signed request from Shopify?
	if app.AdminSignatureOk(r.URL, state) {
		log.Debugf(ctx, "signed request!")
		log.Debugf(ctx, "serveAdmin func AdminSignatureOk shop: %s", params["shop"][0])
		session.Values["current_shop"] = params["shop"][0]
		session.Save(r, w)
	} else if _, ok := session.Values["current_shop"]; !ok {
		log.Debugf(ctx, "no current_shop")
		// not logged in and not signed request
		http.Error(w, "Unauthorized", 401)
		return
	}

	shop, _ := session.Values["current_shop"].(string)
	log.Debugf(ctx, "serveAdmin shop: %s", shop)
	// if we don't have an access token for the shop, obtain one now.
	var shopEntity []Shop
	if _, ok := tokens[shop]; !ok {
		log.Debugf(ctx, "No token stored interally. Checking datastore...")
		q := datastore.NewQuery("Shop").Filter("Name =", shop)
		_, err := q.GetAll(ctx, &shopEntity)
		log.Debugf(ctx, "shopEntity: %s", shopEntity)
		if err != nil || len(shopEntity) == 0 {
			// Handle error.
			state := nonce.NewToken()
			log.Debugf(ctx, "No access token")
			http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
			return
		}
		tokens[shop] = shopEntity[0].Token
		log.Debugf(ctx, "shopEntity token: %s", shopEntity[0].Token)
	}
	log.Debugf(ctx, "shopEntity: %s", shopEntity)
	// they're logged in
	log.Debugf(ctx, "Access token found. They're logged in")
	type AdminVars struct {
		Shop   string
		APIKey string
		AppName string
	}
	v := AdminVars{Shop: shop, APIKey: app.APIKey, AppName: "Tixpire Payments"}

	tpl.ExecuteTemplate(w, "admin.gohtml", v)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
    return
  }
	tpl.ExecuteTemplate(w, "index.gohtml", nil)
}

func serveUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "update RAN")
	session := getSession(r)
	params := r.URL.Query()

	shop := params["shop"][0]
	session.Values["current_shop"] = shop
	err := session.Save(r, w)
	if err != nil {
		panic(err)
	}

	var shopEntity []Shop
	if _, ok := tokens[shop]; !ok {
		log.Debugf(ctx, "No token stored interally. Checking datastore...")
		q := datastore.NewQuery("Shop").Filter("Name =", shop)
		_, err := q.GetAll(ctx, &shopEntity)
		log.Debugf(ctx, "shopEntity: %s", shopEntity)
		if err != nil || len(shopEntity) == 0 {
			// Handle error.
			state := nonce.NewToken()
			log.Debugf(ctx, "No access token")
			http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
			return
		}
		tokens[shop] = shopEntity[0].Token
		log.Debugf(ctx, "shopEntity token: %s", shopEntity[0].Token)
	}
	uri := "https://" + shop
	api = &shopify.API {
		URI: uri,
		Token: tokens[shop],
		Secret: os.Getenv("SHOPIFY_API_SECRET"),
		Context: ctx,
	}
	log.Debugf(ctx, "api initialized: %s", api)

	themes, themesErr := api.Themes()
	log.Debugf(ctx, "themes api called")
	var themeId int64
	if themesErr != nil {
		log.Debugf(ctx, "Themes Error: %s", themesErr)
		http.Error(w, themesErr.Error(), http.StatusInternalServerError)
	}

	for i := 0; i < len(themes); i++ {
		if (themes[i].Role == "main") {
			themeId = themes[i].Id
		}
	}
	log.Debugf(ctx, "Theme id: %s", themeId)

	assetParams := url.Values{}
	assetParams.Set("asset[key]", "snippets/ajax-cart-template.liquid")

	asset, assetErr := api.Asset(themeId, assetParams)
	if assetErr != nil {
		log.Debugf(ctx, "Asset Err: %s", assetErr)
		http.Error(w, assetErr.Error(), http.StatusInternalServerError)
	}

	assetData, dataErr := ioutil.ReadFile("product-page-button.liquid")
	if dataErr != nil {
		log.Debugf(ctx, "Error reading checkout.liquid file: %s", dataErr)
	}
	button := string(assetData)
	var indexTop int
	var indexBottom int
	if shop == "tasteoftravel.myshopify.com" {
		indexTop = strings.Index(assetValue, "</button>\n\n{% endif %}\n<!-- end Bold code -->")
		indexTop = indexTop + len("</button>\n\n{% endif %}\n<!-- end Bold code -->")
		indexBottom = strings.Index(assetValue, "</div>\n            </form>\n\n          </div>\n\n          <div class=\"product-single__description rte\" itemprop=\"description\">\n            {{ product.description }}\n          </div>")
	} else {
		indexTop = strings.Index(assetValue, "{% endif %}\n              </div>")
		indexTop = indexTop + len("{% endif %}\n              </div>")
		indexBottom = strings.Index(assetValue, "{% endform %}\n\n          </div>")
	}

	indexBottom = indexBottom - 1
	newAsset := assetValue[:indexTop] + button + assetValue[indexBottom:]
	assetValue := strings.Replace(asset.Value, "Tixpire Payments", "PAY OVER TIME", 1)

	assetChange := api.NewAsset()
	assetChange.Key = "snippets/ajax-cart-template.liquid"
	assetChange.ThemeId = themeId
	assetChange.Value = assetValue
	assetChangeErr := assetChange.Save()
	if assetChangeErr != nil {
  	log.Debugf(ctx, "Error saving asset: %s", assetChangeErr)
	}
	io.WriteString(w, "Update Success!")
	return
}

func serveAddCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "update RAN")
	session := getSession(r)
	params := r.URL.Query()

	shop := params["shop"][0]
	session.Values["current_shop"] = shop
	err := session.Save(r, w)
	if err != nil {
		panic(err)
	}

	var shopEntity []Shop
	if _, ok := tokens[shop]; !ok {
		log.Debugf(ctx, "No token stored interally. Checking datastore...")
		q := datastore.NewQuery("Shop").Filter("Name =", shop)
		_, err := q.GetAll(ctx, &shopEntity)
		log.Debugf(ctx, "shopEntity: %s", shopEntity)
		if err != nil || len(shopEntity) == 0 {
			// Handle error.
			state := nonce.NewToken()
			log.Debugf(ctx, "No access token")
			http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
			return
		}
		tokens[shop] = shopEntity[0].Token
		log.Debugf(ctx, "shopEntity token: %s", shopEntity[0].Token)
	}
	uri := "https://" + shop
	api = &shopify.API {
		URI: uri,
		Token: tokens[shop],
		Secret: os.Getenv("SHOPIFY_API_SECRET"),
		Context: ctx,
	}
	log.Debugf(ctx, "api initialized: %s", api)

	themes, themesErr := api.Themes()
	log.Debugf(ctx, "themes api called")
	var themeId int64
	if themesErr != nil {
		log.Debugf(ctx, "Themes Error: %s", themesErr)
		http.Error(w, themesErr.Error(), http.StatusInternalServerError)
	}

	for i := 0; i < len(themes); i++ {
		if (themes[i].Role == "main") {
			themeId = themes[i].Id
		}
	}
	log.Debugf(ctx, "Theme id: %s", themeId)

	assetData, dataErr := ioutil.ReadFile("checkout.liquid")
	if dataErr != nil {
		log.Debugf(ctx, "Error reading checkout.liquid file: %s", dataErr)
	}

	asset := api.NewAsset()
	asset.Key = "templates/page.checkout.liquid"
	asset.ThemeId = themeId
	asset.Value = string(assetData)
	assetErr := asset.Save()
	if assetErr != nil {
  	log.Debugf(ctx, "Error saving asset: %s", assetErr)
	}
	io.WriteString(w, "Add Checkout Page Was A Success!")
	return
}

func serveAddProduct(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "update RAN")
	session := getSession(r)
	params := r.URL.Query()

	shop := params["shop"][0]
	session.Values["current_shop"] = shop
	err := session.Save(r, w)
	if err != nil {
		panic(err)
	}

	var shopEntity []Shop
	if _, ok := tokens[shop]; !ok {
		log.Debugf(ctx, "No token stored interally. Checking datastore...")
		q := datastore.NewQuery("Shop").Filter("Name =", shop)
		_, err := q.GetAll(ctx, &shopEntity)
		log.Debugf(ctx, "shopEntity: %s", shopEntity)
		if err != nil || len(shopEntity) == 0 {
			// Handle error.
			state := nonce.NewToken()
			log.Debugf(ctx, "No access token")
			http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
			return
		}
		tokens[shop] = shopEntity[0].Token
		log.Debugf(ctx, "shopEntity token: %s", shopEntity[0].Token)
	}
	uri := "https://" + shop
	api = &shopify.API {
		URI: uri,
		Token: tokens[shop],
		Secret: os.Getenv("SHOPIFY_API_SECRET"),
		Context: ctx,
	}
	log.Debugf(ctx, "api initialized: %s", api)

	themes, themesErr := api.Themes()
	log.Debugf(ctx, "themes api called")
	var themeId int64
	if themesErr != nil {
		log.Debugf(ctx, "Themes Error: %s", themesErr)
		http.Error(w, themesErr.Error(), http.StatusInternalServerError)
	}

	for i := 0; i < len(themes); i++ {
		if (themes[i].Role == "main") {
			themeId = themes[i].Id
		}
	}
	log.Debugf(ctx, "Theme id: %s", themeId)

	assetParams := url.Values{}
	assetParams.Set("asset[key]", "sections/product-template.liquid")

	asset, assetErr := api.Asset(themeId, assetParams)
	if assetErr != nil {
		log.Debugf(ctx, "Asset Err: %s", assetErr)
		http.Error(w, assetErr.Error(), http.StatusInternalServerError)
	}
	assetValue := asset.Value

	assetData, dataErr := ioutil.ReadFile("product-page-button.liquid")
	if dataErr != nil {
		log.Debugf(ctx, "Error reading checkout.liquid file: %s", dataErr)
	}
	button := string(assetData)
	var indexTop int
	var indexBottom int
	if shop == "tasteoftravel.myshopify.com" {
		indexTop = strings.Index(assetValue, "</button>\n\n{% endif %}\n<!-- end Bold code -->")
		indexTop = indexTop + len("</button>\n\n{% endif %}\n<!-- end Bold code -->")
		indexBottom = strings.Index(assetValue, "</div>\n            </form>\n\n          </div>\n\n          <div class=\"product-single__description rte\" itemprop=\"description\">\n            {{ product.description }}\n          </div>")
	} else {
		indexTop = strings.Index(assetValue, "{% endif %}\n              </div>")
		indexTop = indexTop + len("{% endif %}\n              </div>")
		indexBottom = strings.Index(assetValue, "{% endform %}\n\n          </div>")
	}

	indexBottom = indexBottom - 1
	newAsset := assetValue[:indexTop] + button + assetValue[indexBottom:]


	assetChange := api.NewAsset()
	assetChange.Key = "sections/product-template.liquid"
	assetChange.ThemeId = themeId
	assetChange.Value = newAsset
	assetChangeErr := assetChange.Save()
	if assetChangeErr != nil {
  	log.Debugf(ctx, "Error saving asset: %s", assetChangeErr)
	}

	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, newAsset)
	return
}

func serveGetTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Debugf(ctx, "update RAN")
	session := getSession(r)
	params := r.URL.Query()

	shop := params["shop"][0]
	session.Values["current_shop"] = shop
	err := session.Save(r, w)
	if err != nil {
		panic(err)
	}

	var shopEntity []Shop
	if _, ok := tokens[shop]; !ok {
		log.Debugf(ctx, "No token stored interally. Checking datastore...")
		q := datastore.NewQuery("Shop").Filter("Name =", shop)
		_, err := q.GetAll(ctx, &shopEntity)
		log.Debugf(ctx, "shopEntity: %s", shopEntity)
		if err != nil || len(shopEntity) == 0 {
			// Handle error.
			state := nonce.NewToken()
			log.Debugf(ctx, "No access token")
			http.Redirect(w, r, app.AuthorizeURL(shop, "read_themes,write_themes", state), 302)
			return
		}
		tokens[shop] = shopEntity[0].Token
		log.Debugf(ctx, "shopEntity token: %s", shopEntity[0].Token)
	}
	uri := "https://" + shop
	api = &shopify.API {
		URI: uri,
		Token: tokens[shop],
		Secret: os.Getenv("SHOPIFY_API_SECRET"),
		Context: ctx,
	}
	log.Debugf(ctx, "api initialized: %s", api)

	themes, themesErr := api.Themes()
	log.Debugf(ctx, "themes api called")
	var themeId int64
	if themesErr != nil {
		log.Debugf(ctx, "Themes Error: %s", themesErr)
		http.Error(w, themesErr.Error(), http.StatusInternalServerError)
	}

	for i := 0; i < len(themes); i++ {
		if (themes[i].Role == "main") {
			themeId = themes[i].Id
		}
	}
	log.Debugf(ctx, "Theme id: %s", themeId)

	assetParams := url.Values{}
	assetParams.Set("asset[key]", "sections/product-template.liquid")

	asset, assetErr := api.Asset(themeId, assetParams)
	if assetErr != nil {
		log.Debugf(ctx, "Asset Err: %s", assetErr)
		http.Error(w, assetErr.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, asset.Value)
	return
}

func main() {
	http.HandleFunc("/install", serveInstall)
	http.HandleFunc("/admin", serveAdmin)
	http.HandleFunc("/update", serveUpdate)
	http.HandleFunc("/addCheckout", serveAddCheckout)
	http.HandleFunc("/addProduct", serveAddProduct)
	http.HandleFunc("/getTemplate", serveGetTemplate)
	//http.HandleFunc("/shopify/app_proxy/", serveAppProxy)
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
