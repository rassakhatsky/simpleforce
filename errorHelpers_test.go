package simpleforce

import (
	"errors"
	"fmt"
	"testing"
)

var expectedError SalesforceError = SalesforceError{
	HttpCode:     417,
	Message:      logPrefix + " Error. http code: 417 Error Message:  something went wrong Error Code: SMTH_WRNG",
	ErrorCode:    "SMTH_WRNG",
	ErrorMessage: "something went wrong",
}

func TestSuccessfulJSONParse(t *testing.T) {
	response := `[
		{
			"message": "something went wrong",
			"errorCode": "SMTH_WRNG"
		}
	]`

	err := ParseSalesforceError(417, []byte(response))
	if err != expectedError {
		t.Errorf("failed to parse JSON error, got %s", err)
	}
}

func TestSuccessfulXMLParse(t *testing.T) {
	response := `
		<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
			<s:Body>
				<s:Fault>
					<faultcode>SMTH_WRNG</faultcode>
					<faultstring xml:lang="en-US">something went wrong</faultstring>
				</s:Fault>
			</s:Body>
		</s:Envelope>
	`
	err := ParseSalesforceError(417, []byte(response))
	if err != expectedError {
		t.Errorf("failed to parse XML error, got %s", err)
	}
}

func TestUnsuccessfulParse(t *testing.T) {
	response := "surprise!"
	unknownError := SalesforceError{
		HttpCode: 417,
		Message:  "surprise!",
	}

	err := ParseSalesforceError(417, []byte(response))
	if err != unknownError {
		t.Errorf("failed to parse unknown error, got %s", err)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrFailure", ErrFailure},
		{"ErrAuthentication", ErrAuthentication},
		{"ErrObjectTypeMissing", ErrObjectTypeMissing},
		{"ErrObjectClientMissing", ErrObjectClientMissing},
		{"ErrObjectIDMissing", ErrObjectIDMissing},
		{"ErrExternalIDMissing", ErrExternalIDMissing},
		{"ErrMarshalRequest", ErrMarshalRequest},
		{"ErrHTTPRequest", ErrHTTPRequest},
		{"ErrParseResponse", ErrParseResponse},
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a.err, b.err) {
				t.Errorf("sentinel errors %s and %s should be distinct, but errors.Is returned true", a.name, b.name)
			}
		}
	}
}

func TestSentinelErrors_ImplementErrorInterface(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrObjectTypeMissing", ErrObjectTypeMissing},
		{"ErrObjectClientMissing", ErrObjectClientMissing},
		{"ErrObjectIDMissing", ErrObjectIDMissing},
		{"ErrExternalIDMissing", ErrExternalIDMissing},
		{"ErrMarshalRequest", ErrMarshalRequest},
		{"ErrHTTPRequest", ErrHTTPRequest},
		{"ErrParseResponse", ErrParseResponse},
	}

	for _, s := range sentinels {
		if s.err == nil {
			t.Errorf("%s should not be nil", s.name)
		}
		if s.err.Error() == "" {
			t.Errorf("%s.Error() should return a non-empty string", s.name)
		}
	}
}

func TestSentinelErrors_WrappingPreservesIdentity(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrObjectTypeMissing", ErrObjectTypeMissing},
		{"ErrObjectClientMissing", ErrObjectClientMissing},
		{"ErrObjectIDMissing", ErrObjectIDMissing},
		{"ErrExternalIDMissing", ErrExternalIDMissing},
		{"ErrMarshalRequest", ErrMarshalRequest},
		{"ErrHTTPRequest", ErrHTTPRequest},
		{"ErrParseResponse", ErrParseResponse},
	}

	for _, s := range sentinels {
		wrapped := fmt.Errorf("%w: additional context", s.err)
		if !errors.Is(wrapped, s.err) {
			t.Errorf("wrapped %s should be identifiable via errors.Is", s.name)
		}
	}
}
