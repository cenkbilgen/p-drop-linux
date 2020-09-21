package main

import (
  "github.com/andybalholm/brotli"
  "net/http"
  "io"
  "io/ioutil"
  "sync"
  "strings"
)

var brPool = sync.Pool {
	New: func() interface{} {
		w := brotli.NewWriter(ioutil.Discard)
		return w
	},
}

type brResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *brResponseWriter) WriterHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *brResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func Brotli(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "br")

		br := brPool.Get().(*brotli.Writer)
		defer brPool.Put(br)

		br.Reset(w)
		defer br.Close()

		next.ServeHTTP(&brResponseWriter{ResponseWriter: w, Writer: br}, r)
	})
}
