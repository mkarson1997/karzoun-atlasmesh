package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceRegistrationAndResolution(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));server:=httptest.NewServer(NewServer(nil,nil,nil,nil,logger).Handler());defer server.Close();payload:=`{"service":"payments","id":"p1","address":"http://127.0.0.1:9000","weight":1,"ttl_seconds":30}`;resp,err:=http.Post(server.URL+"/v1/services/register","application/json",strings.NewReader(payload));if err!=nil{t.Fatal(err)};defer resp.Body.Close();if resp.StatusCode!=http.StatusCreated{t.Fatalf("register status=%d",resp.StatusCode)};resp,err=http.Get(server.URL+"/v1/services/payments/resolve");if err!=nil{t.Fatal(err)};defer resp.Body.Close();if resp.StatusCode!=http.StatusOK{t.Fatalf("resolve status=%d",resp.StatusCode)};var resolved struct{ID string `json:"id"`};if err:=json.NewDecoder(resp.Body).Decode(&resolved);err!=nil{t.Fatal(err)};if resolved.ID!="p1"{t.Fatalf("resolved=%q",resolved.ID)}}
func TestJobLeaseFencingViaAPI(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));server:=httptest.NewServer(NewServer(nil,nil,nil,nil,logger).Handler());defer server.Close();postJSON(t,server.URL+"/v1/jobs",`{"id":"j1","type":"email","max_attempts":2}`,http.StatusCreated);claimBody:=postJSON(t,server.URL+"/v1/jobs/claim",`{"worker":"w1","lease_seconds":30}`,http.StatusOK);if !bytes.Contains(claimBody,[]byte(`"state":"leased"`)){t.Fatalf("claim body=%s",claimBody)};postJSON(t,server.URL+"/v1/jobs/j1/complete",`{"worker":"other","result":{"ok":true}}`,http.StatusConflict);postJSON(t,server.URL+"/v1/jobs/j1/complete",`{"worker":"w1","result":{"ok":true}}`,http.StatusOK)}
func postJSON(t *testing.T,url,body string,want int)[]byte{t.Helper();resp,err:=http.Post(url,"application/json",strings.NewReader(body));if err!=nil{t.Fatal(err)};defer resp.Body.Close();data,err:=io.ReadAll(resp.Body);if err!=nil{t.Fatal(err)};if resp.StatusCode!=want{t.Fatalf("POST %s status=%d want=%d body=%s",url,resp.StatusCode,want,data)};return data}
