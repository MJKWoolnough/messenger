package messenger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/MJKWoolnough/errors"
	"github.com/MJKWoolnough/memio"
	"golang.org/x/net/publicsuffix"
)

type clientJSON struct {
	Cookies       []*http.Cookie    `json:"cookies"`
	PostData      url.Values        `json:"post_data"`
	DocIDs        map[string]string `json:"doc_ids"`
	Username      string            `json:"username"`
	UsernameShort string            `json:"username_short"`
	Request       uint64            `json:"request"`
}

func (c *Client) MarshalJSON() ([]byte, error) {
	var b memio.Buffer
	if err := c.MarshalJSONWriter(&b); err != nil {
		return nil, err
	}
	return b, nil
}

func (c *Client) MarshalJSONWriter(w io.Writer) error {
	c.requestMu.Lock()
	c.dataMu.RLock()
	data := clientJSON{
		Cookies:       c.client.Jar.Cookies(domain),
		PostData:      c.postData,
		DocIDs:        c.docIDs,
		Username:      c.username,
		UsernameShort: c.usernameShort,
		Request:       c.request,
	}
	c.dataMu.RUnlock()
	c.requestMu.Unlock()
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return errors.WithContext("error marshaling JSON: ", err)
	}
	return nil
}

func (c *Client) UnmarshalJSON(b []byte) error {
	return c.UnmarshalJSONReader((*memio.Buffer)(&b))
}

func (c *Client) UnmarshalJSONReader(r io.Reader) error {
	if c.docIDs != nil {
		return ErrIntialised
	}
	var data clientJSON
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return errors.WithContext("error unmarshaling JSON: ", err)
	}
	c.client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if len(data.Cookies) > 0 {
		c.client.Jar.SetCookies(domain, data.Cookies)
	}
	c.postData = data.PostData
	c.docIDs = data.DocIDs
	c.username = data.Username
	c.usernameShort = data.UsernameShort
	c.request = data.Request
	c.threads = make(map[string]Thread)
	c.users = make(map[string]User)
	return nil
}

const (
	ErrIntialised errors.Error = "already initialised"
)
