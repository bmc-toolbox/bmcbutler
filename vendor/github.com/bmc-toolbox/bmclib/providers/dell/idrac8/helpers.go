package idrac8

import (
	"encoding/json"
	"fmt"
)

// PUTs user config
func (i *IDrac8) putUser(userID int, user UserInfo) error {
	idracPayload := make(map[string]UserInfo)
	idracPayload["iDRAC.Users"] = user

	payload, err := json.Marshal(idracPayload)
	if err != nil {
		return fmt.Errorf("error unmarshaling User payload: %w", err)
	}

	endpoint := fmt.Sprintf("sysmgmt/2012/server/configgroup/iDRAC.Users.%d", userID)
	statusCode, _, err := i.put(endpoint, payload)
	if err != nil {
		return fmt.Errorf("PUT request to set User config returned error: %w", err)
	}

	if statusCode != 200 {
		return fmt.Errorf("PUT request to set User config returned status code: %d", statusCode)
	}

	return nil
}
