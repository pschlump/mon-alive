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
	}{
		// 0
		{
			cmd:      "SendIAmAlive",
			itemName: `bob`,
			chk:      "no-conf",
		},
		// 1
		{
			cmd:      "AddNewItem",
			itemName: `bob`,
			ttl:      60,
		},
		// 2
		{
			cmd:      "SendIAmAlive",
			itemName: `bob`,
			chk:      "conf",
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
			mon.AddNewItem(test.itemName, test.ttl)
		default:
			t.Errorf("Test %2d,  invalid test case, %s\n", ii, test.cmd)
		}
	}

}

/*
func (mon *MonIt) SendIAmShutdown(itemName string) {
func (mon *MonIt) GetNotifyItem() (rv []string) {
func (mon *MonIt) GetItemStatus() (rv []ItemStatus) {
func (mon *MonIt) GetAllItem() (rv []string) {
func (mon *MonIt) RemoveItem(itemName string) {
func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {
func (mon *MonIt) SetConfigFromFile(fn string) {
func (mon *MonIt) GetListOfPotentialItem() (rv []string) {
	! TODO !
* func (mon *MonIt) SendPeriodicIAmAlive(itemName string) {

		//if b != test.expectedBody {
		//	t.Errorf("Error %2d, reject error got: %s, expected %s\n", ii, b, test.expectedBody)
		//}

	// mon.SendPeriodicIAmAlive("Go-FTL")
	_ = mon
*/

const db50 = false

/* vim: set noai ts=4 sw=4: */
