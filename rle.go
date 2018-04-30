package main

import (
	"encoding/base64"
	"math/bits"
	"strconv"
	"strings"

	"github.com/MJKWoolnough/boolmap"
)

var encoder = base64.NewEncoding("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_").WithPadding(base64.NoPadding)

type RLE struct {
	curr  bool
	count int
	buf   boolmap.Slice
	pos   uint
}

func NewRLE(start bool) *RLE {
	r := &RLE{
		curr:  start,
		buf:   make(boolmap.Slice, 256),
		pos:   1,
		count: 1,
	}
	r.buf.SetBool(0, start)
	return r
}

func (r *RLE) WriteBool(b bool) {
	if b == r.curr {
		r.count++
		return
	}
	r.add()
	r.curr = !r.curr
	r.count = 1
}

func (r *RLE) add() {
	bin := strconv.FormatUint(uint64(r.count), 2)
	r.pos += uint(len(bin)) - 1
	for _, d := range bin {
		r.buf.SetBool(r.pos, d == '1')
		r.pos++
	}
}

func (r *RLE) String() string {
	r.add()
	for i, n := range r.buf {
		r.buf[i] = bits.Reverse8(n)
	}
	size := r.pos / 8
	if r.pos%8 > 0 {
		size++
	}
	return strings.TrimRight(encoder.EncodeToString(r.buf[:size]), "0")

}
