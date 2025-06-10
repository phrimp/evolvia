package utils

import (
	"encoding/json"
	"log"

	"github.com/valyala/fasthttp"
)

func BuildRequest(req *fasthttp.Request, method string, body []byte, apiKey string, url string) {
	req.SetBody(body)
	req.Header.SetMethod(method)
	req.Header.Set("API_KEY", apiKey)
	req.Header.SetContentType("application/json")
	req.SetRequestURI(url)
}

func ForwardRequest(req *fasthttp.Request, url, method, api_key string, body []byte) any {
	BuildRequest(req, method, body, api_key, url)
	var resp *fasthttp.Response
	if err := fasthttp.Do(req, resp); err != nil {
		log.Printf("Error forwarding request: %v\n", err)
	}

	var responseData interface{}
	if err := json.Unmarshal(resp.Body(), &responseData); err != nil {
		log.Printf("Error parsing JSON response: %v\n", err)
	}
	return responseData
}
