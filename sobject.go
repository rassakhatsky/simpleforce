package simpleforce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	sobjectClientKey              = "__client__" // private attribute added to locate client instance.
	sobjectAttributesKey          = "attributes" // points to the attributes structure which should be common to all SObjects.
	sobjectIDKey                  = "Id"
	sobjectExternalIDFieldNameKey = "ExternalIDField"
)

// When updating existing records, certain fields are read only and needs to be removed before submitted to Salesforce.
// Following list of fields are extracted from INVALID_FIELD_FOR_INSERT_UPDATE error message.
var blacklistedUpdateFields = []string{
	"LastModifiedDate",
	"LastReferencedDate",
	"IsClosed",
	"ContactPhone",
	"CreatedById",
	"CaseNumber",
	"ContactFax",
	"ContactMobile",
	"IsDeleted",
	"LastViewedDate",
	"SystemModstamp",
	"CreatedDate",
	"ContactEmail",
	"ClosedDate",
	"LastModifiedById",
}

// SObject describes an instance of SObject.
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api_rest.meta/api_rest/resources_sobject_basic_info.htm
type SObject map[string]any

// SObjectMeta describes the metadata returned by describing the object.
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api_rest.meta/api_rest/resources_sobject_describe.htm
type SObjectMeta map[string]any

// SObjectAttributes describes the basic attributes (type and url) of an SObject.
type SObjectAttributes struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Describe queries the metadata of an SObject using the "describe" API.
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api_rest.meta/api_rest/resources_sobject_describe.htm
func (obj *SObject) Describe() *SObjectMeta {
	return obj.DescribeWithContext(context.Background())
}

func (obj *SObject) DescribeWithContext(ctx context.Context) *SObjectMeta {
	if obj.Type() == "" || obj.client() == nil {
		// Sanity check.
		return nil
	}
	url := obj.client().makeURL("sobjects/" + obj.Type() + "/describe")
	data, err := obj.client().httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	var meta SObjectMeta
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil
	}
	return &meta
}

// Get retrieves all the data fields of an SObject. If id is provided, the SObject with the provided external ID will
// be retrieved; otherwise, the existing ID of the SObject will be checked. If the SObject doesn't contain an ID field
// and id is not provided as the parameter, an error is returned.
// If query is successful, the SObject is updated in-place and the same pointer is returned; otherwise, nil and an error are returned.
func (obj *SObject) Get(id ...string) (*SObject, error) {
	return obj.GetWithContext(context.Background(), id...)
}

func (obj *SObject) GetWithContext(ctx context.Context, id ...string) (*SObject, error) {
	if obj.Type() == "" {
		return nil, fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
	}
	if obj.client() == nil {
		return nil, fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
	}

	oid := obj.ID()
	if len(id) > 0 {
		oid = id[0]
	}
	if oid == "" {
		return nil, fmt.Errorf("%w: no ID provided and SObject has no ID set", ErrObjectIDMissing)
	}

	url := obj.client().makeURL("sobjects/" + obj.Type() + "/" + oid)
	data, err := obj.client().httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequest, err)
	}

	err = json.Unmarshal(data, obj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseResponse, err)
	}

	return obj, nil
}

// Create posts the JSON representation of the SObject to salesforce to create the entry.
// If the creation is successful, the ID of the SObject instance is updated with the ID returned.
// Returns the SObject and nil error on success; returns nil and an error on failure.
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api_rest.meta/api_rest/dome_sobject_create.htm
func (obj *SObject) Create() (*SObject, error) {
	return obj.CreateWithContext(context.Background())
}

func (obj *SObject) CreateWithContext(ctx context.Context) (*SObject, error) {
	if obj.Type() == "" {
		return nil, fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
	}
	if obj.client() == nil {
		return nil, fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
	}

	// Make a copy of the incoming SObject, but skip certain metadata fields as they're not understood by salesforce.
	reqObj := obj.makeCopy()
	reqData, err := json.Marshal(reqObj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMarshalRequest, err)
	}

	url := obj.client().makeURL("sobjects/" + obj.Type() + "/")
	respData, err := obj.client().httpRequest(ctx, http.MethodPost, url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequest, err)
	}

	err = obj.setIDFromResponseData(respData)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseResponse, err)
	}

	return obj, nil
}

// Update updates SObject in place. Upon successful update, the same SObject pointer is returned.
// Returns the SObject and nil error on success; returns nil and an error on failure.
// ID is required.
func (obj *SObject) Update() (*SObject, error) {
	return obj.UpdateWithContext(context.Background())
}

