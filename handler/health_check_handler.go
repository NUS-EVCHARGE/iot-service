package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	chargerCon "github.com/NUS-EVCHARGE/ev-provider-service/controller/charger"
	"github.com/NUS-EVCHARGE/ev-provider-service/dto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var chargerIdMap = map[string]Instance{}
var mapLock sync.Mutex

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
} // use default options

type IoTHubRequest struct {
	Command     string `json:"command"`
	Email       string `json:"email"`
	CompanyName string `json:"company_name"`
	ChargerId   string `json:"charger_id"`
	Status      string `json:"status"`
}

type Instance struct {
	conn        *websocket.Conn
	ChargerId   string
	status      string // available, charging
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

func GetChargerEndpointStatus(c *gin.Context) {
	chargerId := c.Query("charger_id")
	if charger, ok := chargerIdMap[chargerId]; ok {
		c.JSON(http.StatusOK, charger.status)
		return
	}
	c.JSON(http.StatusNotFound, "charger not found")
	return
}

// get charger status
func SetChargerEndpointStatus(c *gin.Context) {
	logrus.Info("setting_charger_status")
	var err error
	var req IoTHubRequest
	mapLock.Lock()
	defer mapLock.Unlock()

	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	if charger, ok := chargerIdMap[req.ChargerId]; ok {
		charger.status = req.Status
		chargerIdMap[req.ChargerId] = charger
		charger.conn.WriteMessage(charger.messageType, []byte(fmt.Sprintf("%v", charger.status)))

		// update db
		chargerResult, err := chargerCon.ChargerControllerObj.GetAllChargerByCompanyName(req.CompanyName)
		if err != nil {
			// todo: change to common library
			logrus.WithField("err", err).Error("error getting all charger")
			c.JSON(http.StatusBadRequest, CreateResponse(fmt.Sprintf("%v", err)))
			return
		}
		for _, cr := range chargerResult {
			if cr.UID == req.ChargerId {
				// matched UID
				updatedCharger := dto.Charger{
					ID:     cr.ID,
					Status: req.Status,
				}
				err := chargerCon.ChargerControllerObj.UpdateCharger(updatedCharger)
				if err != nil {
					logrus.WithField("err", err).Error("error updating charger")
					c.JSON(http.StatusBadRequest, CreateResponse(fmt.Sprintf("%v", err)))
					return
				}
				c.JSON(http.StatusOK, "charger updated sucessfully")
				return
			}
		}
		return
	}
	c.JSON(http.StatusNotFound, "charger not found")
	return
}

// communication with charging point
func WsChargerEndpoint(ginC *gin.Context) {
	c, err := upgrader.Upgrade(ginC.Writer, ginC.Request, nil)
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
		switch req.Command {
		case "register":
			chargerIdMap[req.ChargerId] = Instance{
				conn:        c,
				ChargerId:   req.ChargerId,
				status:      "Pending",
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

// added function in handler for Terry's implementation of service health check
func getServiceHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("service is up and running")
}

// this function was originally inside the provider_handler.go
func CreateResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"message": message,
	}
}
