package simpleforce

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSObject_AttributesField(t *testing.T) {
	obj := &SObject{}
	if obj.AttributesField() != nil {
		t.Fail()
	}

	obj.setType("Case")
	if obj.AttributesField().Type != "Case" {
		t.Fail()
	}

	obj.setType("")
	if obj.AttributesField().Type != "" {
		t.Fail()
	}
}

func TestSObject_Type(t *testing.T) {
	obj := &SObject{
		sobjectAttributesKey: SObjectAttributes{Type: "Case"},
	}
	if obj.Type() != "Case" {
		t.Fail()
	}

	obj.setType("CaseComment")
	if obj.Type() != "CaseComment" {
		t.Fail()
	}
}

func TestSObject_InterfaceField(t *testing.T) {
	obj := &SObject{}
	if obj.InterfaceField("test_key") != nil {
		t.Fail()
	}

	(*obj)["test_key"] = "hello"
	if obj.InterfaceField("test_key") == nil {
		t.Fail()
	}
}

func TestSObject_SObjectField(t *testing.T) {
	obj := &SObject{
		sobjectAttributesKey: SObjectAttributes{Type: "CaseComment"},
		"ParentId":           "__PARENT_ID__",
	}

	// Positive checks
	caseObj := obj.SObjectField("Case", "ParentId")
	if caseObj.Type() != "Case" {
		log.Println("Type mismatch")
		t.Fail()
	}
	if caseObj.StringField("Id") != "__PARENT_ID__" {
		log.Println("ID mismatch")
		t.Fail()
	}

	// Negative checks
	userObj := obj.SObjectField("User", "OwnerId")
	if userObj != nil {
		log.Println("Nil mismatch")
		t.Fail()
	}
}

func TestSObject_Describe(t *testing.T) {
	client := requireClient(t, true)
	meta := client.SObject("Case").Describe()
	if meta == nil {
		t.FailNow()
	} else {
		if (*meta)["name"].(string) != "Case" {
			t.Fail()
		}
	}
}

func TestSObject_Get(t *testing.T) {
	client := requireClient(t, true)

	// Search for a valid Case ID first.
	queryResult, err := client.Query("SELECT Id,OwnerId,Subject FROM CASE")
	if err != nil || queryResult == nil {
		log.Println(logPrefix, "query failed,", err)
		t.FailNow()
	}
	if queryResult.TotalSize < 1 {
		t.FailNow()
	}
	oid := queryResult.Records[0].ID()
	ownerID := queryResult.Records[0].StringField("OwnerId")

	// Positive
	obj, err := client.SObject("Case").Get(oid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.ID() != oid || obj.StringField("OwnerId") != ownerID {
		t.Fail()
	}

	// Positive 2
	obj = client.SObject("Case")
	if obj.StringField("OwnerId") != "" {
		t.Fail()
	}
	obj.setID(oid)
	obj, err = obj.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj.ID() != oid || obj.StringField("OwnerId") != ownerID {
		t.Fail()
	}

	// Negative 1
	obj, err = client.SObject("Case").Get("non-exist-id")
	if obj != nil {
		t.Fail()
	}
	if err == nil {
		t.Error("expected error for non-existent ID")
	}

	// Negative 2
	obj = &SObject{}
	_, err = obj.Get()
	if err == nil {
		t.Error("expected error for SObject without type/client")
	}
}

func TestSObject_Create(t *testing.T) {
	client := requireClient(t, true)

	// Positive
	case1 := client.SObject("Case")
	case1.Set("Subject", "Case created by simpleforce on "+time.Now().Format("2006/01/02 03:04:05")).
		Set("Comments", "This case is created by simpleforce")
	case1Result, err := case1.Create()
	if err != nil {
		t.Fatalf("failed to create case: %v", err)
	}
	if case1Result == nil || case1Result.ID() == "" || case1Result.Type() != case1.Type() {
		t.Fail()
	} else {
		got, err := case1Result.Get()
		if err != nil {
			t.Fatalf("failed to get created case: %v", err)
		}
		log.Println(logPrefix, "Case created,", got.StringField("CaseNumber"))
	}

	// Positive 2
	caseComment1 := client.SObject("CaseComment")
	caseComment1.Set("ParentId", case1Result.ID()).
		Set("CommentBody", "This comment is created by simpleforce & used for testing").
		Set("IsPublished", true)
	caseComment1Result, err := caseComment1.Create()
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}
	commentObj, err := caseComment1Result.Get()
	if err != nil {
		t.Fatalf("failed to get created comment: %v", err)
	}
	if commentObj.SObjectField("Case", "ParentId").ID() != case1Result.ID() {
		t.Fail()
	} else {
		log.Println(logPrefix, "CaseComment created,", caseComment1Result.ID())
	}

	// Negative: object without type.
	obj := client.SObject()
	_, err = obj.Create()
	if err == nil {
		t.Error("expected error for object without type")
	}

	// Negative: object without client.
	obj = &SObject{}
	_, err = obj.Create()
	if err == nil {
		t.Error("expected error for object without client")
	}

	// Negative: Invalid type
	obj = client.SObject("__SOME_INVALID_TYPE__")
	_, err = obj.Create()
	if err == nil {
		t.Error("expected error for invalid type")
	}

	// Negative: Invalid field
	obj = client.SObject("Case").Set("__SOME_INVALID_FIELD__", "")
	_, err = obj.Create()
	if err == nil {
		t.Error("expected error for invalid field")
	}
}

