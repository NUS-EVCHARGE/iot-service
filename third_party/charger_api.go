package third_party

import (
	"fmt"

	"github.com/NUS-EVCHARGE/proto-utils/dto"
	"github.com/NUS-EVCHARGE/proto-utils/utils"
)

// const url = "https://jsx85ddz0a.execute-api.ap-southeast-1.amazonaws.com/evcharger-gateway/∂∂provider/api/v1/provider/charger"
const url = "http://localhost:8080/admin/charger"

type BasicAuthCredentials struct {
	Username string `json:"basic_username"`
	Password string `json:"basic_password"`
}

func GetCharger(chargerId int) (dto.Charger, error) {
	sm := utils.SecretManager{
		Region:     "ap-southeast-1",
		SecretName: "evapp_basic_auth",
	}
	var authCred BasicAuthCredentials
	sm.GetSecrets(&authCred)
	res, err := utils.LaunchHttpRequest("GET", url, map[string]string{"id": fmt.Sprintf("%v", chargerId)}, map[string]interface{}{}, utils.WithBasicAuth(
		authCred.Username, authCred.Password,
	))
	if err != nil {
		return dto.Charger{}, err
	}
	var charger dto.Charger
	utils.HttpResponseDefaultParser(res, &charger)
	return charger, nil
}

func UpdateCharger(chargerDetails dto.Charger) error {
	sm := utils.SecretManager{
		Region:     "ap-southeast-1",
		SecretName: "evapp_basic_auth",
	}
	var authCred BasicAuthCredentials
	sm.GetSecrets(&authCred)
	res, err := utils.LaunchHttpRequest("PATCH", url+"/", map[string]string{}, chargerDetails, utils.WithBasicAuth(
		authCred.Username, authCred.Password,
	))
	if err != nil {
		return err
	}
	var charger dto.Charger
	err = utils.HttpResponseDefaultParser(res, &charger)
	if err != nil {
		return err
	}
	return nil
}
