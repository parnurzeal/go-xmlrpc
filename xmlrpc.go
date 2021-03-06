package xmlrpc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

func Request(url string, method string, params ...interface{}) ([]interface{}, interface{}, string, error) {
	request := Serialize(method, params)
	//fmt.Println(request)
	buffer := bytes.NewBuffer([]byte(request))

	response, err := http.Post(url, "text/xml", buffer)
	if err != nil {
		return nil, nil, "", err
	}
	defer response.Body.Close()
	resp, fault, err := Unserialize(response.Body)
	if err != nil {
		return nil, nil, "", err
	}
	return resp, fault, request, nil
}

type MethodResponse struct {
	Params []Param `xml:"params>param"`
	Fault  Value   `xml:"fault>value"`
}

type Param struct {
	Value Value `xml:"value"`
}

type Value struct {
	List     []Value  `xml:"array>data>value"`
	Object   []Member `xml:"struct>member"`
	String   string   `xml:"string"`
	Int      string   `xml:"int"`
	Boolean  string   `xml:"boolean"`
	DateTime string   `xml:"dateTime.iso8601"`
	Double   string   `xml:"double"`
}

type Member struct {
	Name  string `xml:"name"`
	Value Value  `xml:"value"`
}

func unserialize(value Value) interface{} {
	if value.List != nil {
		result := make([]interface{}, len(value.List))
		for i, v := range value.List {
			result[i] = unserialize(v)
		}
		return result

	} else if value.Object != nil {
		result := make(map[string]interface{}, len(value.Object))
		for _, member := range value.Object {
			result[member.Name] = unserialize(member.Value)
		}
		return result

	} else if value.String != "" {
		return fmt.Sprintf("%s", value.String)

	} else if value.Int != "" {
		result, _ := strconv.Atoi(value.Int)
		return result

	} else if value.Boolean != "" {
		return value.Boolean == "1"

	} else if value.DateTime != "" {
		var format = "20060102T15:04:05"
		result, _ := time.Parse(format, value.DateTime)
		return result
	} else if value.Double != "" {
		result, _ := strconv.ParseFloat(value.Double, 64)
		return result
	}
	return nil
}

func Unserialize(buffer io.ReadCloser) ([]interface{}, interface{}, error) {
	body, err := ioutil.ReadAll(buffer)
	if err != nil {
		return nil, nil, err
	}
	var response MethodResponse
	xml.Unmarshal(body, &response)
	result := make([]interface{}, len(response.Params))
	for i, param := range response.Params {
		result[i] = unserialize(param.Value)
	}
	fault := unserialize(response.Fault)
	return result, fault, nil
}

func Serialize(method string, params []interface{}) string {
	request := "<methodCall>"
	request += fmt.Sprintf("<methodName>%s</methodName>", method)
	request += "<params>"

	for _, value := range params {
		request += "<param>"
		request += serialize(value)
		request += "</param>"
	}

	request += "</params></methodCall>"

	return request
}

func serialize(value interface{}) string {
	result := "<value>"
	switch value.(type) {
	case string:
		result += fmt.Sprintf("<string>%s</string>", value.(string))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		result += fmt.Sprintf("<int>%d</int>", value)
	case float32, float64:
		result += fmt.Sprintf("<float>%f</float>", value)
	case map[string]interface{}:
		result += "<struct>"
		for k, v := range value.(map[string]interface{}) {
			result += "<member>"
			result += fmt.Sprintf("<name>%s</name>", k)
			result += serialize(v)
			result += "</member>"
		}
		result += "</struct>"

	default:
		tmpVal := reflect.ValueOf(value)
		if tmpVal.Kind() == reflect.Map {
			result += "<struct>"
			for _, k := range tmpVal.MapKeys() {
				v := tmpVal.MapIndex(k)
				result += "<member>"
				result += fmt.Sprintf("<name>%s</name>", k.Interface())
				result += serialize(v.Interface())
				result += "</member>"
			}
			result += "</struct>"

		} else {
			log.Fatal("Cannot serialise: ", value)
		}
	}
	result += "</value>"
	return result
}
