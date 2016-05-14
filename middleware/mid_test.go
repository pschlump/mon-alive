//
// Monitor Middlware Library
//
// Copyright (C) Philip Schlump, 2014-2016
//

package MonAliveMiddleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pschlump/Go-FTL/server/goftlmux"
	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/Go-FTL/server/mid"
	"github.com/pschlump/mon-alive/qdemolib"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------
func Test_JsonPPathServer(t *testing.T) {
	tests := []struct {
		cmd            string
		setup_cmd      string
		url            string
		expectedBody   string
		expectedStatus string // xyzzy - validate status on return
		key            string
		keyExists      string
		tf             string
	}{

		// Setup - delete monitor::fred
		// 0
		{
			cmd: "del_key",
			key: "monitor::fred",
		},

		// mux.HandleFunc("/api/mon/add-new-item", hdlr.closure_respAddNewItem()).Method("GET", "POST")
		// 1
		{
			cmd:            "get",
			url:            "http://example.com/api/mon/add-new-item?itemName=fred&ttl=60",
			expectedStatus: "success",
		},

		// 2
		{
			cmd:            "get",
			url:            "http://example.com/api/mon/i-am-alive?itemName=fred",
			expectedStatus: "success",
		},

		// 3 -- check that key in step (2) exists now
		{
			cmd:       "chk_key",
			key:       "monitor::fred",
			keyExists: "should-exists",
			tf:        "Check that key %s exists failed - from previous (2) test",
		},

		// mux.HandleFunc("/api/mon/i-failed", hdlr.closure_respIFailed()).Method("GET", "POST")
		// check that monitor::fred is gone
		// 4
		{
			cmd:            "get",
			url:            "http://example.com/api/mon/i-failed?itemName=fred",
			expectedStatus: "success",
		},

		// 5 -- check that key in step (4) has seased to exists now
		{
			cmd:       "chk_key",
			key:       "monitor::fred",
			keyExists: "should-not-exists",
			tf:        "Check that key %s has been removed failed - from previous (4) test",
		},

		// mux.HandleFunc("/api/mon/rem-item", hdlr.closure_respRemItem()).Method("GET", "POST")
		// 6
		{
			cmd:            "get",
			url:            "http://example.com/api/mon/rem-item?itemName=fred",
			expectedStatus: "success",
		},

		// 7
		{
			cmd: "del_key",
			key: "monitor::fred",
		},
	}

	bot := mid.NewConstHandler(`{"abc":"def"}`, "Content-Type", "application/json")
	ms := NewMonAliveMiddlwareServer(bot, []string{"/api/mon/"}, "../global_cfg.json")
	var err error
	lib.SetupTestCreateDirs()

	qdemolib.SetupRedisForTest("../global_cfg.json")
	conn, _ := qdemolib.GetRedisClient()

	for ii, test := range tests {

		switch test.cmd {

		case "del_key":
			conn.Cmd("DEL", test.key)

		case "chk_key":
			s, err := conn.Cmd("GET", test.key).Str()
			if err != nil {
				if test.keyExists == "should-exists" {
					tt := fmt.Sprintf(test.tf, test.key)
					t.Errorf("Test %2d, key check failed: %s\n", ii, tt)
				} else { // should-not-exist
				}
			} else {
				if test.keyExists == "should-exists" {
				} else { // should-not-exist
					tt := fmt.Sprintf(test.tf, test.key)
					t.Errorf("Test %2d, Value: %s - key check failed: %s\n", ii, s, tt)
				}
			}

		case "get":
			rec := httptest.NewRecorder()

			wr := goftlmux.NewMidBuffer(rec, nil)

			var req *http.Request

			req, err = http.NewRequest("GET", test.url, nil)
			if err != nil {
				t.Fatalf("Test %d: Could not create HTTP request: %v", ii, err)
			}
			lib.SetupTestMimicReq(req, "example.com")

			goftlmux.ParseQueryParamsReg(rec, req, &wr.Ps) //

			ms.ServeHTTP(wr, req)
			wr.FinalFlush()

			b := string(rec.Body.Bytes())
			if db81 {
				fmt.Printf("Output: %s\n", b) // { "status":"success" } -- xyzzy - parse and check status
			}

			fl, err := lib.JsonStringToString(b)
			if err != nil {
				t.Errorf("Test %2d, unable to parse return value of --->>>%s<<<---, error %s\n", ii, b, err)
			} else {
				if test.expectedStatus != "" {
					if fl["status"] != test.expectedStatus {
						t.Errorf("Test %2d, invalid return status, got %s, expected %s - returend %s\n", ii, fl["status"], test.expectedStatus, b)
					}
				}
			}

		//if b != test.expectedBody {
		//	t.Errorf("Error %2d, reject error got: %s, expected %s\n", ii, b, test.expectedBody)
		//}

		default:
			t.Errorf("Test %2d, Invalid test.cmd value %s\n", ii, test.cmd)
		}

	}

}

const db81 = false

/* vim: set noai ts=4 sw=4: */
