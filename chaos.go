// Package chaos provides access to Andrews and Arnold's CHAOS v2 API.
package chaos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultEndpoint = "https://chaos2.aa.net.uk"

// API provides the accessors for querying the CHAOS service.
type API struct {
	Endpoint string
	login    url.Values
}

// New takes an Auth with API credentials and returns an API object.
func New(auth Auth) *API {
	return &API{Endpoint: defaultEndpoint, login: auth.form()}
}

// Auth is the authentication credentials for the API.
//
// The API requires either account authentication (AccountNumber and AccountPassword) or control authentication (ControlLogin and ControlPassword.)
//
// ControlLogin may also be passed when using account authentication.
type Auth struct {
	AccountNumber   string
	AccountPassword string
	ControlLogin    string
	ControlPassword string
}

// Construct form values for sending as authentication data.
func (a Auth) form() url.Values {
	f := url.Values{}
	if a.AccountNumber != "" {
		f.Set("account_number", a.AccountNumber)
	}
	if a.AccountPassword != "" {
		f.Set("account_password", a.AccountPassword)
	}
	if a.ControlLogin != "" {
		f.Set("control_login", a.ControlLogin)
	}
	if a.ControlPassword != "" {
		f.Set("control_password", a.ControlPassword)
	}
	return f
}

func (api API) makeRequest(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", api.Endpoint+url, strings.NewReader(api.login.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response code: %d", resp.StatusCode)
	}

	return body, nil
}

// The API returns timestamps in the format "YYYY-mm-dd HH:mm:ss" rather than RFC3389.
//
// Use a custom type to unmarshal JSON to this format.
type chaosTime struct {
	time.Time
}

func (t *chaosTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	// The API returns times in UK local rather than UTC
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		loc = time.Local
	}
	nt, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		return err
	}
	t.Time = nt
	return nil
}

// BroadbandInfo represents information about a broadband line.
type BroadbandInfo struct {
	ID             int       `json:"id,string"`
	Login          string    `json:"login"`
	Postcode       string    `json:"postcode"`
	TXRate         int       `json:"tx_rate,string"`
	RXRate         int       `json:"rx_rate,string"`
	TXRateAdjusted int       `json:"tx_rate_adjusted,string"`
	QuotaMonthly   int       `json:"quota_monthly,string"`
	QuotaRemaining int       `json:"quota_remaining,string"`
	QuotaTimestamp chaosTime `json:"quota_timestamp"`
}

// BroadbandInfo fetches broadband info.
func (api API) BroadbandInfo() ([]BroadbandInfo, error) {
	resp, err := api.makeRequest("/broadband/info")
	if err != nil {
		return nil, err
	}
	r := struct {
		Info  []BroadbandInfo `json:"info"`
		Error string          `json:"error"`
	}{}
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return nil, fmt.Errorf("BroadbandInfo JSON decode: %w", err)
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}
	return r.Info, nil
}

// BroadbandQuota is quota.
type BroadbandQuota struct {
	ID             int       `json:"id,string"`
	QuotaMonthly   int       `json:"quota_monthly"`
	QuotaRemaining int       `json:"quota_remaining,string"`
	QuotaTimestamp chaosTime `json:"quota_timestamp,string"`
}

// BroadbandQuota fetches the broadband quota.
func (api API) BroadbandQuota() ([]BroadbandQuota, error) {
	resp, err := api.makeRequest("/broadband/quota")
	if err != nil {
		return nil, err
	}
	r := struct {
		Quota []BroadbandQuota `json:"quota"`
		Error string           `json:"error"`
	}{}
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return nil, fmt.Errorf("BroadbandQuota JSON decode: %w", err)
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}
	return r.Quota, nil
}
