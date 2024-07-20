package simpleforce

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
)

var (
	// ErrFailure is a generic error if none of the other errors are appropriate.
	ErrFailure = errors.New("general failure")

	// ErrAuthentication is returned when authentication failed.
	ErrAuthentication = errors.New("authentication failure")
)

type jsonError []struct {
	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
}

type xmlError struct {
	Message   string `xml:"Body>Fault>faultstring"`
	ErrorCode string `xml:"Body>Fault>faultcode"`
}

type SalesforceError struct {
	Message      string
	HttpCode     int
	ErrorCode    string
	ErrorMessage string
}

func (err SalesforceError) Error() string {
	return err.Message
}

// Need to get information out of this package.
func ParseSalesforceError(statusCode int, responseBody []byte) (err error) {
	jsonErr := jsonError{}
	err = json.Unmarshal(responseBody, &jsonErr)
	if err == nil {
		return SalesforceError{
			Message: fmt.Sprintf(
				logPrefix+" Error. http code: %v Error Message:  %v Error Code: %v",
				statusCode, jsonErr[0].Message, jsonErr[0].ErrorCode,
			),
			HttpCode:     statusCode,
			ErrorCode:    jsonErr[0].ErrorCode,
			ErrorMessage: jsonErr[0].Message,
		}
	}

	xmlErr := xmlError{}
	err = xml.Unmarshal(responseBody, &xmlErr)
	if err == nil {
		return SalesforceError{
			Message: fmt.Sprintf(
				logPrefix+" Error. http code: %v Error Message:  %v Error Code: %v",
				statusCode, xmlErr.Message, xmlErr.ErrorCode,
			),
			HttpCode:     statusCode,
			ErrorCode:    xmlErr.ErrorCode,
			ErrorMessage: xmlErr.Message,
		}
	}

	return SalesforceError{
		Message:  string(responseBody),
		HttpCode: statusCode,
	}
}