func (obj *SObject) UpdateWithContext(ctx context.Context) (*SObject, error) {
	if obj.Type() == "" {
		return nil, fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
	}
	if obj.client() == nil {
		return nil, fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
	}
	if obj.ID() == "" {
		return nil, fmt.Errorf("%w: SObject has no ID set", ErrObjectIDMissing)
	}

	// Make a copy of the incoming SObject, but skip certain metadata fields as they're not understood by salesforce.
	reqObj := obj.makeCopy()
	reqData, err := json.Marshal(reqObj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMarshalRequest, err)
	}

	queryBase := "sobjects/"
	if obj.client().useToolingAPI {
		queryBase = "tooling/sobjects/"
	}
	url := obj.client().makeURL(queryBase + obj.Type() + "/" + obj.ID())
	_, err = obj.client().httpRequest(ctx, http.MethodPatch, url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequest, err)
	}

	return obj, nil
}

// Upsert creates SObject or updates existing SObject in place. Upon successful upsert, the same SObject pointer is returned.
// Returns the SObject and nil error on success; returns nil and an error on failure.
// ExternalIDField, external ID value, and Type are required.
func (obj *SObject) Upsert() (*SObject, error) {
	return obj.UpsertWithContext(context.Background())
}

func (obj *SObject) UpsertWithContext(ctx context.Context) (*SObject, error) {
	if obj.Type() == "" {
		return nil, fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
	}
	if obj.client() == nil {
		return nil, fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
	}
	if obj.ExternalIDFieldName() == "" {
		return nil, fmt.Errorf("%w: ExternalIDField is not set", ErrExternalIDMissing)
	}
	if obj.ExternalID() == "" {
		return nil, fmt.Errorf("%w: external ID value is empty for field %q", ErrExternalIDMissing, obj.ExternalIDFieldName())
	}

	// Make a copy of the incoming SObject, but skip certain metadata fields as they're not understood by salesforce.
	reqObj := obj.makeCopy()
	reqData, err := json.Marshal(reqObj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMarshalRequest, err)
	}

	queryBase := "sobjects/"
	if obj.client().useToolingAPI {
		queryBase = "tooling/sobjects/"
	}
	url := obj.client().
		makeURL(queryBase + obj.Type() + "/" + obj.ExternalIDFieldName() + "/" + obj.ExternalID())
	respData, err := obj.client().httpRequest(ctx, http.MethodPatch, url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequest, err)
	}

	// Upsert returns with 201 and id in response if a new record is created. If a record is updated, it returns
	// a 204 with an empty response
	if len(respData) > 0 {
		err = obj.setIDFromResponseData(respData)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrParseResponse, err)
		}
	}

	return obj, nil
}

// Delete deletes an SObject record identified by external ID. nil is returned if the operation completes successfully;
// otherwise an error is returned
func (obj *SObject) Delete(id ...string) error {
	return obj.DeleteWithContext(context.Background(), id...)
}

func (obj *SObject) DeleteWithContext(ctx context.Context, id ...string) error {
	if obj.Type() == "" {
		return fmt.Errorf("%w: SObject type is empty", ErrObjectTypeMissing)
	}
	if obj.client() == nil {
		return fmt.Errorf("%w: SObject has no associated client", ErrObjectClientMissing)
	}

	oid := obj.ID()
	if len(id) > 0 {
		oid = id[0]
	}
	if oid == "" {
		return fmt.Errorf("%w: no ID provided and SObject has no ID set", ErrObjectIDMissing)
	}

	url := obj.client().makeURL("sobjects/" + obj.Type() + "/" + oid)
	obj.client().logger.Println(logPrefix, url)
	_, err := obj.client().httpRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	return nil
}

// Type returns the type, or sometimes referred to as name, of an SObject.
func (obj *SObject) Type() string {
	attributes := obj.AttributesField()
	if attributes == nil {
		return ""
	}
	return attributes.Type
}

// ID returns the external ID of the SObject.
func (obj *SObject) ID() string {
	return obj.StringField(sobjectIDKey)
}

// ExternalIDField returns the external ID field of the SObject.
func (obj *SObject) ExternalIDFieldName() string {
	return obj.StringField(sobjectExternalIDFieldNameKey)
}

// ExternalID returns the external ID of the SObject.
func (obj *SObject) ExternalID() string {
	return obj.StringField(obj.ExternalIDFieldName())
}

// StringField accesses a field in the SObject as string. Empty string is returned if the field doesn't exist.
func (obj *SObject) StringField(key string) string {
	value := obj.InterfaceField(key)
	switch value.(type) {
	case string:
		return value.(string)
	default:
		return ""
	}
}

