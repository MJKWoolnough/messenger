package messenger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"vimagination.zapto.org/byteio"
	"vimagination.zapto.org/errors"
	"vimagination.zapto.org/memio"
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
	c.dataMu.RLock()
	data := clientJSON{
		Cookies:       c.client.Jar.Cookies(domain),
		PostData:      c.postData,
		DocIDs:        c.docIDs,
		Username:      c.username,
		UsernameShort: c.usernameShort,
		Request:       atomic.LoadUint64(&c.request),
	}
	c.dataMu.RUnlock()
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
	c.client.Jar = newJar()
	if len(data.Cookies) > 0 {
		c.client.Jar.SetCookies(domain, data.Cookies)
	}
	c.postData = data.PostData
	c.docIDs = data.DocIDs
	c.username = data.Username
	c.usernameShort = data.UsernameShort
	atomic.StoreUint64(&c.request, data.Request)
	c.threads = make(map[string]Thread)
	c.users = make(map[string]User)
	return nil
}

type stringWriter struct {
	byteio.StickyLittleEndianWriter
}

func (s *stringWriter) WriteString(str string) (int, error) {
	s.WriteUint32(uint32(len(str)))
	return s.Write([]byte(str))
}

func (c *Client) MarshalBinary() ([]byte, error) {
	var b memio.Buffer
	if err := c.MarshalBinaryWriter(&b); err != nil {
		return nil, err
	}
	return b, nil
}

func (c *Client) MarshalBinaryWriter(w io.Writer) error {
	// cookies
	// postdata
	// username/usernameShort
	// request
	sw := stringWriter{
		StickyLittleEndianWriter: byteio.StickyLittleEndianWriter{
			Writer: w,
		},
	}
	c.dataMu.RLock()
	cookies := c.client.Jar.Cookies(domain)
	sw.WriteUint8(uint8(len(cookies)))
	for _, cookie := range cookies {
		sw.WriteString(cookie.Name)
		sw.WriteString(cookie.Value)
		sw.WriteString(cookie.Path)
		sw.WriteString(cookie.Domain)
		sw.WriteInt64(int64(cookie.MaxAge))
		if cookie.HttpOnly {
			sw.WriteUint8(1)
		} else {
			sw.WriteUint8(0)
		}
		if cookie.Secure {
			sw.WriteUint8(1)
		} else {
			sw.WriteUint8(0)
		}
		buf, _ := cookie.Expires.MarshalBinary()
		sw.WriteUint8(uint8(len(buf)))
		sw.Write(buf)
	}

	sw.WriteUint8(uint8(len(c.postData)))
	for key := range c.postData {
		sw.WriteString(key)
		sw.WriteString(c.postData.Get(key))
	}

	sw.WriteUint8(uint8(len(c.docIDs)))
	for key := range c.docIDs {
		sw.WriteString(key)
		sw.WriteString(c.docIDs[key])
	}

	sw.WriteString(c.username)
	sw.WriteString(c.usernameShort)
	sw.WriteUint64(atomic.LoadUint64(&c.request))
	c.dataMu.RUnlock()
	return nil
}

type stringReader struct {
	byteio.StickyLittleEndianReader
}

func (s *stringReader) ReadString() string {
	buf := make([]byte, s.ReadUint32())
	n, _ := io.ReadFull(s, buf)
	return string(buf[:n])
}

func (c *Client) UnmarshalBinary(b []byte) error {
	return c.UnmarshalBinaryReader((*memio.Buffer)(&b))
}

func (c *Client) UnmarshalBinaryReader(r io.Reader) error {
	if c.docIDs != nil {
		return ErrIntialised
	}
	sr := stringReader{
		StickyLittleEndianReader: byteio.StickyLittleEndianReader{
			Reader: r,
		},
	}
	c.dataMu.Lock()
	cookies := make([]*http.Cookie, sr.ReadUint8())
	for n := range cookies {
		cookies[n] = &http.Cookie{
			Name:     sr.ReadString(),
			Value:    sr.ReadString(),
			Path:     sr.ReadString(),
			Domain:   sr.ReadString(),
			MaxAge:   int(sr.ReadInt64()),
			HttpOnly: sr.ReadUint8() == 1,
			Secure:   sr.ReadUint8() == 1,
		}
		buf := make([]byte, sr.ReadUint8())
		io.ReadFull(&sr, buf)
		cookies[n].Expires.UnmarshalBinary(buf)
	}
	c.client.Jar = newJar()
	if len(cookies) > 0 {
		c.client.Jar.SetCookies(domain, cookies)
	}

	pdLen := sr.ReadUint8()
	c.postData = make(url.Values, pdLen)
	for i := uint8(0); i < pdLen; i++ {
		key := sr.ReadString()
		c.postData.Set(key, sr.ReadString())
	}

	didLen := sr.ReadUint8()
	c.docIDs = make(map[string]string, didLen)
	for i := uint8(0); i < didLen; i++ {
		key := sr.ReadString()
		c.docIDs[key] = sr.ReadString()
	}

	c.username = sr.ReadString()
	c.usernameShort = sr.ReadString()
	atomic.StoreUint64(&c.request, sr.ReadUint64())

	c.dataMu.Unlock()
	return sr.Err
}

func (c *Client) MarshalText() ([]byte, error) {
	var b memio.Buffer
	if err := c.MarshalTextWriter(&b); err != nil {
		return nil, err
	}
	return b, nil
}

func (c *Client) MarshalTextWriter(w io.Writer) error {
	return c.MarshalJSONWriter(w)
}

func (c *Client) UnmarshalText(b []byte) error {
	return c.UnmarshalTextReader((*memio.Buffer)(&b))
}

func (c *Client) UnmarshalTextReader(r io.Reader) error {
	return c.UnmarshalJSONReader(r)
}

const (
	ErrIntialised errors.Error = "already initialised"
)
