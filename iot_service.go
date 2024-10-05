package main

import (
	"flag"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/NUS-EVCHARGE/iot-service/handler"
)

var (
	r *gin.Engine
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

func main() {

	// self-signed cert and key in same folder to support https and wss
	tlsCert := "mkcert.pem"
	tlsKey := "mkkey.pem"

	flag.Parse()

	r = gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*", "https://localhost:443"},
		AllowMethods:     []string{"PUT", "PATCH", "POST", "GET", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"authorization", "access-control-allow-origin", "Access-Control-Allow-Headers", "Origin", "Content-Length", "Content-Type", "authentication", "Access-Control-Request-Method", "Access-Control-Request-Headers"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	registerHandler()

	// amended r.Run to r.RunTLS with additional arguments to implement https and wss
	go r.Run(":9090")
	if err := r.RunTLS(":443", tlsCert, tlsKey); err != nil {
		logrus.WithField("error", err).Errorf("http server failed to start")
	}

}

func registerHandler() {
	// use to generate swagger ui
	//	@BasePath	/api/v1
	//	@title		IOT Service API
	//	@version	1.0
	//	@schemes	http

	// api versioning
	v1 := r.Group("/api/v1")
	{
		v1.GET("/ws", handler.WsChargerEndpoint)
		v1.GET("/charger/chargerstatus", handler.GetChargerEndpointStatus)
		v1.POST("/charger/chargerstatus", handler.SetChargerEndpointStatus)

		// additional API route for Terry's implementation of service health check
		v1.GET("/iot/health", handler.GetServiceHealthCheck)
	}

}
