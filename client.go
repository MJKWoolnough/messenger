package messenger

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MJKWoolnough/errors"
	"github.com/MJKWoolnough/memio"
	"github.com/robertkrimen/otto"
	xmlpath "gopkg.in/xmlpath.v2"
)

const CLIENT_VERSION = 3822019

const (
	cDomain   = "https://www.messenger.com/"
	cLoginURL = cDomain + "login"
	cAPIURL   = cDomain + "api/graphqlbatch"
)

var (
	domain, loginURL, apiURL *url.URL
	pageScripts              = xmlpath.MustCompile("//script[not(@src)]")
)

func init() {
	domain, _ = url.Parse(cDomain)
	loginURL, _ = url.Parse(cLoginURL)
	apiURL, _ = url.Parse(cAPIURL)
}

type Client struct {
	client                  http.Client
	postData                url.Values
	username, usernameShort string
	docIDs                  map[string]string

	request uint64 // atomic

	dataMu  sync.RWMutex
	threads map[string]Thread
	users   map[string]User
}

/*
func (c *Client) Resume() error {
	c.client.CheckRedirect = noRedirect
	resp, err := c.client.Get(cLoginURL)
	c.client.CheckRedirect = nil
	if err != nil {
		return err
	}
	if resp.StatusCode != 302 {
		return ErrInvalidCookies
	}
	return c.init()
}
*/

