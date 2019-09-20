package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"time"
)

// APIGroup desc
type APIGroup struct {
	GroupName        string            `json:"group_name"`
	LdapGroupName    string            `json:"ldap_group_name"`
	CustomProperties map[string]string `json:"custom_properties"`
	LeaseTime        int               `json:"lease_time"`
	CreateTime       int64             `json:"create_time"`
	CreateBy         string            `json:"create_by"`
}

// APIUser def
type APIUser struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	GroupName  string `json:"group_name"`
	ExpireTime int64  `json:"expire_time"`
	CreateTime int64  `json:"create_time"`
	CreateBy   string `json:"create_by"`
}

func getAPIGroups(e *Env) ([]APIGroup, error) {
	var groups = []APIGroup{}
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	for _, ldapGroup := range e.ldapGroups {
		req, err := http.NewRequest("GET", e.apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("X-API-KEY", e.apiKey)
		req.URL.Path = path.Join(req.URL.Path, fmt.Sprintf("/ldap-groups/%s/groups", ldapGroup))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var group = []APIGroup{}
		err = json.Unmarshal(body, &group)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group...)
	}
	return groups, nil
}

func getAPIUser(e *Env, groupName string) ([]APIUser, error) {
	var user = []APIUser{}
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("GET", e.apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-API-KEY", e.apiKey)
	req.URL.Path = path.Join(req.URL.Path, fmt.Sprintf("/groups/%s/users", groupName))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, err
	}
	return user, nil
}
