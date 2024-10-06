package handler

import (
	"github.com/NUS-EVCHARGE/proto-utils/utils"
	"github.com/gin-gonic/gin"
)

const url = "https://jsx85ddz0a.execute-api.ap-southeast-1.amazonaws.com/evcharger-gateway/provider/api/v1/provider/charger"

func UpdateChargerStatus(c *gin.Context) {

	utils.LaunchHttpRequest("PATCH", url, map[string]string{}, map[string]interface{}{})
}

func GetChargerDetails(c *gin.Context) {

}
