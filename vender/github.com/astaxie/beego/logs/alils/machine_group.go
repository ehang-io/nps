package alils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
)

// MachineGroupAttribute define the Attribute
type MachineGroupAttribute struct {
	ExternalName string `json:"externalName"`
	TopicName    string `json:"groupTopic"`
}

// MachineGroup define the machine Group
type MachineGroup struct {
	Name          string   `json:"groupName"`
	Type          string   `json:"groupType"`
	MachineIDType string   `json:"machineIdentifyType"`
	MachineIDList []string `json:"machineList"`

	Attribute MachineGroupAttribute `json:"groupAttribute"`

	CreateTime     uint32
	LastModifyTime uint32

	project *LogProject
}

// Machine define the Machine
type Machine struct {
	IP            string
	UniqueID      string `json:"machine-uniqueid"`
	UserdefinedID string `json:"userdefined-id"`
}

// MachineList define the Machine List
type MachineList struct {
	Total    int
	Machines []*Machine
}

// ListMachines returns machine list of this machine group.
func (m *MachineGroup) ListMachines() (ms []*Machine, total int, err error) {
	h := map[string]string{
		"x-sls-bodyrawsize": "0",
	}

	uri := fmt.Sprintf("/machinegroups/%v/machines", m.Name)
	r, err := request(m.project, "GET", uri, h, nil)
	if err != nil {
		return
	}

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	if r.StatusCode != http.StatusOK {
		errMsg := &errorMessage{}
		err = json.Unmarshal(buf, errMsg)
		if err != nil {
			err = fmt.Errorf("failed to remove config from machine group")
			dump, _ := httputil.DumpResponse(r, true)
			fmt.Println(dump)
			return
		}
		err = fmt.Errorf("%v:%v", errMsg.Code, errMsg.Message)
		return
	}

	body := &MachineList{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return
	}

	ms = body.Machines
	total = body.Total

	return
}

// GetAppliedConfigs returns applied configs of this machine group.
func (m *MachineGroup) GetAppliedConfigs() (confNames []string, err error) {
	confNames, err = m.project.GetAppliedConfigs(m.Name)
	return
}