func TestSObject_Update(t *testing.T) {
	client := requireClient(t, true)

	// Positive
	created, err := client.SObject("Case").
		Set("Subject", "Case created by simpleforce on "+time.Now().Format("2006/01/02 03:04:05")).
		Create()
	if err != nil || created == nil {
		t.Fatalf("failed to create case: %v", err)
	}
	updated, err := created.
		Set("Subject", "Case subject updated by simpleforce").
		Update()
	if err != nil || updated == nil {
		t.Fatalf("failed to update case: %v", err)
	}
	got, err := updated.Get()
	if err != nil {
		t.Fatalf("failed to get updated case: %v", err)
	}
	if got.StringField("Subject") != "Case subject updated by simpleforce" {
		t.Fail()
	}
}

func TestSObject_Upsert(t *testing.T) {
	client := requireClient(t, true)

	// Positive create new object through upsert
	case1 := client.SObject("Case")
	case1Result, err := case1.Set("Subject", "Case created by simpleforce on "+time.Now().Format("2006/01/02 03:04:05")).
		Set("Comments", "This case is created by simpleforce").
		Set("customExtIdField__c", uuid.NewString()).
		Set("ExternalIDField", "customExtIdField__c").
		Upsert()
	if err != nil || case1Result == nil || case1Result.ID() == "" || case1Result.Type() != case1.Type() {
		t.Fatalf("upsert failed: %v", err)
	}
	got, err := case1Result.Get()
	if err != nil {
		t.Fatalf("failed to get upserted case: %v", err)
	}
	log.Println(logPrefix, "Case created,", got.StringField("CaseNumber"))

	// Positive update existing object through upsert
	case2 := client.SObject("Case").
		Set("Subject", "Case created by simpleforce on "+time.Now().Format("2006/01/02 03:04:05")).
		Set("customExtIdField__c", uuid.NewString())
	case2Result, err := case2.Create()
	if err != nil {
		t.Fatalf("failed to create case2: %v", err)
	}
	_, err = case2.
		Set("Subject", "Case subject updated by simpleforce").
		Set("ExternalIDField", "customExtIdField__c").
		Upsert()
	if err != nil {
		t.Fatalf("upsert update failed: %v", err)
	}
	got2, err := case2Result.Get()
	if err != nil {
		t.Fatalf("failed to get upserted case2: %v", err)
	}
	if got2.StringField("Subject") != "Case subject updated by simpleforce" {
		t.Fail()
	} else {
		got2b, err := case2Result.Get()
		if err != nil {
			t.Fatalf("failed to get case2 again: %v", err)
		}
		log.Println(logPrefix, "Case updated,", got2b.StringField("CaseNumber"))
	}

	// Negative: object without type.
	obj := client.SObject()
	_, err = obj.Upsert()
	if err == nil {
		t.Error("expected error for object without type")
	}

	// Negative: object without client.
	obj = &SObject{}
	_, err = obj.Upsert()
	if err == nil {
		t.Error("expected error for object without client")
	}

	// Negative: Invalid type
	obj = client.SObject("__SOME_INVALID_TYPE__").
		Set("ExternalIDField", "customExtIdField__c").
		Set("customExtIdField__c", uuid.NewString())
	_, err = obj.Upsert()
	if err == nil {
		t.Error("expected error for invalid type")
	}

	// Negative: Invalid field
	obj = client.SObject("Case").
		Set("ExternalIDField", "customExtIdField__c").
		Set("customExtIdField__c", uuid.NewString()).
		Set("__SOME_INVALID_FIELD__", "")
	_, err = obj.Upsert()
	if err == nil {
		t.Error("expected error for invalid field")
	}

	// Negative: Missing ext ID
	obj = client.SObject("Case").
		Set("ExternalIDField", "customExtIdField__c")
	_, err = obj.Upsert()
	if err == nil {
		t.Error("expected error for missing external ID")
	}
}

