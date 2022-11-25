package queries

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

const UrlFlagName = "url"

const getTimeout = time.Second * 5

func get(host string, resource string) string {
	u, err := url.Parse(host)
	if err != nil {
		log.Fatal("host parsing error: ", err)
	}

	u.Path = resource

	client := http.Client{
		Timeout: getTimeout,
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	return string(body)
}
