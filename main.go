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

		fmt.Fprintf(resp, `<table><tr><td>URL</td><td>Connections</td></tr>`)
		for url, data := range c.Connections() {
			fmt.Fprintf(resp, `<tr><td>%s</td><td>%d</td></tr>`, url, data.SharedReadCloser().Counter())
		}
		fmt.Fprintf(resp, `</table>`)
		fmt.Fprintf(resp, `<p>Total: %d</p>`, len(c.Connections()))

		fmt.Fprintf(resp, `</body></html>`)
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