func TestSObject_Delete(t *testing.T) {
	client := requireClient(t, true)

	// Positive: create a case first then delete it and verify if it is gone.
	created, err := client.SObject("Case").
		Set("Subject", "Case created by simpleforce on "+time.Now().Format("2006/01/02 03:04:05")).
		Create()
	if err != nil || created == nil || created.ID() == "" {
		t.Fatalf("failed to create case: %v", err)
	}
	case1, err := created.Get()
	if err != nil {
		t.Fatalf("failed to get created case: %v", err)
	}
	if case1 == nil || case1.ID() == "" {
		t.Fatal()
	}
	caseID := case1.ID()
	if case1.Delete() != nil {
		t.Fail()
	}
	case1, err = client.SObject("Case").Get(caseID)
	if case1 != nil {
		t.Fail()
	}
	// After deletion, Get should return an error (404)
	if err == nil {
		t.Error("expected error when getting deleted case")
	}
}

// TestSObject_GetUpdate validates updating of existing records.
func TestSObject_GetUpdate(t *testing.T) {
	client := requireClient(t, true)

	// Create a new case first.
	created, err := client.SObject("Case").
		Set("Subject", "Original").
		Create()
	if err != nil || created == nil {
		t.Fatalf("failed to create case: %v", err)
	}
	case1, err := created.Get()
	if err != nil {
		t.Fatalf("failed to get created case: %v", err)
	}

	// Query the case by ID, then update the Subject.
	case2, err := client.SObject("Case").Get(case1.ID())
	if err != nil {
		t.Fatalf("failed to get case by ID: %v", err)
	}
	updated, err := case2.
		Set("Subject", "Updated").
		Update()
	if err != nil || updated == nil {
		t.Fatalf("failed to update case: %v", err)
	}
	case2got, err := updated.Get()
	if err != nil {
		t.Fatalf("failed to get updated case: %v", err)
	}

	// Query the case by ID again and check if the Subject has been updated.
	case3, err := client.SObject("Case").Get(case2got.ID())
	if err != nil {
		t.Fatalf("failed to get case3: %v", err)
	}

	if case3.StringField("Subject") != "Updated" {
		t.Fail()
	}

	user1, _ := client.SObject("User").Create()
	if user1 != nil {
		log.Println(user1.ID())
	}
}

func TestSObject_TimeField(t *testing.T) {
	obj := &SObject{}
	if obj.InterfaceField("test_key") != nil {
		t.Fail()
	}

	timestamp := time.Date(2024, 1, 12, 23, 15, 30, 0, time.UTC)
	(*obj)["test_key"] = "2024-01-12T23:15:30.000+0000"
	if v := obj.TimeField("test_key"); v.Unix() != timestamp.Unix() {
		t.Error("Time mismatch")
	}
}

