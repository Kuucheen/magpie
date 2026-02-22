package support

import (
	"os"
	"strings"
	"sync"
)

const (
	envInstanceID     = "MAGPIE_INSTANCE_ID"
	envInstanceName   = "MAGPIE_INSTANCE_NAME"
	envInstanceRegion = "MAGPIE_INSTANCE_REGION"
)

var (
	instanceIDOnce   sync.Once
	instanceIDValue  string
	instanceNameOnce sync.Once
	instanceNameVal  string
	instanceRegOnce  sync.Once
	instanceRegVal   string
)

func GetInstanceID() string {
	instanceIDOnce.Do(func() {
		value := strings.TrimSpace(GetEnv(envInstanceID, ""))
		if value == "" {
			hostname, err := os.Hostname()
			if err == nil {
				value = strings.TrimSpace(hostname)
			}
		}
		if value == "" {
			value = "default"
		}
		instanceIDValue = value
	})
	return instanceIDValue
}

func GetInstanceName() string {
	instanceNameOnce.Do(func() {
		value := strings.TrimSpace(GetEnv(envInstanceName, ""))
		if value == "" {
			value = GetInstanceID()
		}
		instanceNameVal = value
	})
	return instanceNameVal
}

func GetInstanceRegion() string {
	instanceRegOnce.Do(func() {
		value := strings.TrimSpace(GetEnv(envInstanceRegion, ""))
		if value == "" {
			value = "Unknown"
		}
		instanceRegVal = value
	})
	return instanceRegVal
}
