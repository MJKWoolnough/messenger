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
	cDomain   = "https://www.messenger.com/"
	cLoginURL = cDomain + "login"
)

var domain, loginURL *url.URL

func init() {
	domain, _ = url.Parse(cDomain)
	loginURL, _ = url.Parse(cLoginURL)
}

type Client struct {
	http.Client
}

func NewClient(cookies []*http.Cookie) *Client {
	var client Client
	client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if len(cookies) > 0 {
		client.Jar.SetCookies(domain, cookies)
	}
	return &client
}

func (c *Client) Resume() error {
	c.CheckRedirect = noRedirect
	resp, err := c.Get(cLoginURL)
	c.CheckRedirect = nil
	if err != nil {
		return err
	}
	if resp.StatusCode != 302 {
		return ErrInvalidCookies
	}
	return nil
}

func (c *Client) Login(username, password string) error {
	resp, err := c.Get(cLoginURL)
	if err != nil {
		return errors.WithContext("error getting login page: ", err)
	}
	nodes, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		return errors.WithContext("error parsing login page: ", err)
	}
	var cookieSet bool
	for scripts := xmlpath.MustCompile("//script").Iter(nodes); scripts.Next(); {
		node := scripts.Node().String()
		if strings.Contains(node, "_js_datr") {
			parts := strings.Split(regexp.MustCompile("\\[\"_js_datr\"[^\\]]*\\]").FindString(node), "\"")
			if len(parts) > 3 {
				c.Jar.SetCookies(domain, []*http.Cookie{
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
		return errors.Error("error grabbing datr cookie")

	}
	var postURL string
	if loginURLP := xmlpath.MustCompile("//form/@action").Iter(nodes); loginURLP.Next() {
		action, err := url.Parse(loginURLP.Node().String())
		if err != nil {
			return errors.WithContext("error parsing login URL: ", err)
		}
		postURL = loginURL.ResolveReference(action).String()
	} else {
		return errors.Error("error retrieving login POST URL")
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
	c.CheckRedirect = noRedirect
	resp, err = c.PostForm(postURL, inputs)
	if err != nil {
		return errors.WithContext("error POSTing login form: ", err)
	}

	var goodCookies bool
	for _, cookie := range c.GetCookies() {
		if cookie.Name == "c_user" {
			goodCookies = true
			break
		}
	}

	if !goodCookies {
		return ErrInvalidLogin
	}

	return nil
}

func (c *Client) GetCookies() []*http.Cookie {
	return c.Jar.Cookies(domain)
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

const (
	ErrInvalidCookies errors.Error = "invalid cookies"
	ErrInvalidLogin   errors.Error = "invalid login credentials"
)