// newTestClient creates a Client wired to a test HTTP server, suitable for unit tests.
// The caller provides a handler that simulates Salesforce responses.
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL, "test-client-id", "55.0")
	client.sessionID = "test-session-id"
	client.instanceURL = server.URL
	return client, server
}

func TestGetWithContext_MissingType(t *testing.T) {
	// SObject with no type set
	obj := &SObject{}
	result, err := obj.GetWithContext(context.Background(), "some-id")
	if result != nil {
		t.Error("expected nil result for missing type")
	}
	if !errors.Is(err, ErrObjectTypeMissing) {
		t.Errorf("expected ErrObjectTypeMissing, got %v", err)
	}
}

func TestGetWithContext_MissingClient(t *testing.T) {
	// SObject with type but no client
	obj := &SObject{}
	obj.setType("Case")
	result, err := obj.GetWithContext(context.Background(), "some-id")
	if result != nil {
		t.Error("expected nil result for missing client")
	}
	if !errors.Is(err, ErrObjectClientMissing) {
		t.Errorf("expected ErrObjectClientMissing, got %v", err)
	}
}

func TestGetWithContext_MissingID(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when ID is missing")
	})
	defer server.Close()

	obj := client.SObject("Case")
	// No ID set, no ID passed as argument
	result, err := obj.GetWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing ID")
	}
	if !errors.Is(err, ErrObjectIDMissing) {
		t.Errorf("expected ErrObjectIDMissing, got %v", err)
	}
}

func TestGetWithContext_HTTPError(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `[{"message":"not found","errorCode":"NOT_FOUND"}]`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	result, err := obj.GetWithContext(context.Background(), "bad-id")
	if result != nil {
		t.Error("expected nil result for HTTP error")
	}
	if err == nil {
		t.Error("expected error for HTTP 404 response")
	}
	if !errors.Is(err, ErrHTTPRequest) {
		t.Errorf("expected ErrHTTPRequest, got %v", err)
	}
}

func TestGetWithContext_InvalidJSON(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `not valid json`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	result, err := obj.GetWithContext(context.Background(), "some-id")
	if result != nil {
		t.Error("expected nil result for invalid JSON")
	}
	if !errors.Is(err, ErrParseResponse) {
		t.Errorf("expected ErrParseResponse, got %v", err)
	}
}

func TestGetWithContext_Success(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"Id":"001ABC","Subject":"Test Case","attributes":{"type":"Case","url":"/services/data/v55.0/sobjects/Case/001ABC"}}`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	result, err := obj.GetWithContext(context.Background(), "001ABC")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result != obj {
		t.Error("expected result to be same pointer as obj (in-place update)")
	}
	if result.ID() != "001ABC" {
		t.Errorf("expected ID '001ABC', got '%s'", result.ID())
	}
	if result.StringField("Subject") != "Test Case" {
		t.Errorf("expected Subject 'Test Case', got '%s'", result.StringField("Subject"))
	}
}

func TestGetWithContext_SuccessWithExistingID(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"Id":"001ABC","Subject":"Test Case"}`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.setID("001ABC")
	// Call Get with no explicit ID - should use obj.ID()
	result, err := obj.GetWithContext(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID() != "001ABC" {
		t.Errorf("expected ID '001ABC', got '%s'", result.ID())
	}
}

