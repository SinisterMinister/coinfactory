package binance

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func getRequest(method string, u *url.URL, obj interface{}) (*http.Request, error) {
	var (
		req *http.Request
		err error
	)
	if obj != nil {
		payload := structToMap(obj)
		body := strings.NewReader(payload.Encode())
		req, err = http.NewRequest(method, u.String(), body)
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-MBX-APIKEY", viper.GetString("binance.api_key"))
	return req, nil
}

func buildURL(uri string) *url.URL {
	u, err := url.Parse(viper.GetString("binance.rest_url") + uri)
	if err != nil {
		log.WithError(err).Fatal("Malformed value for 'binance.rest_url'!")
	}

	return u
}

func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   time.Second * 2,
		Transport: &http.Transport{},
	}
}

func generateSignature(body []byte) string {
	hash := hmac.New(sha256.New, []byte(viper.GetString("binance.secret_key")))
	hash.Write(body)

	return hex.EncodeToString(hash.Sum(nil))
}

func signRequest(req *http.Request) error {
	// Start with the query
	query := req.URL.Query()
	ts := getTimestampForPayload()

	// Add the timestamp
	query.Set("timestamp", getTimestampForPayload())

	log.WithFields(log.Fields{
		"localts":  time.Now().UnixNano() / time.Millisecond.Nanoseconds(),
		"serverts": ts,
	}).Debug("Setting request timestamp")

	// Generate the payload by combining the query params and the body, if any
	payload := query.Encode()
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		// Put the body back where you found it!
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		// Add the body to the payload
		payload += fmt.Sprintf("%s", body)
	}

	log.WithField("payload", payload).Debug("Generating signature using the payload")

	// Generate the signature and add it to the request
	signature := generateSignature([]byte(payload))
	// query.Set("signature", signature)
	req.URL.RawQuery = query.Encode() + "&signature=" + signature

	return nil
}

func signedGet(u *url.URL) (*http.Response, error) {
	// Build the URL
	req, err := getRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	// Sign the request
	signRequest(req)

	// Send the request and return the response
	return sendRequest(req)
}

func unsignedGet(u *url.URL) (*http.Response, error) {
	// Build the URL
	req, err := getRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	// Send the request and return the response
	return sendRequest(req)
}

func unsignedPost(u *url.URL, obj interface{}) (*http.Response, error) {
	req, err := getRequest("POST", u, obj)
	if err != nil {
		return nil, err
	}

	// Add the proper content type
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Send the request and return the response
	return sendRequest(req)
}

func signedPost(u *url.URL, obj interface{}) (*http.Response, error) {
	req, err := getRequest("POST", u, obj)
	if err != nil {
		return nil, err
	}

	// Sign the request
	signRequest(req)

	// Add the proper content type
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Send the request and return the response
	return sendRequest(req)
}

func unsignedPut(u *url.URL, obj interface{}) (*http.Response, error) {
	req, err := getRequest("PUT", u, obj)
	if err != nil {
		return nil, err
	}

	// Add the proper content type
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Send the request and return the response
	return sendRequest(req)
}

func unsignedDelete(u *url.URL) (*http.Response, error) {
	// Build the URL
	req, err := getRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// Send the request and return the response
	return sendRequest(req)
}

func signedDelete(u *url.URL) (*http.Response, error) {
	// Build the URL
	req, err := getRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// Sign the request
	signRequest(req)

	// Send the request and return the response
	return sendRequest(req)
}

func sendRequest(req *http.Request) (*http.Response, error) {
	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func unsignedGetString(u *url.URL) (string, error) {
	res, err := unsignedGet(u)
	if err != nil {
		return "", err
	}

	body, readerr := ioutil.ReadAll(res.Body)
	if readerr != nil {
		return "", readerr
	}

	log.Debug("Response from server:", res, body)
	return fmt.Sprintf("%s", body), nil
}

func unsignedGetJSON(u *url.URL, obj interface{}) error {
	res, err := unsignedGet(u)
	if err != nil {
		return err
	}

	jsonErr := getJSONResponse(res, obj)
	if jsonErr != nil {
		return jsonErr
	}

	return nil
}

func getJSONResponse(response *http.Response, obj interface{}) error {
	body, readerr := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if readerr != nil {
		return readerr
	}

	jsonErr := json.Unmarshal(body, &obj)
	if jsonErr != nil {
		return jsonErr
	}

	return nil
}
func structToMap(i interface{}) (values url.Values) {
	values = url.Values{}
	iVal := reflect.ValueOf(i)
	typ := iVal.Type()
	for i := 0; i < iVal.NumField(); i++ {
		f := iVal.Field(i)
		// You ca use tags here...
		tag := typ.Field(i).Tag.Get("json")
		if tag != "" {
			tag, opts := parseTag(tag)
			if opts.Contains("omitempty") && isEmptyValue(f) {
				continue
			}
			// Convert each type into a string for the url.Values string map
			var v string
			switch f.Interface().(type) {
			case int, int8, int16, int32, int64:
				v = strconv.FormatInt(f.Int(), 10)
			case uint, uint8, uint16, uint32, uint64:
				v = strconv.FormatUint(f.Uint(), 10)
			case float32:
				v = strconv.FormatFloat(f.Float(), 'f', 4, 32)
			case float64:
				v = strconv.FormatFloat(f.Float(), 'f', 4, 64)
			case []byte:
				v = string(f.Bytes())
			case string:
				v = f.String()
			case decimal.Decimal:
				v = f.Interface().(decimal.Decimal).String()
			}
			if v != "" {
				values.Set(tag, v)
			}
		}
	}
	return values
}

func getTimestampForPayload() string {
	serverTime := time.Now().Add(serverTimeDelta)
	serverTimestamp := strconv.FormatInt(serverTime.UnixNano()/time.Millisecond.Nanoseconds(), 10)

	return serverTimestamp
}

func calculateServerTimeDelta(rawTime int) time.Duration {
	serverTime := time.Unix(0, int64(rawTime*1000000))
	delta := time.Until(serverTime)
	return delta
}

// Taken from encoding/json

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	_, ok := v.Interface().(decimal.Decimal)
	if ok && v.Interface().(decimal.Decimal).Equals(decimal.NewFromFloat(0)) {
		return true
	}
	return false
}

func strictTagsUnmarshal(b []byte, i interface{}) error {
	var tmp interface{}

	// Unmarshal the json into something we can use
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	// Grab all the root level tags
	tags := map[string]bool{}
	iVal := reflect.ValueOf(i).Elem()
	typ := iVal.Type()
	for i := 0; i < iVal.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag != "" {
			tag, _ := parseTag(tag)
			tags[tag] = true
		}
	}

	// Remove any fields without a matching tag
	for k := range tmp.(map[string]interface{}) {
		if _, ok := tags[k]; !ok {
			// Remove unneeded value
			delete(tmp.(map[string]interface{}), k)
		}
	}

	// Unmarshal data
	filtered, err := json.Marshal(tmp.(map[string]interface{}))
	if err != nil {
		return err
	}
	return json.Unmarshal(filtered, i)
}
