package cdp

import (
	"fmt"
	"strings"
)

func (c *connection) networkEvents() {
	for _, t := range c.targets {
		for _, e := range t.page.DrainNetworkEvents() {
			id := t.id + "-request-" + e.ID
			if e.Type == "Document" {
				id = "loader-" + fmt.Sprint(t.contextID)
			}
			if t.responseIDs == nil {
				t.responseIDs = map[string]string{}
			}
			t.responseIDs[id] = e.ID
			for session, enabled := range t.network {
				if !enabled {
					continue
				}
				common := map[string]any{"requestId": id, "loaderId": "loader-" + fmt.Sprint(t.contextID), "timestamp": e.Timestamp, "type": e.Type, "frameId": t.id}
				switch e.Phase {
				case "request":
					common["wallTime"] = e.Timestamp
					common["initiator"] = map[string]string{"type": "other"}
					common["documentURL"] = e.URL
					common["request"] = map[string]any{"url": e.URL, "method": e.Method, "headers": e.Headers, "initialPriority": "Medium", "referrerPolicy": "no-referrer-when-downgrade"}
					c.event("Network.requestWillBeSent", session, common)
				case "response":
					mime := ""
					for k, v := range e.Headers {
						if strings.EqualFold(k, "Content-Type") {
							mime = strings.Split(v, ";")[0]
						}
					}
					common["response"] = map[string]any{"url": e.URL, "status": e.Status, "statusText": "", "headers": e.Headers, "mimeType": mime, "protocol": "http/1.1", "connectionReused": false, "connectionId": 0, "encodedDataLength": 0, "securityState": "unknown"}
					c.event("Network.responseReceived", session, common)
				case "finished":
					common["encodedDataLength"] = e.Size
					c.event("Network.loadingFinished", session, common)
				case "failed":
					common["errorText"] = e.Error
					common["canceled"] = strings.Contains(e.Error, "canceled")
					c.event("Network.loadingFailed", session, common)
				}
			}
		}
	}
}