func TestGet_DelegatesToGetWithContext(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"Id":"001ABC","Subject":"Test Case"}`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	result, err := obj.Get("001ABC")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID() != "001ABC" {
		t.Errorf("expected ID '001ABC', got '%s'", result.ID())
	}
}

// --- CreateWithContext unit tests ---

func TestCreateWithContext_MissingType(t *testing.T) {
	obj := &SObject{}
	result, err := obj.CreateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing type")
	}
	if !errors.Is(err, ErrObjectTypeMissing) {
		t.Errorf("expected ErrObjectTypeMissing, got %v", err)
	}
}

func TestCreateWithContext_MissingClient(t *testing.T) {
	obj := &SObject{}
	obj.setType("Case")
	result, err := obj.CreateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing client")
	}
	if !errors.Is(err, ErrObjectClientMissing) {
		t.Errorf("expected ErrObjectClientMissing, got %v", err)
	}
}

func TestCreateWithContext_MarshalFailure(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when marshal fails")
	})
	defer server.Close()

	obj := client.SObject("Case")
	// A channel value cannot be marshalled to JSON
	obj.Set("BadField", make(chan int))
	result, err := obj.CreateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for marshal failure")
	}
	if !errors.Is(err, ErrMarshalRequest) {
		t.Errorf("expected ErrMarshalRequest, got %v", err)
	}
}

func TestCreateWithContext_HTTPError(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `[{"message":"invalid","errorCode":"INVALID_FIELD"}]`)
	})
	defer server.Close()

	obj := client.SObject("Case").Set("Subject", "Test")
	result, err := obj.CreateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for HTTP error")
	}
	if !errors.Is(err, ErrHTTPRequest) {
		t.Errorf("expected ErrHTTPRequest, got %v", err)
	}
}

func TestCreateWithContext_ParseFailure(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `not valid json`)
	})
	defer server.Close()

	obj := client.SObject("Case").Set("Subject", "Test")
	result, err := obj.CreateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for parse failure")
	}
	if !errors.Is(err, ErrParseResponse) {
		t.Errorf("expected ErrParseResponse, got %v", err)
	}
}

func TestCreateWithContext_Success(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"001NEW","success":true}`)
	})
	defer server.Close()

	obj := client.SObject("Case").Set("Subject", "Test Case")
	result, err := obj.CreateWithContext(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result != obj {
		t.Error("expected result to be same pointer as obj")
	}
	if result.ID() != "001NEW" {
		t.Errorf("expected ID '001NEW', got '%s'", result.ID())
	}
}

