//
// Monitor Middlware Library
//
// Copyright (C) Philip Schlump, 2014-2016
//
package MonAliveLib

import (
	"fmt"
	"os"
	"testing"

	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------
func Test_MonAliveLib(t *testing.T) {

	tests := []struct {
		cmd      string
		expect   string
		itemName string
		ttl      uint64
		chk      string
		tf       string
	}{
		// 0
		{
			cmd:      "RemoveItem",
			itemName: `bob`,
		},
		// 1
		{
			cmd:      "SendIAmAlive",
			itemName: `bob`,
			chk:      "no-conf",
		},
		// 2
		{
			cmd:      "AddNewItem",
			itemName: `bob`,
			ttl:      60,
		},
		// 3
		{
			cmd:      "SendIAmAlive",
			itemName: `bob`,
			chk:      "conf",
		},
		// 4
		{
			cmd:      "GetNotifyItem",
			itemName: `bob`, // verify "bob" not in list
			chk:      "not-in",
		},
		// 5
		{
			cmd:      "GetItemStatus",
			itemName: `bob`,
			chk:      "up",
			tf:       "Inicates that test 4 (previous) (SendIAmAlive) did not work",
		},
		// 6
		{
			cmd:      "GetAllItem",
			itemName: `bob`,
		},
		// 7
		{
			cmd:      "SendIAmShutdown",
			itemName: `bob`,
		},
		// 8
		{
			cmd:      "GetNotifyItem",
			itemName: `bob`, // verify "bob" is not in list
			chk:      "not-in",
		},
		// 9
		{
			cmd:      "GetItemStatus",
			itemName: `bob`,
			chk:      "up",
			tf:       "Inicates that test 7 (2 back) (SendIAmShutdown) did not work",
		},
		// 10
		{
			cmd:      "AddNewItem",
			itemName: `bob`,
			ttl:      60,
		},
		// 11
		{
			cmd:      "SendIAmAlive",
			itemName: `bob`,
			chk:      "conf",
		},
		// 12
		{
			cmd:      "SendIFailed",
			itemName: `bob`,
		},
		// 13
		{
			cmd:      "GetNotifyItem",
			itemName: `bob`, // verify "bob" in list
			chk:      "in-list",
		},
		// 14
		{
			cmd:      "GetItemStatus",
			itemName: `bob`,
			chk:      "down",
			tf:       "Inicates that test 12 (2 back) (SendIFailed) did not work",
		},
		// 15+ - last item - remove to cleanup
		{
			cmd:      "RemoveItem",
			itemName: `bob`,
		},
	}

	qdemolib.SetupRedisForTest("../global_cfg.json")

	monClient, isCon := qdemolib.GetRedisClient()
	if !isCon {
		t.Fatalf("Error connecting to Redis - fatal\n")
		os.Exit(1)
	}
	mon := NewMonIt(func() *redis.Client { return monClient }, func(conn *redis.Client) {})

	conn, _ := qdemolib.GetRedisClient()

	for ii, test := range tests {

		switch test.cmd {
		case "SendIAmAlive":
			myStatus := make(map[string]interface{})
			myStatus["status"] = "ok"
			mon.SendIAmAlive(test.itemName, myStatus)
			s, err := conn.Cmd("GET", "monitor::bob").Str()
			if test.chk == "no-conf" {
				if err == nil {
					t.Errorf("Test %2d, Should not find key, did - not configured yet\n", ii)
				}
			}
			if test.chk == "conf" {
				if err != nil {
					t.Errorf("Test %2d, missing configured item in Reids\n", ii)
				}
			}
			if db50 {
				fmt.Printf("s= >%s<\n", s)
			}
		case "AddNewItem":
			mon.AddNewItem(test.itemName, test.ttl) // checked by "SendIAmAlive" above
		case "RemoveItem":
			mon.RemoveItem(test.itemName) // checked by "SendIAmAlive" above
		case "GetNotifyItem": // Depricate this interface
			rv := mon.GetNotifyItem()
			if db51 {
				fmt.Printf("GetNotifyItem rv= %s\n", lib.SVarI(rv))
			}
			if test.chk == "in-list" {
				if !lib.InArray(test.itemName, rv) {
					t.Errorf("Test %2d, Check to see if in-list, did not find it [%s], list %s\n", test.itemName, lib.SVarI(rv))
				}
			} else if test.chk == "not-in" {
				if lib.InArray(test.itemName, rv) {
					t.Errorf("Test %2d, Check to see if not-in-list, found it [%s], list %s\n", test.itemName, lib.SVarI(rv))
				}
			}
		case "GetItemStatus":
			rv := mon.GetItemStatus()
			if db52 {
				fmt.Printf("GetItemStatus rv= %s\n", lib.SVarI(rv))
			}
			// check that "bob" is "up"
			aStatus := StatusOf(test.itemName, rv)
			if test.chk != aStatus {
				t.Errorf("Test %2d, item status [%s] did not match expected [%s] -- %s\n", ii, aStatus, test.chk, test.tf)
			}
		case "GetAllItem":
			rv := mon.GetAllItem()
			if db51 {
				fmt.Printf("GetAllItem rv= %s\n", lib.SVarI(rv))
			}
			if !lib.InArray(test.itemName, rv) { // check that has "bob" as an item
				t.Errorf("Test %2d, missing 'bob' in items returned\n", ii)
			}
		case "SendIAmShutdown":
			mon.SendIAmShutdown(test.itemName)
		case "SendIFailed":
			mon.SendIFailed(test.itemName)
		default:
			t.Errorf("Test %2d,  invalid test case, %s\n", ii, test.cmd)
		}
	}

}

/*

func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {

Issue:
	func (mon *MonIt) GetNotifyItem() (rv []string) {

func (mon *MonIt) GetListOfPotentialItem() (rv []string) {
	! TODO !

Tested in other locaitons:
	* func (mon *MonIt) SendPeriodicIAmAlive(itemName string) {
	* func (mon *MonIt) SetConfigFromFile(fn string) {

--- note ---------------------------------------------------------------------------------------------------------

*/

const db50 = false
const db51 = false
const db52 = false

/* vim: set noai ts=4 sw=4: */
