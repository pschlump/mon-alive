//
// Copyright (C) Philip Schlump, 2014-2016
//

package MonAliveMiddleware

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/pschlump/Go-FTL/server/cfg"
	"github.com/pschlump/Go-FTL/server/goftlmux"
	"github.com/pschlump/Go-FTL/server/httpmux"
	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/Go-FTL/server/mid"
	"github.com/pschlump/MiscLib"
	"github.com/pschlump/godebug"
	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/radix.v2/redis"
)

// --------------------------------------------------------------------------------------------------------------------------

func init() {

	// normally identical - but not this time.
	initNext := func(next http.Handler, g_cfg *cfg.ServerGlobalConfigType, pp_cfg interface{}, serverName string, pNo int) (rv http.Handler, err error) {
		p_cfg, ok := pp_cfg.(*MonAliveType)
		if ok {
			p_cfg.SetNext(next)
			rv = p_cfg
		} else {
			err = mid.FtlConfigError
			logrus.Errorf("Invalid type passed at: %s", godebug.LF())
		}
		g_cfg.ConnectToRedis()
		p_cfg.g_cfg = g_cfg
		return
	}

	postInit := func(h interface{}, callNo int) error {

		hh, ok := h.(*MonAliveType)
		if !ok {
			// rw.Log.Warn(fmt.Sprintf("Error: Wrong data type passed, Line No:%d\n", hh.LineNo))
			fmt.Printf("Error: Wrong data type passed, Line No:%d\n", hh.LineNo)
			return mid.ErrInternalError
		} else {
			hh.mon = MonAliveLib.NewMonIt(func() (conn *redis.Client) {
				var err error
				conn, err = hh.g_cfg.RedisPool.Get()
				if err != nil {
					logrus.Infof(`{"msg":"Error %s Unable to get redis pooled connection.","LineFile":%q}`+"\n", err, godebug.LF())
					return
				}
				return
			}, func(conn *redis.Client) {
				hh.g_cfg.RedisPool.Put(conn)
			})
		}

		return nil
	}

	// normally identical - not this time
	createEmptyType := func() interface{} {
		rv := &MonAliveType{}
		rv.mux = initMux(rv)
		return rv
	}

	cfg.RegInitItem2("MonAliveMiddlware", initNext, createEmptyType, postInit, `{
		"Paths":             { "type":["string","filepath"], "isarray":true, "required":true },
		"LineNo":            { "type":[ "int" ], "default":"1" }
		}`)
}

// normally identical
func (hdlr *MonAliveType) SetNext(next http.Handler) {
	hdlr.Next = next
}

var _ mid.GoFTLMiddleWare = (*MonAliveType)(nil)

// --------------------------------------------------------------------------------------------------------------------------

func initMux(hdlr *MonAliveType) (mux *httpmux.ServeMux) {

	mux = httpmux.NewServeMux()

	mux.HandleFunc("/api/mon/get-notify-item", hdlr.closure_respGetNotifyItem()).Method("GET")
	mux.HandleFunc("/api/mon/item-status", hdlr.closure_respItemStatus()).Method("GET")
	mux.HandleFunc("/api/mon/get-all-item", hdlr.closure_respGetAllItem()).Method("GET")
	mux.HandleFunc("/api/mon/add-new-item", hdlr.closure_respAddNewItem()).Method("GET")
	mux.HandleFunc("/api/mon/rem-item", hdlr.closure_respRemItem()).Method("GET")
	mux.HandleFunc("/api/mon/upd-config-item", hdlr.closure_respUpdConfigItem()).Method("GET")
	mux.HandleFunc("/api/mon/list-potential", hdlr.closure_respListPotential()).Method("GET")
	mux.HandleFunc("/api/mon/reload-config", hdlr.closure_respReloadConfig()).Method("GET")

	mux.HandleErrors(http.StatusNotFound, httpmux.HandlerFunc(errorHandlerFunc))
	return
}

// ----------------------------------------------------------------------------------------------------------------------------

func errorHandlerFunc(ww http.ResponseWriter, req *http.Request) {
	code := http.StatusForbidden
	ww.Header().Set("Content-Type", "text/plain; charset=utf-8")
	ww.Header().Set("X-Content-Type-Options", "nosniff")
	ww.WriteHeader(code)
	fmt.Fprintln(ww, "401 unautorized")
}

type MonAliveType struct {
	Next   http.Handler                //
	Paths  []string                    // Path to respond to
	LineNo int                         //
	g_cfg  *cfg.ServerGlobalConfigType //
	mon    *MonAliveLib.MonIt          //
	mux    *httpmux.ServeMux           // for non-encrypted (regular) calls
}