func TestCreate_DelegatesToCreateWithContext(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"001NEW","success":true}`)
	})
	defer server.Close()

	obj := client.SObject("Case").Set("Subject", "Test")
	result, err := obj.Create()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID() != "001NEW" {
		t.Errorf("expected ID '001NEW', got '%s'", result.ID())
	}
}

// --- UpdateWithContext unit tests ---

func TestUpdateWithContext_MissingType(t *testing.T) {
	obj := &SObject{}
	result, err := obj.UpdateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing type")
	}
	if !errors.Is(err, ErrObjectTypeMissing) {
		t.Errorf("expected ErrObjectTypeMissing, got %v", err)
	}
}

func TestUpdateWithContext_MissingClient(t *testing.T) {
	obj := &SObject{}
	obj.setType("Case")
	result, err := obj.UpdateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing client")
	}
	if !errors.Is(err, ErrObjectClientMissing) {
		t.Errorf("expected ErrObjectClientMissing, got %v", err)
	}
}

func TestUpdateWithContext_MissingID(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when ID is missing")
	})
	defer server.Close()

	obj := client.SObject("Case")
	result, err := obj.UpdateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing ID")
	}
	if !errors.Is(err, ErrObjectIDMissing) {
		t.Errorf("expected ErrObjectIDMissing, got %v", err)
	}
}

func TestUpdateWithContext_MarshalFailure(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when marshal fails")
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.setID("001ABC")
	obj.Set("BadField", make(chan int))
	result, err := obj.UpdateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for marshal failure")
	}
	if !errors.Is(err, ErrMarshalRequest) {
		t.Errorf("expected ErrMarshalRequest, got %v", err)
	}
}

func TestUpdateWithContext_HTTPError(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `[{"message":"invalid","errorCode":"INVALID_FIELD"}]`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.setID("001ABC")
	obj.Set("Subject", "Test")
	result, err := obj.UpdateWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for HTTP error")
	}
	if !errors.Is(err, ErrHTTPRequest) {
		t.Errorf("expected ErrHTTPRequest, got %v", err)
	}
}

func TestUpdateWithContext_Success(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.setID("001ABC")
	obj.Set("Subject", "Updated Subject")
	result, err := obj.UpdateWithContext(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result != obj {
		t.Error("expected result to be same pointer as obj (in-place update)")
	}
}

func TestUpdate_DelegatesToUpdateWithContext(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.setID("001ABC")
	obj.Set("Subject", "Test")
	result, err := obj.Update()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- UpsertWithContext unit tests ---

func TestUpsertWithContext_MissingType(t *testing.T) {
	obj := &SObject{}
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing type")
	}
	if !errors.Is(err, ErrObjectTypeMissing) {
		t.Errorf("expected ErrObjectTypeMissing, got %v", err)
	}
}

func TestUpsertWithContext_MissingClient(t *testing.T) {
	obj := &SObject{}
	obj.setType("Case")
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing client")
	}
	if !errors.Is(err, ErrObjectClientMissing) {
		t.Errorf("expected ErrObjectClientMissing, got %v", err)
	}
}

func TestUpsertWithContext_MissingExternalIDField(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when external ID field is missing")
	})
	defer server.Close()

	obj := client.SObject("Case")
	// Set external ID value but no field name
	obj.Set("customExtIdField__c", "some-value")
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing external ID field")
	}
	if !errors.Is(err, ErrExternalIDMissing) {
		t.Errorf("expected ErrExternalIDMissing, got %v", err)
	}
}

func TestUpsertWithContext_MissingExternalID(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when external ID is missing")
	})
	defer server.Close()

	obj := client.SObject("Case")
	// Set field name but no value for that field
	obj.Set("ExternalIDField", "customExtIdField__c")
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for missing external ID")
	}
	if !errors.Is(err, ErrExternalIDMissing) {
		t.Errorf("expected ErrExternalIDMissing, got %v", err)
	}
}

func TestUpsertWithContext_MarshalFailure(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made when marshal fails")
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.Set("BadField", make(chan int))
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for marshal failure")
	}
	if !errors.Is(err, ErrMarshalRequest) {
		t.Errorf("expected ErrMarshalRequest, got %v", err)
	}
}

func TestUpsertWithContext_HTTPError(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `[{"message":"invalid","errorCode":"INVALID_FIELD"}]`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.Set("Subject", "Test")
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for HTTP error")
	}
	if !errors.Is(err, ErrHTTPRequest) {
		t.Errorf("expected ErrHTTPRequest, got %v", err)
	}
}

func TestUpsertWithContext_ParseFailure(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// 201 means new record created, but response is invalid JSON
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `not valid json`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.Set("Subject", "Test")
	result, err := obj.UpsertWithContext(context.Background())
	if result != nil {
		t.Error("expected nil result for parse failure")
	}
	if !errors.Is(err, ErrParseResponse) {
		t.Errorf("expected ErrParseResponse, got %v", err)
	}
}

func TestUpsertWithContext_SuccessNewRecord(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		// 201 with ID means a new record was created
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"001NEW","success":true}`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.Set("Subject", "Test Case")
	result, err := obj.UpsertWithContext(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result != obj {
		t.Error("expected result to be same pointer as obj")
	}
	if result.ID() != "001NEW" {
		t.Errorf("expected ID '001NEW', got '%s'", result.ID())
	}
}

func TestUpsertWithContext_SuccessUpdatedRecord(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		// 204 with empty body means existing record was updated
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.setID("001EXISTING")
	obj.Set("Subject", "Updated Case")
	result, err := obj.UpsertWithContext(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result != obj {
		t.Error("expected result to be same pointer as obj (in-place update)")
	}
}

func TestUpsert_DelegatesToUpsertWithContext(t *testing.T) {
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"001NEW","success":true}`)
	})
	defer server.Close()

	obj := client.SObject("Case")
	obj.Set("ExternalIDField", "customExtIdField__c")
	obj.Set("customExtIdField__c", "ext-123")
	obj.Set("Subject", "Test")
	result, err := obj.Upsert()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID() != "001NEW" {
		t.Errorf("expected ID '001NEW', got '%s'", result.ID())
	}
}
