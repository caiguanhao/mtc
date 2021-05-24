package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
	"strings"
)

func main() {
	address := flag.String("address", "127.0.0.1:22222", "address to listen to")
	flag.Parse()

	var mtc MTC

	err := rpc.RegisterName("MTC", &mtc)
	if err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", jsonrpcOverHttpHandler)
	server := &http.Server{
		Addr:    *address,
		Handler: logRequest(mux),
	}
	log.Println("listening json-rpc over http", *address)
	server.ListenAndServe()
}

func jsonrpcOverHttpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method == "GET" {
		w.Write(indexHtml)
		return
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	if r.Body == nil {
		http.NotFound(w, r)
		return
	}
	defer r.Body.Close()
	res := jsonRPCRequest{r.Body, &bytes.Buffer{}}
	c := codec{codec: jsonrpc.NewServerCodec(&res)}
	rpc.ServeCodec(&c)
	w.Header().Set("Content-Type", "application/json")
	if c.isError {
		w.WriteHeader(400)
	}
	_, err := io.Copy(w, res.readWriter)
	if err != nil {
		log.Println("response error:", err)
		return
	}
	return
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

type (
	codec struct {
		codec   rpc.ServerCodec
		request *rpc.Request
		isError bool
	}

	jsonRPCRequest struct {
		reader     io.Reader
		readWriter io.ReadWriter
	}
)

func (c *codec) ReadRequestHeader(r *rpc.Request) error {
	c.request = r
	return c.codec.ReadRequestHeader(r)
}

func (c *codec) ReadRequestBody(x interface{}) error {
	err := c.codec.ReadRequestBody(x)
	b, _ := json.Marshal(x)
	log.Println("->", c.request.ServiceMethod, "-", strings.TrimSpace(string(b)))
	return err
}

func (c *codec) WriteResponse(r *rpc.Response, x interface{}) error {
	return c.codec.WriteResponse(r, x)
}

func (c *codec) Close() error {
	return c.codec.Close()
}

func (r *jsonRPCRequest) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *jsonRPCRequest) Write(p []byte) (n int, err error) {
	return r.readWriter.Write(p)
}

func (r *jsonRPCRequest) Close() error {
	return nil
}
