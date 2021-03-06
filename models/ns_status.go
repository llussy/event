package models

import (
	"strings"
	"sync"
	"time"

	"github.com/lodastack/event/common"
)

type (
	TAG   string
	NS    string
	ALARM string
	HOST  string

	TagStatus   map[TAG]Status
	HostStatus  map[HOST]TagStatus
	AlarmStatus map[ALARM]HostStatus
	NsStatus    map[NS]AlarmStatus
)

var StatusData = make(NsStatus)
var StatusMu sync.RWMutex

// GetNsStatusFromGlobal will read status from global object and return status by input param.
// The function will return thes status of the ns itself or its leaf child ns.
func GetNsStatusFromGlobal(nsStr string) NsStatus {
	ns := NS(nsStr)
	var output map[NS]AlarmStatus

	StatusMu.RLock()
	if ns == "" {
		output = StatusData
	} else {
		output = map[NS]AlarmStatus{}
		for _ns, alarmStatus := range StatusData {
			if !strings.HasSuffix("."+string(_ns), "."+string(ns)) {
				continue
			}
			output[_ns] = alarmStatus
		}
	}
	StatusMu.RUnlock()
	return output
}

type (
	// WalkResult is result of walk status.
	WalkResult map[NS]interface{}

	// WalkFunc is the type of the function called for each HostStatus visited by Walk.
	WalkFunc func(ns NS, alarmVersion ALARM, host HOST, tag TAG, status string, result WalkResult)
)

// Walk walks the status, calling walkFunc for each HostStatus.
// If one ns/alarm has no alarm/host status, Walk pass zero param to walkFunc.
func (s *NsStatus) Walk(walkFunc WalkFunc) WalkResult {
	result := make(map[NS]interface{}, len(*s))
	for ns, alarmStatus := range *s {
		if len(alarmStatus) == 0 {
			walkFunc(ns, "", "", "", common.OK, result)
			continue
		}
		for alarmVersion, hostsStatus := range alarmStatus {
			if len(hostsStatus) == 0 {
				walkFunc(ns, alarmVersion, "", "", common.OK, result)
				continue
			}
			for host, tagStatus := range hostsStatus {
				for tagString, tagStatus := range tagStatus {
					walkFunc(ns, alarmVersion, host, tagString, tagStatus.Level, result)
				}
			}
		}
	}
	return result
}

// GetNsStatus return map[string]bool reveal ns status.
// ns is identified by OK if has no alarmStatus.
func (s *NsStatus) GetNsStatus() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, tag TAG, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = true
		} else if status != common.OK {
			result[ns] = false
		}
	})
}

// getAlarmStatus return map[NS]map[ALARM]bool reveal alarm status.
// ns/alarm is identified by OK if has no hostStatus.
func (s *NsStatus) GetAlarmStatus() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, tag TAG, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[ALARM]bool)
		}

		if alarmVersion == "" {
			return
		}
		if _, exist := result[ns].(map[ALARM]bool)[alarmVersion]; !exist && status == common.OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = true
		} else if status != common.OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = false
		}
	})
}

// getNotOkHost return map[NS]map[HOST]bool reveal the not OK host.
func (s *NsStatus) GetNotOkHost() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, tag TAG, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[HOST]bool)
		}

		if host != "" && status != common.OK {
			result[ns].(map[HOST]bool)[host] = false
		}
	})
}

// GetStatusList return status list by level(OK, CRITICAL...).
func (s *NsStatus) GetStatusList(alarmVersion, host, level string) []Status {
	output := make([]Status, 0)
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	for _, alarmStatus := range *s {
		for _alarmVersion, hostStatus := range alarmStatus {
			if alarmVersion != "" && alarmVersion != string(_alarmVersion) {
				continue
			}
			for _host, hostStatus := range hostStatus {
				if host != "" && host != string(_host) {
					continue
				}
				for _, tagStatus := range hostStatus {
					if tagStatus.Level == "" {
						continue
					}
					tagStatus.LastTime = ((time.Since(tagStatus.CreateTime)) / time.Second)
					if level == "" || tagStatus.Level == level {
						output = append(output, tagStatus)
					}
				}
			}
		}
	}
	return output
}
