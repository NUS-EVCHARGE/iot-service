package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var addr = flag.String("addr", ":8080", "http service address")
var chargerIdMap = map[string]Instance{}
var mapLock sync.Mutex

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
} // use default options

type IoTHubRequest struct {
	Command   string `json:"command"`
	CompanyId string `json:"company_id"`
	ChargerId string `json:"charger_id"`
	Status    Status `json:"status"`
}

type Instance struct {
	conn        *websocket.Conn
	ChargerId   string
	status      Status // available, charging
	messageType int
}

type Status int

const (
	Available Status = iota
	Pending
	Charging
	Offline
	Error
)

// get charger status
func getChargerStatus(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		chargerId := r.URL.Query().Get("charger_id")
		if charger, ok := chargerIdMap[chargerId]; ok {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(charger.status)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode("charger not found")
	case "POST":
		logrus.Info("setting_charger_status")
		var bodyBytes []byte
		var err error
		var req IoTHubRequest
		mapLock.Lock()
		defer mapLock.Unlock()

		if r.Body != nil {
			bodyBytes, err = ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Printf("Body reading error: %v", err)
				return
			}
			defer r.Body.Close()
		}

		if err = json.Unmarshal(bodyBytes, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode("payload unmarshall failed")
		}

		if charger, ok := chargerIdMap[req.ChargerId]; ok {
			charger.status = req.Status
			chargerIdMap[req.ChargerId] = charger
			w.WriteHeader(http.StatusOK)
			// requires to add charger status validation
			json.NewEncoder(w).Encode(charger.status)
			charger.conn.WriteMessage(charger.messageType, []byte(fmt.Sprintf("%v", charger.status)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode("charger not found")
	}
}

func getServiceHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("service is up and running")
}

// communication with charging point
func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		// business logic here

		if err != nil {
			logrus.WithField("err", err).Error("failed_to_read_message_from_client")
			break
		}

		// parse message as json
		var req IoTHubRequest
		err = json.Unmarshal(message, &req)
		if err != nil {
			logrus.WithField("req", string(message)).WithField("err", err).Error("failed_to_unmarshall_client_request")
		}
		mapLock.Lock()
		defer mapLock.Unlock()
		switch req.Command {
		case "register":
			chargerIdMap[req.ChargerId] = Instance{
				conn:        c,
				ChargerId:   req.ChargerId,
				status:      Pending,
				messageType: mt,
			}
			if err := c.WriteMessage(mt, []byte("instance register success")); err != nil {
				logrus.WithField("err", err).Error("failed_to_write_message_to_client")
			}
		case "unregister":
			if _, ok := chargerIdMap[req.ChargerId]; ok {
				delete(chargerIdMap, req.ChargerId)
				if err := c.WriteMessage(mt, []byte("instance unregister success")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
				}
			} else {
				logrus.WithField("req", req).Error("instance_does_not_exist")
			}
		default:
			// status switching
			if instance, ok := chargerIdMap[req.ChargerId]; ok {
				instance.status = req.Status
				chargerIdMap[req.ChargerId] = instance
				if err := c.WriteMessage(mt, []byte("instance status update success")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
				}
			} else {
				err := c.WriteMessage(mt, []byte("instance not registered"))
				logrus.WithField("err", err).Error("failed_to_write_message_to_client")
			}

		}
		mapLock.Unlock()
	}
}

func main() {
	flag.Parse()
	http.HandleFunc("/ws", wsEndpoint)
	http.HandleFunc("/charger", getChargerStatus)
	http.HandleFunc("iot/health", getServiceHealthCheck)

	logrus.Info("serving_iot_service...")
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