func NewMonAliveMiddlwareServer(n http.Handler, p []string, prefix string, userRoles []string) *MonAliveType {
	return &MonAliveType{
		Next:  n,
		Paths: p,
		// g_cfg  *cfg.ServerGlobalConfigType //
		// mon    *MonAliveLib.MonIt          //
		// mux    *httpmux.ServeMux           // for non-encrypted (regular) calls
	}
}

func (hdlr *MonAliveType) ServeHTTP(www http.ResponseWriter, req *http.Request) {

	if pn := lib.PathsMatchN(hdlr.Paths, req.URL.Path); pn >= 0 {
		if rw, ok := www.(*goftlmux.MidBuffer); ok {

			trx := mid.GetTrx(rw)
			trx.PathMatched(1, "MonAliveMiddlware", hdlr.Paths, pn, req.URL.Path)

			// -use- mux and match paths
			hh, _, err := hdlr.mux.Handler(req) // rv.mux.ServeHTTP(www, req)
			if err == nil {
				hh.ServeHTTP(www, req)
				return
			}

			// fmt.Printf("At: %s, s=%s\n", godebug.LF(), s)
			// Close off array
			www.Header().Set("Content-Type", "application/json")                     // For JSON data
			www.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
			www.Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
			www.Header().Set("Expires", "0")                                         // Proxies.
			s := "Some Helpful Message"
			fmt.Fprintf(www, "{ \"status\":\"error\", \"msg\":%q }", s) // return it to user

		} else {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
			fmt.Printf("%s\n", mid.ErrNonMidBufferWriter)
			www.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		hdlr.Next.ServeHTTP(www, req)
	}
}

func SetNoCacheHeaders(www http.ResponseWriter, req *http.Request) {
	www.Header().Set("Content-Type", "application/json")                     // For JSON data
	www.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
	www.Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
	www.Header().Set("Expires", "0")                                         // Proxies.
}

// URL: /api/mon/get-notify-item
// func (mon *MonIt) GetNotifyItem() (rv []string) {
//	mux.HandleFunc("/api/mon/get-notify-item", hdlr.closure_respGetNotifyItem()).Method("GET")
func (hdlr *MonAliveType) closure_respGetNotifyItem() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		s := hdlr.mon.GetNotifyItem()
		SetNoCacheHeaders(www, req)
		fmt.Fprintf(www, "{ \"status\":\"success\", \"data\": %s }", lib.SVar(s))
	}
}

/*
// URL: /api/mon/reload-config?fn=
func (mon *MonIt) SetConfigFromFile(fn string) {
//	mux.HandleFunc("/api/mon/add-new-item"   , hdlr.closure_respAddNewItem()).Method("GET")
//	mux.HandleFunc("/api/mon/rem-item"       , hdlr.closure_respRemItem()).Method("GET")
//	mux.HandleFunc("/api/mon/upd-config-item", hdlr.closure_respUpdConfigItem()).Method("GET")
//	mux.HandleFunc("/api/mon/list-potential" , hdlr.closure_respListPotential()).Method("GET")
*/

// URL: /api/mon/item-status
// func (mon *MonIt) GetItemStatus() (rv []ItemStatus) {
//	mux.HandleFunc("/api/mon/item-status"    , hdlr.closure_respItemStatusItem()).Method("GET")
func (hdlr *MonAliveType) closure_respItemStatus() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		x := hdlr.mon.GetItemStatus()
		SetNoCacheHeaders(www, req)
		fmt.Fprintf(www, "%s", lib.SVarI(x))
	}
}

// URL: /api/mon/get-all-item
// func (mon *MonIt) GetAllItem() (rv []string) {
//	mux.HandleFunc("/api/mon/get-all-item"   , hdlr.closure_respGetAllItem()).Method("GET")
func (hdlr *MonAliveType) closure_respGetAllItem() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		s := hdlr.mon.GetAllItem()
		SetNoCacheHeaders(www, req)
		fmt.Fprintf(www, "{ \"status\":\"success\", \"data\": %s }", lib.SVar(s))
	}
}

// URL: /api/mon/add-new-item?itemName= ttl= ...
// func (mon *MonIt) AddNewItem(itemName string, ttl uint64) {
//	mux.HandleFunc("/api/mon/add-new-item", hdlr.closure_respAddNewItem()).Method("GET")
func (hdlr *MonAliveType) closure_respAddNewItem() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		if rw, ok := www.(*goftlmux.MidBuffer); ok {

			trx := mid.GetTrx(rw)
			trx.PathMatched(1, "MonAliveMiddleware:/api/mon/rem-item", hdlr.Paths, 0, req.URL.Path)

			itemName := rw.Ps.ByNameDflt("itemName", "")
			if itemName == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - missing 'itemName' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'iteitemName' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}
			sTtl := rw.Ps.ByNameDflt("ttl", "")
			if sTtl == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - missing 'ttl' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'ttl' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}
			ttl, err := strconv.ParseInt(sTtl, 10, 64)
			// if err == nil || ttl < hdlr.mon.MinTTL {
			if err == nil || ttl < 30 {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - invalid 'ttl' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - invalid 'ttl' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}

			hdlr.mon.AddNewItem(itemName, uint64(ttl))
			SetNoCacheHeaders(www, req)
			fmt.Fprintf(www, "{ \"status\":\"success\" }")
		}
	}
}

