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
)

// -----------------------------------------------------------------------------------------------------------------------------------------------
func Test_JsonPPathServer(t *testing.T) {
	tests := []struct {
		url          string
		expectedBody string
	}{
		// 0
		{
			url: "http://example.com/api/mon/i-am-alive?itemName=fred",
		},
	}

	bot := mid.NewConstHandler(`{"abc":"def"}`, "Content-Type", "application/json")
	ms := NewMonAliveMiddlwareServer(bot, []string{"/api/mon/"}, "../global_cfg.json")
	var err error
	lib.SetupTestCreateDirs()

	for ii, test := range tests {

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
		fmt.Printf("Output: %s\n", b) // { "status":"success" } -- xyzzy - parse and check status

		//if b != test.expectedBody {
		//	t.Errorf("Error %2d, reject error got: %s, expected %s\n", ii, b, test.expectedBody)
		//}

	}

}

/* vim: set noai ts=4 sw=4: */