// SObjectField accesses a field in the SObject as another SObject. This is only applicable if the field is an external
// ID to another object. The typeName of the SObject must be provided. <nil> is returned if the field is empty.
func (obj *SObject) SObjectField(typeName, key string) *SObject {
	// First check if there's an associated ID directly.
	oid := obj.StringField(key)
	if oid != "" {
		object := &SObject{}
		object.setClient(obj.client())
		object.setType(typeName)
		object.setID(oid)
		return object
	}

	// Secondly, check if this could be a linked object, which doesn't have an ID but has the attributes.
	linkedObjRaw := obj.InterfaceField(key)
	linkedObjMapper, ok := linkedObjRaw.(map[string]any)
	if !ok {
		return nil
	}
	attrs, ok := linkedObjMapper[sobjectAttributesKey].(map[string]any)
	if !ok {
		return nil
	}

	// Reusing typeName here, which is ok
	typeName, _ = attrs["type"].(string)
	url, _ := attrs["url"].(string)
	if typeName == "" || url == "" {
		return nil
	}

	// Both type and url exist in attributes, this is a linked object!
	// Get the ID from URL.
	rIndex := strings.LastIndex(url, "/")
	if rIndex == -1 || rIndex+1 == len(url) {
		// hmm... this shouldn't happen, unless the URL is hand crafted.
		if obj.client() != nil {
			obj.client().logger.Println(logPrefix, "invalid url,", url)
		}
		return nil
	}
	oid = url[rIndex+1:]

	object := obj.client().SObject(typeName)
	object.setID(oid)
	for key, val := range linkedObjMapper {
		object.Set(key, val)
	}

	return object
}

// InterfaceField accesses a field in the SObject as raw interface. This allows access to any type of fields.
func (obj *SObject) InterfaceField(key string) any {
	return (*obj)[key]
}

func (obj *SObject) TimeField(key string) (t time.Time) {
	value := obj.InterfaceField(key)
	switch v := value.(type) {
	case string:
		if t, err := time.Parse("2006-01-02T15:04:05.000+0000", v); err == nil {
			return t
		}
	}
	return
}

// AttributesField returns a read-only copy of the attributes field of an SObject.
func (obj *SObject) AttributesField() *SObjectAttributes {
	attributes := obj.InterfaceField(sobjectAttributesKey)

	switch attributes.(type) {
	case SObjectAttributes:
		// Use a temporary variable to copy the value of attributes and return the address of the temp value.
		attrs := (attributes).(SObjectAttributes)
		return &attrs
	case map[string]any:
		// Can't convert attributes to concrete type; decode interface.
		mapper := attributes.(map[string]any)
		attrs := &SObjectAttributes{}
		if mapper["type"] != nil {
			attrs.Type = mapper["type"].(string)
		}
		if mapper["url"] != nil {
			attrs.URL = mapper["url"].(string)
		}
		return attrs
	default:
		return nil
	}
}

// Set indexes value into SObject instance with provided key. The same SObject pointer is returned to allow
// chained access.
func (obj *SObject) Set(key string, value any) *SObject {
	(*obj)[key] = value
	return obj
}

// client returns the associated Client with the SObject.
func (obj *SObject) client() *Client {
	client := obj.InterfaceField(sobjectClientKey)
	switch client.(type) {
	case *Client:
		return client.(*Client)
	default:
		return nil
	}
}

// setClient sets the associated Client with the SObject.
func (obj *SObject) setClient(client *Client) {
	(*obj)[sobjectClientKey] = client
}

// setType sets the type, or name for the SObject.
func (obj *SObject) setType(typeName string) {
	attributes := obj.InterfaceField(sobjectAttributesKey)
	switch attributes.(type) {
	case SObjectAttributes:
		attrs := obj.AttributesField()
		attrs.Type = typeName
		(*obj)[sobjectAttributesKey] = *attrs
	default:
		(*obj)[sobjectAttributesKey] = SObjectAttributes{
			Type: typeName,
		}
	}
}

// setID sets the external ID for the SObject.
func (obj *SObject) setID(id string) {
	(*obj)[sobjectIDKey] = id
}

// makeCopy copies the fields of an SObject to a new map without metadata fields.
func (obj *SObject) makeCopy() map[string]any {
	stripped := make(map[string]any)
	for key, val := range *obj {
		if key == sobjectClientKey ||
			key == sobjectAttributesKey ||
			key == sobjectIDKey ||
			key == sobjectExternalIDFieldNameKey ||
			key == obj.ExternalIDFieldName() {
			continue
		}
		stripped[key] = val
	}
	for _, key := range blacklistedUpdateFields {
		delete(stripped, key)
	}
	return stripped
}

func (obj *SObject) setIDFromResponseData(respData []byte) error {
	// Use an anonymous struct to parse the result if any. This might need to be changed if the result should
	// be returned to the caller in some manner, especially if the client would like to decode the errors.
	var respVal struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
	}
	err := json.Unmarshal(respData, &respVal)
	if err != nil {
		if obj.client() != nil {
			obj.client().logger.Println(logPrefix, "failed to process response data,", err)
		}
		return err
	}

	if !respVal.Success || respVal.ID == "" {
		if obj.client() != nil {
			obj.client().logger.Println(logPrefix, "unsuccessful")
		}
		return errors.New("request was unsuccessful")
	}

	obj.setID(respVal.ID)
	return nil
}
