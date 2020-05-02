Notification from notif-micro-service
	1. Use Email -> SMS
	2. Look at notif:{{.name}} keys where names are from live-mon etc.
	3. Have ping-alive run a 2nd Go routine - with it's own connectio to Redis
		- when things are down - 
			increment notif:{{.name}}
		- when things are up -
			set notif:{{.name} 0
	4. Add to data a "notif-threshold", "remediate-threshold"

