package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"syscall"

	"github.com/gorilla/mux"

	"github.com/bborbe/stream/cache"

	"github.com/pkg/errors"

	"github.com/bborbe/argument"
	"github.com/elazarl/goproxy"
	"github.com/getsentry/raven-go"
	"github.com/golang/glog"
)

func main() {
	defer glog.Flush()
	glog.CopyStandardLogTo("info")
	runtime.GOMAXPROCS(runtime.NumCPU())
	_ = flag.Set("logtostderr", "true")

	app := &application{}
	if err := argument.Parse(app); err != nil {
		glog.Exitf("parse app failed: %v", err)
	}

	glog.V(0).Infof("application started")
	if err := app.run(contextWithSig(context.Background())); err != nil {
		raven.CaptureErrorAndWait(err, map[string]string{})
		glog.Exitf("application failed: %+v", err)
	}
	glog.V(0).Infof("application finished")
	os.Exit(0)
}

func contextWithSig(ctx context.Context) context.Context {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-signalCh:
		case <-ctx.Done():
		}
	}()

	return ctxWithCancel
}

type application struct {
}

func (a *application) run(ctx context.Context) error {

	c := cache.NewCache(
		ctx,
		http.DefaultClient,
	)

	router := mux.NewRouter()
	router.Path("/").HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Add("Content-Type", "text/html")
		resp.WriteHeader(http.StatusOK)
		fmt.Fprintf(resp, `<html><body>`)
		fmt.Fprintf(resp, `<h1>Hello</h1>`)

		fmt.Fprintf(resp, `<table><tr><td>URL</td><td>Connections</td><td>Size</td><td>Listeners</td></tr>`)
		for url, data := range c.Connections() {
			stream := data.Stream()
			fmt.Fprintf(resp, `<tr>`)
			fmt.Fprintf(resp, `<td>%s</td>`, url)
			fmt.Fprintf(resp, `<td>%d</td>`, len(stream.Listeners()))
			fmt.Fprintf(resp, `<td>%d</td>`, stream.Size())
			fmt.Fprintf(resp, `<td><ul>`)
			for _, listener := range stream.Listeners() {
				fmt.Fprintf(resp, `<li>%d</li>`, listener.Position())
			}
			fmt.Fprintf(resp, `</ul></td>`)
			fmt.Fprintf(resp, `</tr>`)
		}
		fmt.Fprintf(resp, `</table>`)
		fmt.Fprintf(resp, `<p>Total: %d</p>`, len(c.Connections()))

		fmt.Fprintf(resp, `<p><a href="/start">jump to start</a></p>`)
		fmt.Fprintf(resp, `<p><a href="/end">jump to end</a></p>`)

		fmt.Fprintf(resp, `</body></html>`)
	})
	router.Path("/start").HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for _, connection := range c.Connections() {
			stream := connection.Stream()
			listeners := stream.Listeners()
			for _, listener := range listeners {
				listener.SetPosition(0)
			}
		}
		fmt.Fprintf(resp, `set pos to start`)
	})
	router.Path("/end").HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for _, connection := range c.Connections() {
			stream := connection.Stream()
			pos := stream.Size()
			listeners := stream.Listeners()
			for _, listener := range listeners {
				listener.SetPosition(pos)
			}
		}
		fmt.Fprintf(resp, `set pos to end`)
	})

	proxy := goproxy.NewProxyHttpServer()
	proxy.NonproxyHandler = router
	proxy.Verbose = true

	list := []*regexp.Regexp{
		regexp.MustCompile(`lw\d+.aach.tb-group.fm`),
		regexp.MustCompile(`relay\d+.t4e.dj`),
	}

	proxy.OnRequest(goproxy.ReqHostMatches(list...)).DoFunc(
		func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			resp, err := c.RoundTrip(req)
			if err != nil {
				glog.Warningf("request failed: %v", err)
				return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusInternalServerError, "Failed")
			}
			return req, resp
		})

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (request *http.Request, response *http.Response) {
		glog.V(0).Infof("host: %s", req.Host)
		return req, nil
	})

	server := &http.Server{
		Addr:    ":3128",
		Handler: proxy,
	}
	go func() {
		select {
		case <-ctx.Done():
			if err := server.Shutdown(ctx); err != nil {
				glog.Warningf("shutdown failed: %v", err)
			}
		}
	}()
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		glog.V(0).Info(err)
		return nil
	}
	return errors.Wrap(err, "httpServer failed")
}