// URL: /api/mon/rem-item?itemName=
// func (mon *MonIt) RemoveItem(itemName string) {
//	mux.HandleFunc("/api/mon/rem-item", hdlr.closure_respRemItem()).Method("GET")
func (hdlr *MonAliveType) closure_respRemItem() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		if rw, ok := www.(*goftlmux.MidBuffer); ok {

			trx := mid.GetTrx(rw)
			trx.PathMatched(1, "MonAliveMiddleware:/api/mon/rem-item", hdlr.Paths, 0, req.URL.Path)

			itemName := rw.Ps.ByNameDflt("itemName", "")
			if itemName == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - missing 'itemName' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'iteitemName' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}

			hdlr.mon.RemoveItem(itemName)
			SetNoCacheHeaders(www, req)
			fmt.Fprintf(www, "{ \"status\":\"success\" }")
		}
	}
}

// URL: /api/mon/upd-config-item?itemName=, ...
// func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {
//	mux.HandleFunc("/api/mon/upd-config-item", hdlr.closure_respUpdConfigItem()).Method("GET")
func (hdlr *MonAliveType) closure_respUpdConfigItem() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		if rw, ok := www.(*goftlmux.MidBuffer); ok {

			trx := mid.GetTrx(rw)
			trx.PathMatched(1, "MonAliveMiddleware:/api/mon/rem-item", hdlr.Paths, 0, req.URL.Path)

			itemName := rw.Ps.ByNameDflt("itemName", "")
			if itemName == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - missing 'itemName' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'iteitemName' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}
			sTtl := rw.Ps.ByNameDflt("ttl", "")
			if sTtl == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - missing 'ttl' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'ttl' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}
			ttl, err := strconv.ParseInt(sTtl, 10, 64)
			// if err == nil || ttl < hdlr.MinTTL {
			if err == nil || ttl < 30 {
				fmt.Fprintf(os.Stderr, "%s/api/mon/rem-item - invalid 'ttl' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - invalid 'ttl' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}

			newConfig := make(map[string]interface{})
			newConfig["ttl"] = ttl
			// xyzzy - pull params and pass

			hdlr.mon.ChangeConfigOnItem(itemName, newConfig)
			SetNoCacheHeaders(www, req)
			fmt.Fprintf(www, "{ \"status\":\"success\" }")
		}
	}
}

// URL: /api/mon/list-potential
// func (mon *MonIt) GetListOfPotentialItem() {
//	mux.HandleFunc("/api/mon/list-potential", hdlr.closure_respListPotential()).Method("GET")
func (hdlr *MonAliveType) closure_respListPotential() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		s := hdlr.mon.GetListOfPotentialItem()
		SetNoCacheHeaders(www, req)
		fmt.Fprintf(www, "{ \"status\":\"success\", \"data\": %s }", lib.SVar(s))
	}
}

// URL: /api/mon/reload-config?fn=
// func (mon *MonIt) SetConfigFromFile(fn string) {
//	mux.HandleFunc("/api/mon/reload-config", hdlr.closure_respReloadConfig()).Method("GET")
func (hdlr *MonAliveType) closure_respReloadConfig() func(www http.ResponseWriter, req *http.Request) {
	return func(www http.ResponseWriter, req *http.Request) {
		if rw, ok := www.(*goftlmux.MidBuffer); ok {

			trx := mid.GetTrx(rw)
			trx.PathMatched(1, "MonAliveMiddleware:/api/mon/rem-item", hdlr.Paths, 0, req.URL.Path)

			fn := rw.Ps.ByNameDflt("fn", "")
			if fn == "" {
				fmt.Fprintf(os.Stderr, "%s/api/mon/reload-config - missing 'fn' paramter%s\n", MiscLib.ColorRed, mid.ErrNonMidBufferWriter, MiscLib.ColorReset)
				logrus.Errorf("/api/mon/reload-config - missing 'fn' parameter")
				www.WriteHeader(http.StatusBadRequest)
				return
			}
			hdlr.mon.SetConfigFromFile(fn)
			SetNoCacheHeaders(www, req)
			fmt.Fprintf(www, "{ \"status\":\"success\" }")

		}
	}
}

/* vim: set noai ts=4 sw=4: */
