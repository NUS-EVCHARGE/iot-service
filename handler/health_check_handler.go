package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/NUS-EVCHARGE/iot-service/third_party"
	"github.com/NUS-EVCHARGE/proto-utils/dto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var chargerIdMap = map[string]*Instance{}
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
	conn             *websocket.Conn
	ChargerId        string
	status           string // available, charging
	messageType      int
	ChargerDetails   dto.Charger
	lastAccessedTime time.Time
	done             chan bool
}

type Status int

const (
	Available Status = iota
	Pending
	Charging
	Offline
	Error
)

func NewInstance(c *websocket.Conn, chargerId string) (*Instance, error) {
	chargerIdInt, err := strconv.Atoi(chargerId)

	if err != nil {
		return nil, err
	}
	chargerDetails, err := third_party.GetCharger(chargerIdInt)
	if err != nil {
		return nil, err
	}
	instance := Instance{
		conn:             c,
		ChargerId:        chargerId,
		status:           "Pending",
		ChargerDetails:   chargerDetails,
		lastAccessedTime: time.Now(),
		done:             make(chan bool, 1),
	}
	go func() {
		t := time.NewTicker(time.Second)
		for {
			select {
			case <-t.C:
				logrus.WithField("time", instance.lastAccessedTime).WithField("time_since", time.Since(instance.lastAccessedTime).Seconds()).Info("keep_alive_checker")
				if time.Since(instance.lastAccessedTime).Seconds() > 30 {
					instance.done <- true
				}
			case <-instance.done:
				logrus.Info("keep_alive_expired")
				delete(chargerIdMap, instance.ChargerId)
				return
			}
		}

	}()
	return &instance, nil
}

func GetChargerEndpointStatus(c *gin.Context) {
	chargerId := c.Query("charger_id")
	if charger, ok := chargerIdMap[chargerId]; ok {
		c.JSON(http.StatusOK, charger.status)
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
		// keepalive check
		switch req.Command {
		case "register":

			instance, err := NewInstance(c, req.ChargerId)
			if err != nil {
				c.WriteMessage(mt, []byte(fmt.Sprintf("err creating intance: %v", err)))
				continue
			}
			chargerIdMap[req.ChargerId] = instance

			if err := c.WriteMessage(mt, []byte("instance register success")); err != nil {
				logrus.WithField("err", err).Error("failed_to_write_message_to_client")
			}
			logrus.Info("register instance success")
		case "unregister":
			if instance, ok := chargerIdMap[req.ChargerId]; ok {
				instance.done <- true
				delete(chargerIdMap, req.ChargerId)
				if err := c.WriteMessage(mt, []byte("instance unregister success")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
				}
				logrus.Info("unregister instance success")

			} else {
				logrus.WithField("req", req).Error("instance_does_not_exist")
			}
		case "keepalive":
			if instance, ok := chargerIdMap[req.ChargerId]; !ok {
				// break connection
				if err := c.WriteMessage(mt, []byte("instance keepalive failed")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
					continue
				}
			} else {
				instance.lastAccessedTime = time.Now()
				if err := c.WriteMessage(mt, []byte("instance keepalive success")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
					continue
				}
				logrus.Info("keepalive instance success")
			}

		default:
			// status switching
			if instance, ok := chargerIdMap[req.ChargerId]; ok {
				instance.ChargerDetails.Status = req.Status
				chargerIdMap[req.ChargerId] = instance

				third_party.UpdateCharger(instance.ChargerDetails)
				if err := c.WriteMessage(mt, []byte("instance status update success")); err != nil {
					logrus.WithField("err", err).Error("failed_to_write_message_to_client")
				}
				logrus.WithField("status", instance.ChargerDetails.Status).Info("update instance success")
			} else {
				err := c.WriteMessage(mt, []byte("instance not registered"))
				logrus.WithField("err", err).Error("failed_to_write_message_to_client")
			}

		}
		mapLock.Unlock()
	}
}

// added function in handler for Terry's implementation of service health check
func GetServiceHealthCheck(gin *gin.Context) {
	gin.JSON(http.StatusOK, "service is up and running")
}

// this function was originally inside the provider_handler.go
func CreateResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"message": message,
	}
}