func Login(username, password string) (*Client, error) {
	var c Client
	resp, err := c.client.Get(cLoginURL)
	if err != nil {
		return nil, errors.WithContext("error getting login page: ", err)
	}
	nodes, err := xmlpath.ParseHTML(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, errors.WithContext("error parsing login page: ", err)
	}
	var (
		cookieSet   bool
		cookieValue string
	)
	if err = runCode(
		jsFuncs{
			"setCookieValue": func(call otto.FunctionCall) otto.Value {
				cookieSet = true
				cookieValue = call.Argument(0).String()
				return otto.UndefinedValue()
			},
		},
		xmlPathIter{pageScripts.Iter(nodes)},
	); err != nil {
		return nil, errors.WithContext("error grabbing datr cookie: ", err)
	}
	if !cookieSet {
		return nil, ErrDatrCookie

	}
	c.client.Jar.SetCookies(domain, []*http.Cookie{
		&http.Cookie{
			Name:     "datr",
			Value:    cookieValue,
			Path:     "/",
			Expires:  time.Now().Add(time.Hour * 48),
			HttpOnly: true,
			Secure:   true,
		},
	})
	var postURL string
	if loginURLP := xmlpath.MustCompile("//form/@action").Iter(nodes); loginURLP.Next() {
		action, err := url.Parse(loginURLP.Node().String())
		if err != nil {
			return nil, errors.WithContext("error parsing login URL: ", err)
		}
		postURL = loginURL.ResolveReference(action).String()
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
	c.client.CheckRedirect = noRedirect
	resp, err = c.postForm(postURL, inputs)
	if err != nil {
		return nil, errors.WithContext("error POSTing login form: ", err)
	}

	var goodCookies bool
	for _, cookie := range c.client.Jar.Cookies(domain) {
		if cookie.Name == "c_user" {
			_, err = strconv.ParseUint(cookie.Value, 10, 64)
			if err != nil {
				return nil, errors.WithContext("error parsing userID:", err)
			}
			goodCookies = true
			break
		}
	}

	if !goodCookies {
		return nil, ErrInvalidLogin
	}

	if err = c.init(); err != nil {
		return nil, err
	}
	return &c, nil
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func (c *Client) init() error {
	resp, err := c.client.Get(cDomain)
	if err != nil {
		return errors.WithContext("error grabbing init page: ", err)
	}
	nodes, err := xmlpath.ParseHTML(resp.Body)
	resp.Body.Close()
	if err != nil {
		return errors.WithContext("error parsing init page: ", err)
	}

	var (
		udSet, dtsgSet, sdSet, snSet bool
		sprinkleName, sprinkleValue  string
		highestBit                   int64
	)
	bitmap := make(map[int64]struct{})
	resources := make(map[string][]string)

	c.postData = make(url.Values)
	c.postData.Set("__a", "1")
	c.postData.Set("__rev", strconv.FormatUint(CLIENT_VERSION, 10))
	var list threadList
	if err = runCode(
		jsFuncs{
			"setUserData": func(call otto.FunctionCall) otto.Value {
				c.postData.Set("__user", call.Argument(0).String())
				c.username = call.Argument(1).String()
				c.usernameShort = call.Argument(2).String()
				udSet = true
				return otto.UndefinedValue()
			},
			"setDTSGToken": func(call otto.FunctionCall) otto.Value {
				sprinkleValue = call.Argument(0).String()
				c.postData.Set("fb_dtsg", sprinkleValue)
				dtsgSet = true
				return otto.UndefinedValue()
			},
			"setSiteData": func(call otto.FunctionCall) otto.Value {
				c.postData.Set(call.Argument(1).String(), call.Argument(2).String())
				c.postData.Set(call.Argument(3).String(), call.Argument(4).String())
				sdSet = true
				return otto.UndefinedValue()
			},
			"setSprinkleName": func(call otto.FunctionCall) otto.Value {
				sprinkleName = call.Argument(0).String()
				snSet = true
				return otto.UndefinedValue()
			},
			"setBitmap": func(call otto.FunctionCall) otto.Value {
				i, _ := call.Argument(0).ToInteger()
				bitmap[i] = struct{}{}
				if i > highestBit {
					highestBit = i
				}
				return otto.UndefinedValue()
			},
			"setResource": func(call otto.FunctionCall) otto.Value {
				key := call.Argument(0).String()
				resources[key] = append(resources[key], call.Argument(1).String())
				return otto.UndefinedValue()
			},
			"setThreadData": func(call otto.FunctionCall) otto.Value {
				json.NewDecoder(strings.NewReader(call.Argument(0).String())).Decode(&list)
				return otto.UndefinedValue()
			},
		},
		xmlPathIter{pageScripts.Iter(nodes)},
	); err != nil {
		return errors.WithContext("error getting init values: ", err)
	}

	if !udSet {
		err = ErrUnsetUserData
	} else if !dtsgSet {
		err = ErrUnsetDTSGToken
	} else if !sdSet {
		err = ErrUnsetSiteData
	} else if !snSet || sprinkleName == "" {
		err = ErrUnsetSprinkleName
	}
	if err != nil {
		return errors.WithContext("error getting init config: ", err)
	}

	c.threads = make(map[string]Thread, len(list.List.Data.Viewer.MessageThreads.Nodes))
	c.users = make(map[string]User)
	if err = c.parseThreadData(list); err != nil {
		return err
	}

	buf := make(memio.Buffer, 1, 3*len(sprinkleValue)+1)
	buf[0] = '2'
	for _, char := range sprinkleValue {
		fmt.Fprint(&buf, char)
	}
	c.postData.Set(sprinkleName, string(buf))

	_, set := bitmap[0]
	r := newRLE(set)
	for i := int64(1); i < highestBit; i++ {
		_, set = bitmap[i]
		r.WriteBool(set)
	}
	c.postData.Set("__dyn", r.String())

	loaded := make(map[string]struct{})
	si := make(stringIter, 0, 100)
	var sb strings.Builder

	for _, resource := range resources {
		for _, url := range resource {
			if _, ok := loaded[url]; !ok {
				resp, err := c.client.Get(url)
				if err != nil {
					return errors.WithContext("error getting resource: ", err)
				}
				_, err = io.Copy(&sb, resp.Body)
				resp.Body.Close()
				if err != nil {
					return errors.WithContext("error reading resource: ", err)
				}
				si = append(si, sb.String())
				sb.Reset()
				loaded[url] = struct{}{}
			}
		}
	}

	c.docIDs = make(map[string]string, len(resources))

	if err = runCode(
		jsFuncs{
			"setID": func(call otto.FunctionCall) otto.Value {
				c.docIDs[call.Argument(0).String()] = call.Argument(1).String()
				return otto.UndefinedValue()
			},
		},
		&si,
	); err != nil {
		return errors.WithContext("error running resource scripts: ", err)
	}

	return nil
}

func (c *Client) postForm(url string, data url.Values) (*http.Response, error) {
	for key := range c.postData {
		data.Set(key, c.postData.Get(key))
	}
	data.Set("__req", strconv.FormatUint(atomic.AddUint64(&c.request, 1), 36))
	return c.client.PostForm(url, data)
}

const (
	ErrDatrCookie        errors.Error = "error grabbing datr cookie"
	ErrInvalidCookies    errors.Error = "invalid cookies"
	ErrInvalidLogin      errors.Error = "invalid login credentials"
	ErrUnsetUserData     errors.Error = "user data not set"
	ErrUnsetDTSGToken    errors.Error = "DTSG token not set"
	ErrUnsetSiteData     errors.Error = "site data not set"
	ErrUnsetSprinkleName errors.Error = "sprinkle name not set"
)
