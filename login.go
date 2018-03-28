package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/MJKWoolnough/errors"
	"golang.org/x/net/publicsuffix"
	xmlpath "gopkg.in/xmlpath.v2"
)

const (
	domain   = "https://www.messenger.com/"
	loginURL = domain + "login"
)

func ConfirmCookies(cookies []*http.Cookie) error {
	var client http.Client
	client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	d, _ := url.Parse(domain)
	client.Jar.SetCookies(d, cookies)
	client.CheckRedirect = noRedirect
	resp, err := client.Get(loginURL)
	if err != nil {
		return err
	}
	if resp.StatusCode != 302 {
		return ErrInvalidCookies
	}
	return nil
}

func Login(username, password string) ([]*http.Cookie, error) {
	d, _ := url.Parse(domain)
	u, _ := url.Parse(loginURL)
	var client http.Client
	client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	resp, err := client.Get(loginURL)
	if err != nil {
		return nil, errors.WithContext("error getting login page: ", err)
	}
	nodes, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		return nil, errors.WithContext("error parsing login page: ", err)
	}
	var cookieSet bool
	for scripts := xmlpath.MustCompile("//script").Iter(nodes); scripts.Next(); {
		node := scripts.Node().String()
		if strings.Contains(node, "_js_datr") {
			parts := strings.Split(regexp.MustCompile("\\[\"_js_datr\"[^\\]]*\\]").FindString(node), "\"")
			if len(parts) > 3 {
				client.Jar.SetCookies(d, []*http.Cookie{
					&http.Cookie{
						Name:     "datr",
						Value:    parts[3],
						Path:     "/",
						Expires:  time.Now().Add(time.Hour * 48),
						HttpOnly: true,
						Secure:   true,
					},
				})
				cookieSet = true
				break
			}
		}
	}
	if !cookieSet {
		return nil, errors.Error("error grabbing datr cookie")

	}
	var postURL string
	if loginURLP := xmlpath.MustCompile("//form/@action").Iter(nodes); loginURLP.Next() {
		action, err := url.Parse(loginURLP.Node().String())
		if err != nil {
			return nil, errors.WithContext("error parsing login URL: ", err)
		}
		postURL = u.ResolveReference(action).String()
	} else {
		return nil, errors.Error("error retrieving login POST URL")
	}

	inputs := make(url.Values)
	for iter := xmlpath.MustCompile("//form//input/@name").Iter(nodes); iter.Next(); {
		node := iter.Node()
		if value := xmlpath.MustCompile(fmt.Sprintf("//input[@name=%q]/@value", node)).Iter(nodes); value.Next() {
			inputs.Add(node.String(), value.Node().String())
		} else {
			inputs.Add(node.String(), "")
		}
	}
	inputs.Set("email", username)
	inputs.Set("pass", password)
	inputs.Set("login", "1")
	inputs.Set("persistant", "1")
	client.CheckRedirect = noRedirect
	resp, err = client.PostForm(postURL, inputs)
	if err != nil {
		return nil, errors.WithContext("error POSTing login form: ", err)
	}
	cookies := client.Jar.Cookies(d)

	var goodCookies bool
	for _, cookie := range cookies {
		if cookie.Name == "c_user" {
			goodCookies = true
			break
		}
	}

	if !goodCookies {
		return nil, ErrInvalidLogin
	}

	return cookies, nil
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

const (
	ErrInvalidCookies errors.Error = "invalid cookies"
	ErrInvalidLogin   errors.Error = "invalid login credentials"
)
